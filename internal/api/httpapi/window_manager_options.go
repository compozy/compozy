package httpapi

import (
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remotehost"
)

type httpExtendedServices struct {
	resources         core.ResourceService
	extensions        ExtensionService
	windowManager     core.WindowManagerProvider
	terminal          core.TerminalProvider
	appTokens         core.AppTokenManager
	openAPI           *core.OpenAPIDocument
	gateway           core.GatewayService
	ingress           core.IngressService
	gatewayAdmission  gateway.AdmissionController
	gatewayChallenges gateway.ChallengeResolver
	gatewayTier       gateway.Tier
	appTokenAuth      AppTokenAuthenticator
	appTokenLimiter   *remotehost.AuthFailureLimiter
	appAccess         AppAccessPolicy
	deviceAuth        gateway.DeviceAuthenticator
	authLimiter       *remotehost.AuthFailureLimiter
	pairingLimiter    *remotehost.AuthFailureLimiter
	ingressLimiter    *ingress.RateLimiter
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
