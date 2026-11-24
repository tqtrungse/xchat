/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package jwt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"xchat/pkg/jitter"
	"xchat/pkg/log"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

const (
	clockSkew = 30 * time.Second

	// jwksRefreshInterval is the interval between two refreshes of a JWKS
	// that has keys.
	jwksRefreshInterval = 15 * time.Minute

	// jwksRateLimitWaitMax limits the wait of a token with an unknown key ID
	// for the refresh rate limiter. The keyfunc default is one minute. That
	// default blocks the request.
	jwksRateLimitWaitMax = time.Second
)

type verifier struct {
	jwks keyfunc.Keyfunc
	// cancel stops the refresh goroutine of this verifier.
	cancel context.CancelFunc
}

type auth[T any] struct {
	current atomic.Pointer[verifier]
	config  AuthConfig
	logger  *log.Logger
	options options
}

// NewAuth gives an Auth that verifies tokens with the keys of the JWKS at
// config.JWKSURL. Verify decodes the "ext" claim of each token into a T.
// NewAuth waits no more than the request timeout for the first fetch. If the
// first fetch fails, NewAuth logs the error and retries in the background.
// Ready reports false until the JWKS has a key. A canceled ctx stops the
// retry.
func NewAuth[T any](
	ctx context.Context,
	config AuthConfig,
	logger *log.Logger,
) (Auth[T], error) {
	return newAuth[T](ctx, config, logger, defaultOptions)
}

// Verify checks the token and decodes its claims into a new ExtClaims.
// It returns ErrTokenInvalid if the token is not valid. This includes a claim
// that does not decode into its field, for example a string in a number field
// of T.
func (a *auth[T]) Verify(token string) (*ExtClaims[T], error) {
	claims := new(ExtClaims[T])
	v := a.current.Load()
	tk, err := jwt.ParseWithClaims(
		token,
		claims,
		v.jwks.Keyfunc,
		// Force signature algorithm to prevent algorithm confusion attacks.
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(a.config.Issuer),
		jwt.WithAudience(a.config.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		// A keyfunc failure with no key in the storage means that the JWKS is
		// not available. All other failures mean that the token is not valid.
		if errors.Is(err, jwt.ErrTokenUnverifiable) && !a.ready(v) {
			return nil, fmt.Errorf("%w: %w", ErrVerifierUnavailable, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrTokenInvalid, err)
	}
	if !tk.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

// Ready reports whether the verifier has at least one verification key. A
// deployment can call Ready before it sends traffic to this service.
func (a *auth[T]) Ready() bool {
	return a.ready(a.current.Load())
}

func newAuth[T any](
	ctx context.Context,
	config AuthConfig,
	logger *log.Logger,
	opts options,
) (Auth[T], error) {
	// These checks do not depend on the config layer. They keep the
	// invariant of this package: an Auth has each OIDC setting.
	if config.Issuer == "" {
		return nil, ErrIssuerRequired
	}
	if err := validateURL(config.Issuer, ErrIssuerInsecure); err != nil {
		return nil, err
	}
	// The aud claim is a free string (RFC 7519).
	if config.Audience == "" {
		return nil, ErrAudienceRequired
	}
	if config.JWKSURL == "" {
		return nil, ErrJWKSURLRequired
	}
	if err := validateURL(config.JWKSURL, ErrJWKSURLInsecure); err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, ErrLoggerRequired
	}

	retryDelay, err := jitter.New(opts.retryMin, opts.retryMax)
	if err != nil {
		return nil, fmt.Errorf("invalid JWKS retry delay: %w", err)
	}

	a := &auth[T]{
		config:  config,
		logger:  logger.Component("jwt").With(zap.String("jwks_url", config.JWKSURL)),
		options: opts,
	}

	v, err := a.newVerifier(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS client: %w", err)
	}

	a.current.Store(v)
	if !a.ready(v) {
		go a.retryUntilReady(ctx, v, retryDelay)
	}
	return a, nil
}

// newVerifier creates a JWKS client. If strict is true, newVerifier returns an
// error when the first fetch fails. Otherwise, newVerifier writes the error to
// the log.
func (a *auth[T]) newVerifier(ctx context.Context, strict bool) (*verifier, error) {
	verifierCtx, cancel := context.WithCancel(ctx)
	noErr := !strict
	jwks, err := keyfunc.NewDefaultOverrideCtx(
		verifierCtx,
		[]string{a.config.JWKSURL},
		keyfunc.Override{
			HTTPTimeout:               a.options.requestTimeout,
			RefreshInterval:           jwksRefreshInterval,
			RateLimitWaitMax:          jwksRateLimitWaitMax,
			NoErrorReturnFirstHTTPReq: &noErr,
			RefreshErrorHandlerFunc:   a.refreshErrorHandler(verifierCtx),
		},
	)
	if err != nil {
		// Stop the refresh goroutine that keyfunc has already started.
		cancel()
		return nil, err
	}
	return &verifier{
		jwks:   jwks,
		cancel: cancel,
	}, nil
}

// refreshErrorHandler writes a failed JWKS fetch to the logger of the service.
// Without it, keyfunc writes the error to slog.Default. The JWKS library
// cancels the request context before it calls the handler for the first fetch
// and for a periodic refresh. Therefore, the handler checks verifierCtx. The
// verifier cancels this context only when it stops.
func (a *auth[T]) refreshErrorHandler(verifierCtx context.Context) func(string) func(context.Context, error) {
	return func(string) func(context.Context, error) {
		return func(_ context.Context, err error) {
			if verifierCtx.Err() != nil {
				return
			}
			a.logger.Error("failed to refresh JWKS", zap.Error(err))
		}
	}
}

// retryUntilReady creates a new verifier until one has a key. Then it
// replaces the current verifier with the new one. The delay value sets the
// wait between two attempts. The wait increases to retryMax and has jitter.
// The jitter prevents all pods of a deployment from fetching the JWKS at the
// same time. Only this goroutine uses delay.
func (a *auth[T]) retryUntilReady(ctx context.Context, v *verifier, delay *jitter.Delay) {
	wait := delay.Next()
	for attempt := 1; !a.ready(v); attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		next, err := a.newVerifier(ctx, true)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			wait = delay.Next()
			a.logger.Warn(
				"failed to fetch JWKS, retrying",
				zap.Int("attempt", attempt),
				zap.Duration("next_wait", wait),
				zap.Error(err),
			)
			continue
		}
		// Use the new verifier and stop the refresh goroutine of the old one.
		a.current.Swap(next).cancel()
		v = next
	}
	a.logger.Info("JWKS is ready")
}

func (a *auth[T]) ready(v *verifier) bool {
	keys, err := v.jwks.Storage().KeyReadAll(context.Background())
	if err != nil {
		return false
	}
	return len(keys) > 0
}

// validateURL accepts an https URL. It also accepts an http URL to a loopback
// host, so that a developer can run a local IdP without TLS. A loopback
// request does not leave the host.
func validateURL(raw string, insecureErr error) error {
	u, err := url.Parse(raw) // url.Parse lowercases the scheme
	if err != nil || u.Host == "" {
		return fmt.Errorf("%w: %q", insecureErr, raw)
	}

	switch u.Scheme {
	case "https":
		return nil

	case "http":
		if isLoopback(u.Hostname()) {
			return nil
		}
	}
	return fmt.Errorf("%w: %q", insecureErr, raw)
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
