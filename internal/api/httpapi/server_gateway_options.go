package httpapi

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
)

// SurfaceSet identifies the route inventory fixed at listener construction.
type SurfaceSet string

const (
	SurfaceSetLocal          SurfaceSet = "local"
	SurfaceSetPrivate        SurfaceSet = "private"
	SurfaceSetPublicIngress  SurfaceSet = "public_ingress"
	SurfaceSetPublicOperator SurfaceSet = "public_operator"
	SurfaceSetPublicCombined SurfaceSet = "public_combined"
)

func (s SurfaceSet) validate() error {
	switch s {
	case SurfaceSetLocal, SurfaceSetPrivate, SurfaceSetPublicIngress,
		SurfaceSetPublicOperator, SurfaceSetPublicCombined:
		return nil
	default:
		return fmt.Errorf("httpapi: unsupported surface set %q", s)
	}
}

func (s SurfaceSet) hasOperatorRoutes() bool {
	return s == SurfaceSetPrivate || s == SurfaceSetPublicOperator || s == SurfaceSetPublicCombined
}

// WithSurfaceSet fixes the listener's route inventory without trusting request metadata.
func WithSurfaceSet(surfaceSet SurfaceSet) Option {
	return func(server *Server) {
		server.surfaceSet = surfaceSet
	}
}

// WithDeviceAuth installs the device credential authority for tier listeners.
func WithDeviceAuth(auth gateway.DeviceAuthenticator) Option {
	return func(server *Server) {
		server.deviceAuth = auth
	}
}

// WithGatewayService installs gateway management handlers.
func WithGatewayService(service core.GatewayService) Option {
	return func(server *Server) {
		server.gateway = service
		if admission, ok := service.(gateway.AdmissionController); ok {
			server.gatewayAdmission = admission
		}
	}
}

// WithGatewayAdmission installs the fail-closed request gate for tier listeners.
func WithGatewayAdmission(admission gateway.AdmissionController) Option {
	return func(server *Server) {
		server.gatewayAdmission = admission
	}
}

// WithGatewayAuthLimiter overrides failed-auth throttling, primarily for deterministic tests.
func WithGatewayAuthLimiter(limiter *gateway.AuthFailureLimiter) Option {
	return func(server *Server) {
		server.authLimiter = limiter
	}
}
