package httpapi

import (
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
)

type httpExtendedServices struct {
	resources         core.ResourceService
	extensions        ExtensionService
	windowManager     core.WindowManagerProvider
	terminal          core.TerminalProvider
	gateway           core.GatewayService
	gatewayAdmission  gateway.AdmissionController
	gatewayChallenges gateway.ChallengeResolver
	gatewayTier       gateway.Tier
	deviceAuth        gateway.DeviceAuthenticator
	authLimiter       *gateway.AuthFailureLimiter
	ingressLimiter    *gateway.IngressRateLimiter
	surfaceSet        SurfaceSet
}

// WithWindowManagerProvider injects the per-profile window managers.
func WithWindowManagerProvider(provider core.WindowManagerProvider) Option {
	return func(server *Server) {
		server.windowManager = provider
	}
}

// WithTerminalProvider injects the per-profile terminal managers.
func WithTerminalProvider(provider core.TerminalProvider) Option {
	return func(server *Server) {
		server.terminal = provider
	}
}
