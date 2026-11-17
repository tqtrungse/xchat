/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package http

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xchat/internal/iam/core/rejection"
	"xchat/pkg/jwt"
	"xchat/pkg/jwt/mocks"
	"xchat/pkg/log"
	"xchat/pkg/log/logtest"
	"xchat/pkg/reject"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewServerDefaults(t *testing.T) {
	server := newTestServer(t, newMockAuth(t))

	require.Equal(t, defaultReadHeaderTimeout, server.httpServer.ReadHeaderTimeout)
	require.Equal(t, defaultReadTimeout, server.httpServer.ReadTimeout)
	require.Equal(t, defaultWriteTimeout, server.httpServer.WriteTimeout)
	require.Equal(t, defaultIdleTimeout, server.httpServer.IdleTimeout)
	require.Equal(t, defaultMaxHeaderBytes, server.httpServer.MaxHeaderBytes)
	require.Equal(t, "127.0.0.1:0", server.httpServer.Addr)

	handler, exists := server.httpServer.Handler.(*gin.Engine)
	require.True(t, exists)
	require.Same(t, server.engine, handler)
}

func TestNewServerRequiredFields(t *testing.T) {
	for _, test := range []struct {
		name   string
		config Config
	}{
		{
			name: "missing address",
			config: Config{
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			},
		},
		{
			name: "missing certificate",
			config: Config{
				Addr:    "127.0.0.1:0",
				KeyFile: "key.pem",
			},
		},
		{
			name: "missing key",
			config: Config{
				Addr:     "127.0.0.1:0",
				CertFile: "cert.pem",
			},
		},
		{
			name: "missing OIDC settings",
			config: Config{
				Addr:     "127.0.0.1:0",
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			logger, rejections := newTestDeps(t)
			_, err := NewServer(context.Background(), test.config, logger, rejections)
			require.Error(t, err)
		})
	}
}

func TestNewServerRequiresRejections(t *testing.T) {
	logger, _ := newTestDeps(t)
	_, err := NewServer(context.Background(), testServerConfig(), logger, nil)
	require.ErrorIs(t, err, errRejectionsRequired)
}

func TestNewServerRequiresLogger(t *testing.T) {
	_, rejections := newTestDeps(t)
	_, err := NewServer(context.Background(), testServerConfig(), nil, rejections)
	require.ErrorIs(t, err, errLoggerRequired)
}

func TestServerRunRequiresCertificate(t *testing.T) {
	server := newTestServer(t, newMockAuth(t))

	err := server.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, "serve 127.0.0.1:0")
}

func TestServerServesHTTPS(t *testing.T) {
	certFile, keyFile := writeTestCertificate(t)

	logger, rejections := newTestDeps(t)
	server, err := buildServer(Config{
		Addr:     "127.0.0.1:0",
		CertFile: certFile,
		KeyFile:  keyFile,
	}, newMockAuth(t), logger, rejections)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.httpServer.ServeTLS(listener, certFile, keyFile)
	}()

	client := &nethttp.Client{
		Transport: &nethttp.Transport{
			TLSClientConfig: &tls.Config{
				// The test certificate is self-signed.
				InsecureSkipVerify: true,
			},
		},
	}

	response, err := client.Get("https://" + listener.Addr().String() + "/healthz")
	require.NoError(t, err)
	require.Equal(t, nethttp.StatusNoContent, response.StatusCode)
	require.NoError(t, response.Body.Close())

	err = server.Shutdown(context.Background())
	require.NoError(t, err)

	err = <-serveResult
	require.ErrorIs(t, err, nethttp.ErrServerClosed)
}

