package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// #nosec G101 -- this is a Gin request-context key, not credential material.
const appTokenOriginKey = "compozy.app_token_origin"

// #nosec G101 -- this is a Gin request-context key, not credential material.
const appTokenIDKey = "compozy.app_token_id"

// AppAccessPolicy resolves a physical route to its canonical operation decisions.
type AppAccessPolicy func(method, route string) bool

type AppTokenAuthenticator interface {
	AuthenticateAppToken(context.Context, string) (apptoken.Token, error)
	TouchAppTokenLastSeen(context.Context, string, uint64) error
	AppTokenOriginAllowed(string) bool
	AcquireMutation(context.Context, string, uint64) (context.Context, error)
	RevalidateForCommit(context.Context, string, uint64) error
	Connections() remotehost.ConnectionRegistry
}

func WithAppTokenAuth(auth AppTokenAuthenticator, policy AppAccessPolicy) Option {
	return func(server *Server) { server.appTokenAuth = auth; server.appAccess = policy }
}

func WithAppTokenAuthLimiter(limiter *remotehost.AuthFailureLimiter) Option {
	return func(server *Server) { server.appTokenLimiter = limiter }
}

func (s *Server) appTokenAuthMiddleware() gin.HandlerFunc {
	limiter := s.appTokenLimiter
	if limiter == nil {
		limiter = remotehost.NewAuthFailureLimiter(
			apptoken.AuthFailureLimit,
			apptoken.AuthFailureWindow,
			s.now,
		)
	}
	return func(c *gin.Context) { s.admitAppTokenRequest(c, limiter) }
}

func (s *Server) admitAppTokenRequest(c *gin.Context, limiter *remotehost.AuthFailureLimiter) {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	_, ownOrigin := resolveAllowedOrigin(origin, requestSchemeForTarget(c.Request, false), c.Request.Host, s.host)
	foreign := origin != "" && !ownOrigin
	secret := appTokenBearer(c.Request)
	if !foreign && !apptoken.HasPrefix(secret) {
		c.Next()
		return
	}
	normalized, normalizeErr := apptoken.NormalizeOrigin(origin)
	if foreign && !s.admitAppTokenOrigin(c, origin, normalized, normalizeErr, secret) {
		return
	}
	source := normalized
	if origin == "" {
		source = "no-origin"
	} else if normalizeErr != nil {
		source = "invalid-origin"
	}
	token, ok := s.authenticateAppToken(c, limiter, source, secret, foreign)
	if !ok {
		return
	}
	s.serveAppTokenRequest(c, token, foreign, origin)
}

func (s *Server) admitAppTokenOrigin(
	c *gin.Context,
	origin, normalized string,
	normalizeErr error,
	secret string,
) bool {
	if normalizeErr != nil {
		s.warnAppTokenAccess(c, "app_token.origin_denied", "invalid-origin", "")
		core.RespondAppTokenError(c, apptoken.ErrOriginNotAllowed, true)
		return false
	}
	allowed := s.appTokenAuth != nil && s.appTokenAuth.AppTokenOriginAllowed(normalized)
	if allowed {
		setBrowserAccessHeaders(c.Writer.Header())
		c.Header("Access-Control-Allow-Origin", origin)
	}
	if !allowed && (c.Request.Method == http.MethodOptions || secret == "") {
		s.warnAppTokenAccess(c, "app_token.origin_denied", normalized, "")
		core.RespondAppTokenError(c, apptoken.ErrOriginNotAllowed, true)
		return false
	}
	if c.Request.Method == http.MethodOptions {
		c.Set(appTokenOriginKey, origin)
		c.Header("Access-Control-Max-Age", "600")
		c.Next()
		return false
	}
	return true
}

