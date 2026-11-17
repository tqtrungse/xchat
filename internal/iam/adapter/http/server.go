/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"xchat/internal/iam/core/rejection"

	"xchat/pkg/collection"
	"xchat/pkg/jwt"
	"xchat/pkg/log"
	"xchat/pkg/reject"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type contextKey uint8

const (
	claimsContextKey contextKey = iota
	bearerRealm                 = "iam"
	healthzPath                 = "/healthz"
	readyzPath                  = "/readyz"
)

// ClaimsFromContext gives the claims that authenticate attached to the
// request. The second result is false if authenticate did not run.
func ClaimsFromContext(c *gin.Context) (*jwt.Claims, bool) {
	value, exists := c.Get(claimsContextKey)
	if !exists {
		return nil, false
	}
	claims, exists := value.(*jwt.Claims)
	return claims, exists
}

// Server serves the IAM HTTP API over HTTPS with a gin router.
type Server struct {
	engine     *gin.Engine
	httpServer *http.Server
	auth       jwt.Auth
	rejections *reject.Stats[rejection.Reason]
	public     *gin.RouterGroup
	api        *gin.RouterGroup
	certFile   string
	keyFile    string
}

// NewServer makes a Server from config. NewServer returns an error if a
// required field is empty, if the logger is nil, if the rejection stats are
// nil, or if the OIDC provider settings are not valid.
func NewServer(
	ctx context.Context,
	config Config,
	logger *log.Logger,
	rejections *reject.Stats[rejection.Reason],
) (*Server, error) {
	err := config.validate()
	if err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, errLoggerRequired
	}
	if rejections == nil {
		return nil, errRejectionsRequired
	}

	auth, err := jwt.NewAuth(
		ctx,
		jwt.AuthConfig{
			Issuer:   config.OIDCIssuer,
			Audience: config.OIDCAudience,
			JWKSURL:  config.OIDCJWKSURL,
		},
		logger,
	)
	if err != nil {
		return nil, err
	}

	return buildServer(config, auth, logger, rejections)
}

// buildServer makes a Server with a valid config. buildServer puts the
// default value in each timeout that is zero. The logger and the rejection
// stats must not be nil.
func buildServer(
	config Config,
	auth jwt.Auth,
	logger *log.Logger,
	rejections *reject.Stats[rejection.Reason],
) (*Server, error) {
	config.applyDefault()

	// The default gin mode writes each route to the log. Use the release mode
	// unless the GIN_MODE environment variable selects a mode.
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	s := &Server{
		engine:     gin.New(),
		auth:       auth,
		rejections: rejections,
		certFile:   config.CertFile,
		keyFile:    config.KeyFile,
	}

	// Do not trust the forwarding headers of a proxy by default. A deployment
	// behind a trusted proxy must call SetTrustedProxies with the proxy CIDRs.
	err := s.engine.SetTrustedProxies(nil)
	if err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	s.engine.Use(recoverPanic(logger))
	// A probe runs every few seconds on each pod. An access log entry for each
	// probe hides the real requests and fills the log queue.
	s.engine.Use(requestLogger(logger, collection.NewSet[string](healthzPath, readyzPath)))
	s.engine.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	})
	// The /healthz route reports liveness. The /readyz route reports readiness.
	// The /readyz route returns 503 until the server can verify a token.
	s.public = s.engine.Group("")
	s.public.GET(healthzPath, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	s.public.GET(readyzPath, func(c *gin.Context) {
		if !s.Ready() {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusNoContent)
	})
	s.api = s.engine.Group("/api")
	s.api.Use(s.authenticate)

	s.httpServer = &http.Server{
		Addr:              config.Addr,
		Handler:           s.engine,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
	}

	return s, nil
}

// APIGroup gives the authenticated router group with the /api prefix. Register
// each API route before Run.
func (s *Server) APIGroup() *gin.RouterGroup {
	return s.api
}

// PublicGroup gives the router group without authentication. Use it only for
// public metadata. The server already registers GET /healthz and GET /readyz
// on this group. Do not register these paths again: gin panics on a duplicate
// route. The access log skips these two routes.
func (s *Server) PublicGroup() *gin.RouterGroup {
	return s.public
}

// Ready reports whether the server can verify a token. A deployment can call
// Ready before it sends traffic to the server.
func (s *Server) Ready() bool {
	return s.auth.Ready()
}

// Run listens on Config.Addr and serves HTTPS. Run returns nil after Shutdown.
// Run returns an error if the server cannot listen on the address or cannot
// load the certificate.
func (s *Server) Run() error {
	err := s.httpServer.ListenAndServeTLS(s.certFile, s.keyFile)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve %s: %w", s.httpServer.Addr, err)
	}
	return nil
}

// Shutdown stops the server and waits for each active request. Shutdown
// returns the error of ctx when ctx ends before the wait. It then closes each
// connection at once.
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("shutdown: %w", err), s.httpServer.Close())
	}
	return nil
}

// requestLogger writes one access log entry after each request. It skips a
// request whose route is in skipRoutes. The route is the gin route pattern,
// not the request path. The entry does not contain the query string or an
// authorization header.
func requestLogger(logger *log.Logger, skipRoutes collection.Set[string]) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if skipRoutes.Contains(c.FullPath()) {
			return
		}
		logger.Info(
			"http request",
			zap.String("method", c.Request.Method),
			zap.String("route", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
			zap.Int("errors", len(c.Errors)),
		)
	}
}

// recoverPanic logs each panic and returns a 500 response. The logger receives
// the stack, not stderr.
func recoverPanic(logger *log.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		logger.Error(
			"panic",
			zap.Any("panic", recovered),
			zap.ByteString("stack", debug.Stack()),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal_error",
		})
	})
}
