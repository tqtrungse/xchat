/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	auth := newTestAuth(t, server.URL)

	require.True(t, auth.Ready())

	claims, err := auth.Verify(signTestToken(t, privateKey, testKeyID))
	require.NoError(t, err)
	require.Equal(t, "user-1", claims.Subject)
}

func TestVerifyInvalidSignature(t *testing.T) {
	servedKey := newTestKey(t)
	otherKey := newTestKey(t)
	server := newJWKSServer(t, jwksBody(t, servedKey, testKeyID))
	auth := newTestAuth(t, server.URL)

	_, err := auth.Verify(signTestToken(t, otherKey, testKeyID))
	require.ErrorIs(t, err, ErrTokenInvalid)
	require.NotErrorIs(t, err, ErrVerifierUnavailable)
}

func TestVerifyUnknownKeyID(t *testing.T) {
	privateKey := newTestKey(t)
	server := newJWKSServer(t, jwksBody(t, privateKey, testKeyID))
	auth := newTestAuth(t, server.URL)

	_, err := auth.Verify(signTestToken(t, privateKey, "unknown-key"))
	require.ErrorIs(t, err, ErrTokenInvalid)
	require.NotErrorIs(t, err, ErrVerifierUnavailable)
}

func TestVerifyVerifierUnavailable(t *testing.T) {
	privateKey := newTestKey(t)
	server := newUnavailableJWKSServer(t)
	auth := newTestAuth(t, server.URL)

	require.False(t, auth.Ready())

	_, err := auth.Verify(signTestToken(t, privateKey, testKeyID))
	require.ErrorIs(t, err, ErrVerifierUnavailable)
	require.NotErrorIs(t, err, ErrTokenInvalid)
}

func TestVerifyClockSkew(t *testing.T) {
	privateKey := newTestKey(t)
	server := newJWKSServer(t, jwksBody(t, privateKey, testKeyID))
	auth := newTestAuth(t, server.URL)

	t.Run("inside leeway", func(t *testing.T) {
		claims := testClaims(time.Now())
		claims.ExpiresAt = jwtlib.NewNumericDate(time.Now().Add(-10 * time.Second))
		claims.IssuedAt = jwtlib.NewNumericDate(time.Now().Add(-time.Hour))

		_, err := auth.Verify(signTestClaims(t, privateKey, testKeyID, claims))
		require.NoError(t, err)
	})

	t.Run("outside leeway", func(t *testing.T) {
		claims := testClaims(time.Now())
		claims.ExpiresAt = jwtlib.NewNumericDate(time.Now().Add(-2 * time.Minute))
		claims.IssuedAt = jwtlib.NewNumericDate(time.Now().Add(-time.Hour))

		_, err := auth.Verify(signTestClaims(t, privateKey, testKeyID, claims))
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
			wantError: ErrJwksUrlRequired,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewAuth(t.Context(), test.config)
			require.ErrorIs(t, err, test.wantError)
		})
	}
}

func newTestAuth(t *testing.T, jwksURL string) Auth {
	t.Helper()

	auth, err := NewAuth(t.Context(), AuthConfig{
		Issuer:   testIssuer,
		Audience: testAudience,
		JWKSURL:  jwksURL,
	})
	require.NoError(t, err)
	return auth
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
				"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
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
