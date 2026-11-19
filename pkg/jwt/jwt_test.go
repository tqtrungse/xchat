/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xchat/pkg/log"
	"xchat/pkg/log/logtest"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://issuer.example"
	testAudience = "xchat"
	testKeyID    = "test-key"
)

func TestVerifyValidToken(t *testing.T) {
	privateKey := newTestKey(t)
	server := newJWKSServer(t, jwksBody(t, privateKey, testKeyID))
	testAuth := newTestAuth(t, server.URL)

	require.True(t, testAuth.Ready())

	claims, err := testAuth.Verify(signTestToken(t, privateKey, testKeyID))
	require.NoError(t, err)
	require.Equal(t, "user-1", claims.Subject)
}

func TestVerifyInvalidSignature(t *testing.T) {
	servedKey := newTestKey(t)
	otherKey := newTestKey(t)
	server := newJWKSServer(t, jwksBody(t, servedKey, testKeyID))
	testAuth := newTestAuth(t, server.URL)

	_, err := testAuth.Verify(signTestToken(t, otherKey, testKeyID))
	require.ErrorIs(t, err, ErrTokenInvalid)
	require.NotErrorIs(t, err, ErrVerifierUnavailable)
}

func TestVerifyUnknownKeyID(t *testing.T) {
	privateKey := newTestKey(t)
	server := newJWKSServer(t, jwksBody(t, privateKey, testKeyID))
	testAuth := newTestAuth(t, server.URL)

	_, err := testAuth.Verify(signTestToken(t, privateKey, "unknown-key"))
	require.ErrorIs(t, err, ErrTokenInvalid)
	require.NotErrorIs(t, err, ErrVerifierUnavailable)
}

func TestVerifyVerifierUnavailable(t *testing.T) {
	privateKey := newTestKey(t)
	server := newUnavailableJWKSServer(t)
	testAuth, entries := newTestAuthWithLog(t, server.URL, testOptions)

	require.False(t, testAuth.Ready())

	_, err := testAuth.Verify(signTestToken(t, privateKey, testKeyID))
	require.ErrorIs(t, err, ErrVerifierUnavailable)
	require.NotErrorIs(t, err, ErrTokenInvalid)

	// The logger of the service receives the failure of the first fetch.
	// slog.Default does not receive it.
	require.Contains(t, messages(entries(), "error"), "failed to refresh JWKS")
}

func TestNewAuthRetriesUntilReady(t *testing.T) {
	privateKey := newTestKey(t)
	body := jwksBody(t, privateKey, testKeyID)

	// The first three requests fail: the first fetch and two retries.
	var requests atomic.Int32
	server := httptest.NewServer(nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		if requests.Add(1) <= 3 {
			writer.WriteHeader(nethttp.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	t.Cleanup(server.Close)

	testAuth, entries := newTestAuthWithLog(t, server.URL, testOptions)
	require.False(t, testAuth.Ready())
	require.Eventually(t, testAuth.Ready, 5*time.Second, 10*time.Millisecond)

	claims, err := testAuth.Verify(signTestToken(t, privateKey, testKeyID))
	require.NoError(t, err)
	require.Equal(t, "user-1", claims.Subject)

	require.Eventually(
		t,
		func() bool {
			return slices.Contains(messages(entries(), "info"), "JWKS is ready")
		},
		time.Second, 10*time.Millisecond,
	)
	warnings := messages(entries(), "warn")
	require.Len(t, warnings, 2)
	require.Equal(t, "failed to fetch JWKS, retrying", warnings[0])
}

func TestNewAuthStopsRetryOnCancel(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		requests.Add(1)
		writer.WriteHeader(nethttp.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	logger, closeLogger, _ := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })

	ctx, cancel := context.WithCancel(t.Context())
	testAuth, err := newAuth(ctx, testAuthConfig(server.URL), logger, testOptions)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return requests.Load() >= 2 }, 5*time.Second, 10*time.Millisecond)

	cancel()
	// Wait for the current attempt to finish. Then check that no more
	// attempts start.
	time.Sleep(4 * testOptions.retryMax)
	stopped := requests.Load()
	time.Sleep(4 * testOptions.retryMax)
	require.Equal(t, stopped, requests.Load())
	require.False(t, testAuth.Ready())
}

