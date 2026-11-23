package core

import (
	"context"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remotehost"
)

// GatewayService exposes remote reachability and device lifecycle operations.
type GatewayService interface {
	Status(context.Context) (gateway.Status, error)
	Audit(context.Context) (gateway.AuditReport, error)
	SetSurfaceExposure(context.Context, gateway.SurfaceExposureRequest) (gateway.Status, error)
	EnableProvider(context.Context, gateway.ProviderEnableRequest) (gateway.Status, error)
	DisableProvider(context.Context, gateway.Tier, string) (gateway.Status, error)
	MintPairing(context.Context, gateway.PairingRequest) (gateway.PairingArtifact, error)
	RedeemPairing(context.Context, gateway.RedeemRequest) (gateway.IssuedCredential, error)
	ListDevices(context.Context) ([]gateway.DeviceSession, error)
	RevokeDevice(context.Context, string) (gateway.RevokeResult, error)
	RemoveDevice(context.Context, string) (gateway.RemoveResult, error)
	RenameDevice(context.Context, string, string) (gateway.DeviceSession, error)
	MintStreamTicket(context.Context, string) (remotehost.StreamTicket, error)
}

// IngressService manages confirmed webhook bindings and their public projections.
type IngressService interface {
	Bind(context.Context, ingress.BindRequest) (ingress.Binding, error)
	Unbind(context.Context, ingress.UnbindRequest) (ingress.UnbindResult, error)
	Project(context.Context, ingress.SubjectRef) (ingress.Projection, error)
}
