package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

const (
	gatewayDeviceCookieName = "compozy_gateway_device"
	gatewayStreamTicketKey  = "ticket"
)

type deviceConnectionRegistryProvider interface {
	Connections() gateway.ConnectionRegistry
}

func (h *Handlers) deviceAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.deviceAuth == nil {
			abortGatewayError(c, gateway.ErrDeviceUnauthenticated)
			return
		}
		source := gatewayAuthSource(c.Request)
		limiter := h.authLimiter
		if limiter == nil {
			abortGatewayError(c, gateway.ErrDeviceUnauthenticated)
			return
		}
		if !limiter.Allow(source, false) {
			abortGatewayError(c, gateway.ErrAuthRateLimited)
			return
		}

		if isGatewayStreamRequest(c.Request) {
			h.authenticateGatewayStream(c, limiter, source)
			return
		}
		credential := gatewayCredentialFromRequest(c.Request)
		device, err := h.deviceAuth.Authenticate(c.Request.Context(), credential)
		if err != nil {
			limiter.Failure(source, false)
			abortGatewayError(c, err)
			return
		}
		limiter.Success(source)
		requestCtx, err := h.authenticatedGatewayRequestContext(c.Request.Context(), c.Request, device)
		if err != nil {
			abortGatewayError(c, err)
			return
		}
		defer gateway.ReleaseMutation(requestCtx)
		c.Request = c.Request.WithContext(requestCtx)
		c.Next()
	}
}

func (h *Handlers) authenticateGatewayStream(c *gin.Context, limiter *gateway.AuthFailureLimiter, source string) {
	ticket := strings.TrimSpace(c.Query(gatewayStreamTicketKey))
	device, err := h.deviceAuth.ConsumeStreamTicket(c.Request.Context(), ticket)
	if err != nil {
		limiter.Failure(source, false)
		abortGatewayError(c, err)
		return
	}
	provider, ok := h.deviceAuth.(deviceConnectionRegistryProvider)
	if !ok {
		abortGatewayError(c, gateway.ErrDeviceUnauthenticated)
		return
	}
	registry := provider.Connections()
	if registry == nil {
		abortGatewayError(c, gateway.ErrDeviceUnauthenticated)
		return
	}

	authenticatedCtx, err := h.authenticatedGatewayRequestContext(c.Request.Context(), c.Request, device)
	if err != nil {
		abortGatewayError(c, err)
		return
	}
	streamContext, cancel := context.WithCancel(authenticatedCtx)
	done := make(chan struct{})
	handle := registry.RegisterWaitable(device.ID, cancel, done)
	defer func() {
		cancel()
		close(done)
		registry.Deregister(device.ID, handle)
		gateway.ReleaseMutation(authenticatedCtx)
	}()
	if err := h.deviceAuth.RevalidateForCommit(streamContext, device.ID, device.RevokeEpoch); err != nil {
		abortGatewayError(c, gateway.ErrStreamTicketInvalid)
		return
	}
	limiter.Success(source)
	c.Request = c.Request.WithContext(streamContext)
	c.Next()
}

func (h *Handlers) authenticatedGatewayRequestContext(
	ctx context.Context,
	request *http.Request,
	device gateway.DeviceSession,
) (context.Context, error) {
	authenticatedCtx := gateway.ContextWithDevice(ctx, device)
	if !isGatewayMutationRequest(request) {
		return authenticatedCtx, nil
	}
	mutationCtx, err := h.deviceAuth.AcquireMutation(authenticatedCtx, device.ID, device.RevokeEpoch)
	if err != nil {
		return nil, err
	}
	return store.ContextWithMutationCommitFence(mutationCtx, func(commitCtx context.Context) error {
		return h.deviceAuth.RevalidateForCommit(commitCtx, device.ID, device.RevokeEpoch)
	}), nil
}

func isGatewayMutationRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func gatewayCredentialFromRequest(request *http.Request) string {
	if request == nil {
		return ""
	}
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	if parts := strings.Fields(authorization); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	cookie, err := request.Cookie(gatewayDeviceCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func gatewayAuthSource(request *http.Request) string {
	if request == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	if remote := strings.TrimSpace(request.RemoteAddr); remote != "" {
		return remote
	}
	return "unknown"
}

func isGatewayStreamRequest(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	path := strings.TrimSuffix(request.URL.Path, "/")
	if request.Method == http.MethodPost {
		return isSessionPromptStreamPath(path)
	}
	if request.Method != http.MethodGet {
		return false
	}
	if strings.HasSuffix(path, "/stream") || strings.HasSuffix(path, "/catalog-stream") ||
		strings.HasSuffix(path, "/log-tail") {
		return true
	}
	if strings.Contains(path, "/loop-runs/") && strings.HasSuffix(path, "/events") {
		return true
	}
	if !strings.Contains(path, "/extensions/") || !strings.HasSuffix(path, "/logs") {
		return false
	}
	follow, err := core.ParseOptionalBool(request.URL.Query().Get("follow"))
	return err == nil && follow
}

func isSessionPromptStreamPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 6 && parts[0] == "api" && parts[1] == "workspaces" &&
		strings.TrimSpace(parts[2]) != "" && parts[3] == "sessions" &&
		strings.TrimSpace(parts[4]) != "" && parts[5] == "prompt"
}

func abortGatewayError(c *gin.Context, err error) {
	status := core.StatusForGatewayError(err)
	payload := core.ErrorPayloadForStatus(status, err, true)
	payload.Code = core.GatewayErrorCode(err)
	c.AbortWithStatusJSON(status, payload)
}