func TestVerifyClockSkew(t *testing.T) {
	privateKey := newTestKey(t)
	server := newJWKSServer(t, jwksBody(t, privateKey, testKeyID))
	testAuth := newTestAuth(t, server.URL)

	t.Run("inside leeway", func(t *testing.T) {
		claims := testClaims(time.Now())
		claims.ExpiresAt = jwtlib.NewNumericDate(time.Now().Add(-10 * time.Second))
		claims.IssuedAt = jwtlib.NewNumericDate(time.Now().Add(-time.Hour))

		_, err := testAuth.Verify(signTestClaims(t, privateKey, testKeyID, claims))
		require.NoError(t, err)
	})

	t.Run("outside leeway", func(t *testing.T) {
		claims := testClaims(time.Now())
		claims.ExpiresAt = jwtlib.NewNumericDate(time.Now().Add(-2 * time.Minute))
		claims.IssuedAt = jwtlib.NewNumericDate(time.Now().Add(-time.Hour))

		_, err := testAuth.Verify(signTestClaims(t, privateKey, testKeyID, claims))
		require.ErrorIs(t, err, ErrTokenInvalid)
	})
}

func TestNewAuthRequiredFields(t *testing.T) {
	for _, test := range []struct {
		name      string
		config    AuthConfig
		wantError error
	}{
		{
			name: "missing issuer",
			config: AuthConfig{
				Audience: testAudience,
				JWKSURL:  "https://issuer.example/jwks",
			},
			wantError: ErrIssuerRequired,
		},
		{
			name: "missing audience",
			config: AuthConfig{
				Issuer:  testIssuer,
				JWKSURL: "https://issuer.example/jwks",
			},
			wantError: ErrAudienceRequired,
		},
		{
			name: "missing JWKS URL",
			config: AuthConfig{
				Issuer:   testIssuer,
				Audience: testAudience,
			},
			wantError: ErrJWKSURLRequired,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			logger, closeLogger, _ := logtest.New(t, log.Config{})
			t.Cleanup(func() { require.NoError(t, closeLogger()) })

			_, err := NewAuth(t.Context(), test.config, logger)
			require.ErrorIs(t, err, test.wantError)
		})
	}

	t.Run("missing logger", func(t *testing.T) {
		_, err := NewAuth(t.Context(), testAuthConfig("https://issuer.example/jwks"), nil)
		require.ErrorIs(t, err, ErrLoggerRequired)
	})
}

func TestValidateURL(t *testing.T) {
	for _, test := range []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name: "https",
			raw:  "https://issuer.example/jwks",
		},
		{
			name: "https with port",
			raw:  "https://issuer.example:8443/jwks",
		},
		{
			name: "http localhost",
			raw:  "http://localhost:8080/realms/iam/jwks",
		},
		{
			name: "http localhost uppercase",
			raw:  "http://LOCALHOST:8080/jwks",
		},
		{
			name: "http ipv4 loopback",
			raw:  "http://127.0.0.1:8080/jwks",
		},
		{
			name: "http ipv6 loopback",
			raw:  "http://[::1]:8080/jwks",
		},
		{
			name:    "http public host",
			raw:     "http://issuer.example/jwks",
			wantErr: true,
		},
		{
			name:    "ftp scheme",
			raw:     "ftp://issuer.example/jwks",
			wantErr: true,
		},
		{
			name:    "relative path",
			raw:     "/jwks",
			wantErr: true,
		},
		{
			name:    "missing host",
			raw:     "https:///jwks",
			wantErr: true,
		},
		{
			name:    "missing scheme",
			raw:     "issuer.example/jwks",
			wantErr: true,
		},
		{
			name:    "malformed",
			raw:     "://bad",
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateURL(test.raw, ErrJWKSURLInsecure)
			if !test.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrJWKSURLInsecure)
		})
	}

	t.Run("issuer error", func(t *testing.T) {
		err := validateURL("http://issuer.example", ErrIssuerInsecure)
		require.ErrorIs(t, err, ErrIssuerInsecure)
	})
}

func TestNewAuthRejectsInsecureIssuer(t *testing.T) {
	logger, closeLogger, _ := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })

	config := testAuthConfig("https://issuer.example/jwks")
	config.Issuer = "http://issuer.example"

	_, err := NewAuth(t.Context(), config, logger)
	require.ErrorIs(t, err, ErrIssuerInsecure)
}