func (s *Server) authenticateAppToken(
	c *gin.Context, limiter *remotehost.AuthFailureLimiter, source, secret string, foreign bool,
) (apptoken.Token, bool) {
	if !limiter.Allow(source, false) {
		s.warnAppTokenAccess(c, "app_token.rate_limited", source, "")
		c.Header("Retry-After", "60")
		core.RespondAppTokenError(c, apptoken.ErrRateLimited, true)
		return apptoken.Token{}, false
	}
	failed := false
	defer func() {
		if failed {
			limiter.Failure(source, false)
		} else {
			limiter.Complete(source)
		}
	}()
	if secret == "" {
		failed = true
		s.warnAppTokenAuthFailure(c, source, apptoken.ErrRequired)
		core.RespondAppTokenError(c, apptoken.ErrRequired, true)
		return apptoken.Token{}, false
	}
	if s.appTokenAuth == nil {
		failed = true
		s.warnAppTokenAuthFailure(c, source, apptoken.ErrUnauthenticated)
		core.RespondAppTokenError(c, apptoken.ErrUnauthenticated, true)
		return apptoken.Token{}, false
	}
	token, err := s.appTokenAuth.AuthenticateAppToken(c.Request.Context(), secret)
	if err != nil {
		failed = errors.Is(err, apptoken.ErrUnauthenticated) || errors.Is(err, apptoken.ErrRevoked)
		if failed {
			s.warnAppTokenAuthFailure(c, source, err)
		}
		core.RespondAppTokenError(c, err, true)
		return apptoken.Token{}, false
	}
	if foreign && !slices.Contains(token.Origins, source) {
		failed = true
		s.warnAppTokenAccess(c, "app_token.origin_denied", source, token.ID)
		core.RespondAppTokenError(c, apptoken.ErrOriginNotAllowed, true)
		return apptoken.Token{}, false
	}
	if foreign && (s.appAccess == nil || !s.appAccess(c.Request.Method, c.FullPath())) {
		s.warnAppTokenAccess(c, "app_token.route_forbidden", source, token.ID)
		core.RespondAppTokenError(c, apptoken.ErrRouteForbidden, true)
		return apptoken.Token{}, false
	}
	return token, true
}

func (s *Server) serveAppTokenRequest(c *gin.Context, token apptoken.Token, foreign bool, origin string) {
	ctx := core.ContextWithAppToken(c.Request.Context(), token)
	if isAppTokenMutationRequest(c.Request) {
		mutationCtx, err := s.appTokenAuth.AcquireMutation(ctx, token.ID, token.RevokeEpoch)
		if err != nil {
			core.RespondAppTokenError(c, err, true)
			return
		}
		defer remotehost.ReleaseMutation(mutationCtx)
		ctx = store.ContextWithMutationCommitFence(mutationCtx, func(commitCtx context.Context) error {
			return s.appTokenAuth.RevalidateForCommit(commitCtx, token.ID, token.RevokeEpoch)
		})
	}
	if isAppTokenStreamRequest(c.Request) {
		registry := s.appTokenAuth.Connections()
		if registry == nil {
			core.RespondAppTokenError(c, apptoken.ErrUnauthenticated, true)
			return
		}
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan struct{})
		handle := registry.RegisterWaitable(token.ID, cancel, done)
		defer func() { close(done); registry.Deregister(token.ID, handle) }()
		ctx = streamCtx
	}
	if err := s.appTokenAuth.RevalidateForCommit(ctx, token.ID, token.RevokeEpoch); err != nil {
		core.RespondAppTokenError(c, err, true)
		return
	}
	if err := s.appTokenAuth.TouchAppTokenLastSeen(ctx, token.ID, token.RevokeEpoch); err != nil {
		core.RespondAppTokenError(c, err, true)
		return
	}
	if foreign {
		c.Set(appTokenOriginKey, origin)
	}
	c.Set(appTokenIDKey, token.ID)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func appTokenBearer(request *http.Request) string {
	parts := strings.Fields(request.Header.Get("Authorization"))
	if len(parts) >= 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.Join(parts[1:], " ")
	}
	return ""
}

func (s *Server) warnAppTokenAccess(c *gin.Context, event, origin, tokenID string) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.WarnContext(c.Request.Context(), event,
		"origin", origin, "remote", c.Request.RemoteAddr,
		"app_token_id", tokenID, "route", c.FullPath())
}

func (s *Server) warnAppTokenAuthFailure(c *gin.Context, origin string, err error) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	_, reason := core.AppTokenErrorStatus(err, true)
	logger.WarnContext(c.Request.Context(), "app_token.auth_failed",
		"origin", origin, "remote", c.Request.RemoteAddr, "reason", reason)
}