func TestAuthenticate(t *testing.T) {
	validClaims := &jwt.Claims{
		Scope: "read",
	}

	for _, test := range []struct {
		name          string
		setHeader     bool
		authorization string
		verifyToken   string
		verifyClaims  *jwt.Claims
		verifyError   error
		wantStatus    int
		wantHeader    string
		wantJSON      string
		wantHandler   bool
	}{
		{
			name:       "missing header",
			wantStatus: nethttp.StatusUnauthorized,
			wantHeader: `Bearer realm="iam"`,
			wantJSON:   `{"error":"invalid_request","error_description":"missing or invalid Authorization header"}`,
		},
		{
			name:          "wrong scheme",
			setHeader:     true,
			authorization: "Basic abc",
			wantStatus:    nethttp.StatusUnauthorized,
			wantHeader:    `Bearer realm="iam", error="invalid_request", error_description="missing or invalid Authorization header"`,
			wantJSON:      `{"error":"invalid_request","error_description":"missing or invalid Authorization header"}`,
		},
		{
			name:          "empty token",
			setHeader:     true,
			authorization: "Bearer   ",
			wantStatus:    nethttp.StatusUnauthorized,
			wantHeader:    `Bearer realm="iam", error="invalid_request", error_description="missing or invalid Authorization header"`,
			wantJSON:      `{"error":"invalid_request","error_description":"missing or invalid Authorization header"}`,
		},
		{
			name:          "verification failed",
			setHeader:     true,
			authorization: "Bearer bad-token",
			verifyToken:   "bad-token",
			verifyError:   jwt.ErrTokenInvalid,
			wantStatus:    nethttp.StatusUnauthorized,
			wantHeader:    `Bearer realm="iam", error="invalid_token", error_description="failed to verify token"`,
			wantJSON:      `{"error":"invalid_token","error_description":"failed to verify token"}`,
		},
		{
			name:          "verifier unavailable",
			setHeader:     true,
			authorization: "Bearer any-token",
			verifyToken:   "any-token",
			verifyError:   jwt.ErrVerifierUnavailable,
			wantStatus:    nethttp.StatusServiceUnavailable,
			wantJSON:      `{"error":"server_error","error_description":"token verification is unavailable"}`,
		},
		{
			name:          "valid token",
			setHeader:     true,
			authorization: "Bearer good-token",
			verifyToken:   "good-token",
			verifyClaims:  validClaims,
			wantStatus:    nethttp.StatusNoContent,
			wantHandler:   true,
		},
		{
			name:          "lowercase scheme",
			setHeader:     true,
			authorization: "bearer good-token",
			verifyToken:   "good-token",
			verifyClaims:  validClaims,
			wantStatus:    nethttp.StatusNoContent,
			wantHandler:   true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			auth := newMockAuth(t)
			if test.verifyToken != "" {
				auth.EXPECT().Verify(test.verifyToken).Return(test.verifyClaims, test.verifyError)
			}
			server := newTestServer(t, auth)

			handlerCalled := false
			var handlerClaims *jwt.Claims
			server.APIGroup().GET("/ping", func(c *gin.Context) {
				handlerCalled = true
				claims, exists := ClaimsFromContext(c)
				require.True(t, exists)
				handlerClaims = claims
				c.Status(nethttp.StatusNoContent)
			})

			request := httptest.NewRequest(nethttp.MethodGet, "/api/ping", nil)
			if test.setHeader {
				request.Header.Set("Authorization", test.authorization)
			}
			recorder := httptest.NewRecorder()
			server.engine.ServeHTTP(recorder, request)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			require.Empty(t, recorder.Header().Get("Pragma"))
			require.Equal(t, test.wantHandler, handlerCalled)
			if test.wantHandler {
				require.Same(t, validClaims, handlerClaims)
				return
			}

			require.Nil(t, handlerClaims)
			require.Equal(t, test.wantHeader, recorder.Header().Get("WWW-Authenticate"))
			require.JSONEq(t, test.wantJSON, recorder.Body.String())
		})
	}
}