func TestNewAuthRejectsInsecureJWKSURL(t *testing.T) {
	logger, closeLogger, _ := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })

	_, err := NewAuth(t.Context(), testAuthConfig("http://issuer.example/jwks"), logger)
	require.ErrorIs(t, err, ErrJWKSURLInsecure)
}

func TestNewAuthLogsFirstFetchFailure(t *testing.T) {
	server := newUnavailableJWKSServer(t)
	testAuth, entries := newTestAuthWithLog(t, server.URL, testOptions)
	require.False(t, testAuth.Ready())

	require.Eventually(
		t,
		func() bool {
			return slices.Contains(messages(entries(), "error"), "failed to refresh JWKS")
		},
		5*time.Second, 10*time.Millisecond,
	)
}

func TestNewAuthJWKSRequestTimeout(t *testing.T) {
	const requestTimeout = 50 * time.Millisecond

	// The handler waits for the client to cancel the request.
	var requests atomic.Int32
	server := httptest.NewServer(nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		requests.Add(1)
		<-request.Context().Done()
	}))
	t.Cleanup(server.Close)

	opts := testOptions
	opts.requestTimeout = requestTimeout
	start := time.Now()
	testAuth, _ := newTestAuthWithLog(t, server.URL, opts)
	elapsed := time.Since(start)

	require.False(t, testAuth.Ready())
	// The server received the request. NewAuth waited for the timeout. A
	// fast failure cannot cause the result.
	require.GreaterOrEqual(t, requests.Load(), int32(1))
	require.GreaterOrEqual(t, elapsed, requestTimeout)
	// The default timeout is one minute. This bound catches a missing override.
	require.Less(t, elapsed, 5*time.Second)
}

// testOptions keeps the retries of a test short.
var testOptions = options{
	requestTimeout: time.Second,
	retryMin:       10 * time.Millisecond,
	retryMax:       50 * time.Millisecond,
}

type logEntry struct {
	Level   string `json:"level"`
	Message string `json:"msg"`
}

func testAuthConfig(jwksURL string) AuthConfig {
	return AuthConfig{
		Issuer:   testIssuer,
		Audience: testAudience,
		JWKSURL:  jwksURL,
	}
}

func newTestAuth(t *testing.T, jwksURL string) Auth {
	t.Helper()

	testAuth, _ := newTestAuthWithLog(t, jwksURL, testOptions)
	return testAuth
}

// newTestAuthWithLog gives an Auth and a function that reads the log entries
// of that Auth.
func newTestAuthWithLog(t *testing.T, jwksURL string, opts options) (Auth, func() []logEntry) {
	t.Helper()

	logger, closeLogger, path := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })

	testAuth, err := newAuth(t.Context(), testAuthConfig(jwksURL), logger, opts)
	require.NoError(t, err)

	entries := func() []logEntry {
		t.Helper()

		require.NoError(t, logger.Sync())
		raw, err := os.ReadFile(path)
		require.NoError(t, err)

		var result []logEntry
		for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
			if line == "" {
				continue
			}
			var entry logEntry
			require.NoError(t, json.Unmarshal([]byte(line), &entry))
			result = append(result, entry)
		}
		return result
	}
	return testAuth, entries
}

// messages gives the message of each entry at the level.
func messages(entries []logEntry, level string) []string {
	var result []string
	for _, entry := range entries {
		if entry.Level == level {
			result = append(result, entry.Message)
		}
	}
	return result
}

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey
}

func newJWKSServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func newUnavailableJWKSServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		writer.WriteHeader(nethttp.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return server
}

func jwksBody(t *testing.T, privateKey *rsa.PrivateKey, keyID string) []byte {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": keyID,
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
			},
		},
	})
	require.NoError(t, err)
	return body
}

func signTestToken(t *testing.T, privateKey *rsa.PrivateKey, keyID string) string {
	return signTestClaims(t, privateKey, keyID, testClaims(time.Now()))
}

func testClaims(now time.Time) jwtlib.RegisteredClaims {
	return jwtlib.RegisteredClaims{
		Issuer:    testIssuer,
		Audience:  jwtlib.ClaimStrings{testAudience},
		Subject:   "user-1",
		IssuedAt:  jwtlib.NewNumericDate(now),
		ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Hour)),
	}
}

func signTestClaims(
	t *testing.T,
	privateKey *rsa.PrivateKey,
	keyID string,
	claims jwtlib.RegisteredClaims,
) string {
	t.Helper()

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	token.Header["kid"] = keyID

	signed, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return signed
}
