package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

const appTokenOriginKey = "compozy.app_token_origin"
const appTokenIDKey = "compozy.app_token_id"

// AppAccessPolicy resolves a physical route to its canonical operation decisions.
type AppAccessPolicy func(method, route string) bool

func WithAppTokenAuth(auth gateway.AppTokenAuthenticator, policy AppAccessPolicy) Option {
	return func(server *Server) { server.appTokenAuth = auth; server.appAccess = policy }
}

func WithAppTokenAuthLimiter(limiter *gateway.AuthFailureLimiter) Option {
	return func(server *Server) { server.appTokenLimiter = limiter }
}

func (s *Server) appTokenAuthMiddleware() gin.HandlerFunc {
	limiter := s.appTokenLimiter
	if limiter == nil {
		limiter = gateway.NewAuthFailureLimiter(
			gateway.AppTokenAuthFailureLimit,
			gateway.AppTokenAuthFailureWindow,
			s.now,
		)
	}
	return func(c *gin.Context) { s.admitAppTokenRequest(c, limiter) }
}

func (s *Server) admitAppTokenRequest(c *gin.Context, limiter *gateway.AuthFailureLimiter) {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	_, ownOrigin := resolveAllowedOrigin(origin, requestSchemeForTarget(c.Request, false), c.Request.Host, s.host)
	foreign := origin != "" && !ownOrigin
	secret := appTokenBearer(c.Request)
	if !foreign && !gateway.HasAppTokenPrefix(secret) {
		c.Next()
		return
	}
	normalized, normalizeErr := gateway.NormalizeAppOrigin(origin)
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
		core.RespondAppTokenError(c, gateway.ErrAppOriginNotAllowed, true)
		return false
	}
	allowed := s.appTokenAuth != nil && s.appTokenAuth.AppTokenOriginAllowed(normalized)
	if allowed {
		setBrowserAccessHeaders(c.Writer.Header())
		c.Header("Access-Control-Allow-Origin", origin)
	}
	if !allowed && (c.Request.Method == http.MethodOptions || secret == "") {
		s.warnAppTokenAccess(c, "app_token.origin_denied", normalized, "")
		core.RespondAppTokenError(c, gateway.ErrAppOriginNotAllowed, true)
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
	c *gin.Context, limiter *gateway.AuthFailureLimiter, source, secret string, foreign bool,
) (gateway.AppToken, bool) {
	if !limiter.Allow(source, false) {
		s.warnAppTokenAccess(c, "app_token.rate_limited", source, "")
		c.Header("Retry-After", "60")
		core.RespondAppTokenError(c, gateway.ErrAppTokenRateLimited, true)
		return gateway.AppToken{}, false
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
		core.RespondAppTokenError(c, gateway.ErrAppTokenRequired, true)
		return gateway.AppToken{}, false
	}
	if s.appTokenAuth == nil {
		failed = true
		core.RespondAppTokenError(c, gateway.ErrAppTokenUnauthenticated, true)
		return gateway.AppToken{}, false
	}
	token, err := s.appTokenAuth.AuthenticateAppToken(c.Request.Context(), secret)
	if err != nil {
		failed = errors.Is(err, gateway.ErrAppTokenUnauthenticated) || errors.Is(err, gateway.ErrAppTokenRevoked)
		core.RespondAppTokenError(c, err, true)
		return gateway.AppToken{}, false
	}
	if foreign && !slices.Contains(token.Origins, source) {
		failed = true
		s.warnAppTokenAccess(c, "app_token.origin_denied", source, token.Session.ID)
		core.RespondAppTokenError(c, gateway.ErrAppOriginNotAllowed, true)
		return gateway.AppToken{}, false
	}
	if foreign && (s.appAccess == nil || !s.appAccess(c.Request.Method, c.FullPath())) {
		s.warnAppTokenAccess(c, "app_token.route_forbidden", source, token.Session.ID)
		core.RespondAppTokenError(c, gateway.ErrAppTokenRouteForbidden, true)
		return gateway.AppToken{}, false
	}
	return token, true
}

func (s *Server) serveAppTokenRequest(c *gin.Context, token gateway.AppToken, foreign bool, origin string) {
	device := token.Session
	ctx := core.ContextWithAppToken(c.Request.Context(), token)
	if isGatewayMutationRequest(c.Request) {
		mutationCtx, err := s.appTokenAuth.AcquireMutation(ctx, device.ID, device.RevokeEpoch)
		if err != nil {
			core.RespondAppTokenError(c, err, true)
			return
		}
		defer gateway.ReleaseMutation(mutationCtx)
		ctx = store.ContextWithMutationCommitFence(mutationCtx, func(commitCtx context.Context) error {
			return s.appTokenAuth.RevalidateForCommit(commitCtx, device.ID, device.RevokeEpoch)
		})
	}
	if isGatewayStreamRequest(c.Request) {
		registry := s.appTokenAuth.Connections()
		if registry == nil {
			core.RespondAppTokenError(c, gateway.ErrAppTokenUnauthenticated, true)
			return
		}
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan struct{})
		handle := registry.RegisterWaitable(device.ID, cancel, done)
		defer func() { close(done); registry.Deregister(device.ID, handle) }()
		ctx = streamCtx
	}
	if err := s.appTokenAuth.RevalidateForCommit(ctx, device.ID, device.RevokeEpoch); err != nil {
		core.RespondAppTokenError(c, err, true)
		return
	}
	if err := s.appTokenAuth.TouchAppTokenLastSeen(ctx, device.ID, device.RevokeEpoch); err != nil {
		core.RespondAppTokenError(c, err, true)
		return
	}
	if foreign {
		c.Set(appTokenOriginKey, origin)
	}
	c.Set(appTokenIDKey, device.ID)
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

func rejectTierAppTokens() gin.HandlerFunc {
	return func(c *gin.Context) {
		if gateway.HasAppTokenPrefix(appTokenBearer(c.Request)) {
			abortAppTokenAccess(
				c,
				http.StatusForbidden,
				"gateway_app_token_local_only",
				"app tokens work on the local listener only",
			)
			return
		}
		c.Next()
	}
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

func abortAppTokenAccess(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, contract.ErrorPayload{Error: message, Code: code})
}