func TestAuthenticateRecordsRejections(t *testing.T) {
	logger, rejections := newTestDeps(t)

	auth := newMockAuth(t)
	auth.EXPECT().Verify("bad-token").Return(nil, jwt.ErrTokenInvalid)
	auth.EXPECT().Verify("unavailable-token").Return(nil, jwt.ErrVerifierUnavailable)

	server, err := buildServer(testServerConfig(), auth, logger, rejections)
	require.NoError(t, err)

	server.APIGroup().GET("/ping", func(c *gin.Context) {
		c.Status(nethttp.StatusNoContent)
	})

	for _, authorization := range []string{
		"",
		"Basic abc",
		"Bearer bad-token",
		"Bearer unavailable-token",
	} {
		request := httptest.NewRequest(nethttp.MethodGet, "/api/ping", nil)
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}
		recorder := httptest.NewRecorder()
		server.engine.ServeHTTP(recorder, request)
	}

	require.EqualValues(t, 2, rejections.Count(rejection.AuthHeader))
	require.EqualValues(t, 1, rejections.Count(rejection.AuthToken))
}

func TestRequestLoggerWritesEntry(t *testing.T) {
	logger, closeLogger, path := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })
	rejections := reject.New[rejection.Reason](logger, time.Hour, rejection.Names())

	auth := newMockAuth(t)
	auth.EXPECT().Verify("good-token").Return(&jwt.Claims{}, nil)

	server, err := buildServer(testServerConfig(), auth, logger, rejections)
	require.NoError(t, err)
	server.APIGroup().GET("/ping", func(c *gin.Context) {
		c.Status(nethttp.StatusNoContent)
	})

	request := httptest.NewRequest(nethttp.MethodGet, "/api/ping", nil)
	request.Header.Set("Authorization", "Bearer good-token")
	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(recorder, request)

	require.Equal(t, nethttp.StatusNoContent, recorder.Code)

	require.NoError(t, logger.Sync())
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"msg":"http request"`)
	require.Contains(t, string(raw), `"route":"/api/ping"`)
	require.Contains(t, string(raw), `"status":204`)
}

func TestRecoverPanicWritesEntry(t *testing.T) {
	logger, closeLogger, path := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })
	rejections := reject.New[rejection.Reason](logger, time.Hour, rejection.Names())

	auth := newMockAuth(t)
	auth.EXPECT().Verify("good-token").Return(&jwt.Claims{}, nil)

	server, err := buildServer(testServerConfig(), auth, logger, rejections)
	require.NoError(t, err)
	server.APIGroup().GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	request := httptest.NewRequest(nethttp.MethodGet, "/api/panic", nil)
	request.Header.Set("Authorization", "Bearer good-token")
	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(recorder, request)

	require.Equal(t, nethttp.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"error":"internal_error"}`, recorder.Body.String())

	require.NoError(t, logger.Sync())
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"msg":"panic"`)
	require.Contains(t, string(raw), `"panic":"boom"`)
}

func TestServerReady(t *testing.T) {
	for _, test := range []struct {
		name  string
		ready bool
	}{
		{
			name:  "ready",
			ready: true,
		},
		{
			name:  "not ready",
			ready: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			auth := newMockAuth(t)
			auth.EXPECT().Ready().Return(test.ready)
			server := newTestServer(t, auth)
			require.Equal(t, test.ready, server.Ready())
		})
	}
}

func TestHealthEndpoints(t *testing.T) {
	for _, test := range []struct {
		name        string
		path        string
		ready       bool
		expectReady bool
		wantStatus  int
	}{
		{
			name:       "health",
			path:       "/healthz",
			wantStatus: nethttp.StatusNoContent,
		},
		{
			name:        "ready",
			path:        "/readyz",
			ready:       true,
			expectReady: true,
			wantStatus:  nethttp.StatusNoContent,
		},
		{
			name:        "not ready",
			path:        "/readyz",
			expectReady: true,
			wantStatus:  nethttp.StatusServiceUnavailable,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			auth := newMockAuth(t)
			if test.expectReady {
				auth.EXPECT().Ready().Return(test.ready)
			}
			server := newTestServer(t, auth)

			request := httptest.NewRequest(nethttp.MethodGet, test.path, nil)
			recorder := httptest.NewRecorder()
			server.engine.ServeHTTP(recorder, request)

			require.Equal(t, test.wantStatus, recorder.Code)
		})
	}
}

func TestRequestLoggerSkipsProbes(t *testing.T) {
	auth := newMockAuth(t)
	auth.EXPECT().Ready().Return(true).AnyTimes()

	logger, closeLogger, path := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })
	rejections := reject.New[rejection.Reason](logger, time.Hour, rejection.Names())
	server, err := buildServer(testServerConfig(), auth, logger, rejections)
	require.NoError(t, err)

	server.PublicGroup().GET("/metadata", func(c *gin.Context) {
		c.Status(nethttp.StatusOK)
	})

	for _, target := range []string{healthzPath, readyzPath, "/metadata", "/missing"} {
		request := httptest.NewRequest(nethttp.MethodGet, target, nil)
		server.engine.ServeHTTP(httptest.NewRecorder(), request)
	}

	require.NoError(t, logger.Sync())
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	type entry struct {
		Message string `json:"msg"`
		Route   string `json:"route"`
		Status  int    `json:"status"`
	}
	var entries []entry
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var e entry
		require.NoError(t, json.Unmarshal([]byte(line), &e))
		if e.Message == "http request" {
			entries = append(entries, e)
		}
	}

	// An unmatched request has an empty route. The logger still writes an
	// entry for it.
	require.Equal(t, []entry{
		{Message: "http request", Route: "/metadata", Status: nethttp.StatusOK},
		{Message: "http request", Route: "", Status: nethttp.StatusNotFound},
	}, entries)
}

func TestServerDoesNotTrustProxyHeaders(t *testing.T) {
	server := newTestServer(t, newMockAuth(t))

	server.PublicGroup().GET("/client-ip", func(c *gin.Context) {
		c.String(nethttp.StatusOK, c.ClientIP())
	})

	request := httptest.NewRequest(nethttp.MethodGet, "/client-ip", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.7")
	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(recorder, request)

	require.Equal(t, "192.0.2.10", recorder.Body.String())
}

func TestPublicGroupHasNoAuthentication(t *testing.T) {
	auth := newMockAuth(t)
	auth.EXPECT().Verify(gomock.Any()).Times(0)

	logger, rejections := newTestDeps(t)
	server, err := buildServer(testServerConfig(), auth, logger, rejections)
	require.NoError(t, err)

	request := httptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(recorder, request)

	require.Equal(t, nethttp.StatusNoContent, recorder.Code)
}

func testServerConfig() Config {
	return Config{
		Addr:         "127.0.0.1:0",
		CertFile:     "cert.pem",
		KeyFile:      "key.pem",
		OIDCIssuer:   "https://issuer.example",
		OIDCAudience: "xchat",
		OIDCJWKSURL:  "https://issuer.example/.well-known/jwks.json",
	}
}

func writeTestCertificate(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	privateKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err)

	directory := t.TempDir()
	certFile := filepath.Join(directory, "cert.pem")
	keyFile := filepath.Join(directory, "key.pem")

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	})
	require.NoError(t, os.WriteFile(certFile, certificatePEM, 0o600))

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyDER,
	})
	require.NoError(t, os.WriteFile(keyFile, privateKeyPEM, 0o600))

	return certFile, keyFile
}

func newMockAuth(t *testing.T) *mocks.MockAuth {
	t.Helper()
	return mocks.NewMockAuth(gomock.NewController(t))
}

func newTestDeps(t *testing.T) (*log.Logger, *reject.Stats[rejection.Reason]) {
	t.Helper()

	logger, closeLogger, _ := logtest.New(t, log.Config{})
	t.Cleanup(func() { require.NoError(t, closeLogger()) })

	return logger, reject.New[rejection.Reason](logger, time.Hour, rejection.Names())
}

func newTestServer(t *testing.T, auth jwt.Auth) *Server {
	t.Helper()

	logger, rejections := newTestDeps(t)
	server, err := buildServer(testServerConfig(), auth, logger, rejections)
	require.NoError(t, err)
	return server
}
