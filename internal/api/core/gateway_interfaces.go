package core

import (
	"context"

	"github.com/compozy/compozy/internal/gateway"
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
	RenameDevice(context.Context, string, string) (gateway.DeviceSession, error)
	MintStreamTicket(context.Context, string) (gateway.StreamTicket, error)
	ResolveIngressSubject(context.Context, gateway.IngressSubjectRef) (gateway.IngressSubject, error)
	BindIngress(context.Context, gateway.IngressBindRequest) (gateway.IngressBinding, error)
	UnbindIngress(context.Context, gateway.IngressUnbindRequest) (gateway.UnbindResult, error)
	ProjectIngress(context.Context, gateway.IngressSubjectRef) (gateway.IngressProjection, error)
}
