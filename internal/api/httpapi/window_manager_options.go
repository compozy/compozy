package httpapi

import (
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/windowmanager"
)

type httpExtendedServices struct {
	resources         core.ResourceService
	extensions        ExtensionService
	windowManager     windowmanager.Service
	gateway           core.GatewayService
	gatewayAdmission  gateway.AdmissionController
	gatewayChallenges gateway.ChallengeResolver
	gatewayTier       gateway.Tier
	deviceAuth        gateway.DeviceAuthenticator
	authLimiter       *gateway.AuthFailureLimiter
	surfaceSet        SurfaceSet
}

// WithWindowManagerService injects the daemon-authoritative window manager.
func WithWindowManagerService(service windowmanager.Service) Option {
	return func(server *Server) {
		server.windowManager = service
	}
}
