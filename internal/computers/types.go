// Package computers owns the local controller's remote computer registry and connections.
package computers

import (
	"context"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

type Status string

const (
	Connected      Status = "connected"
	Connecting     Status = "connecting"
	Offline        Status = "offline"
	NeedsAttention Status = "needs_attention"
	NeedsPro       Status = "needs_pro"
	Incompatible   Status = "incompatible"
	Disabled       Status = "disabled"
)

const (
	DefaultWeight = config.DefaultComputerWeight
	PlanOK        = "ok"
	PlanLimited   = "limited"
)

type RemoveOptions struct {
	Unlink    bool
	Uninstall bool
}

type View struct {
	store.Computer
	Status      Status
	PlanState   string
	RouteInUse  *store.ComputerRoute
	RouteErrors map[string]string
	Scopes      []remoteproto.Scope
	Diagnosis   *Diagnosis
	Protocol    *remoteproto.ProtocolRange
}

type Diagnosis struct {
	Step    string
	Code    string
	Message string
	At      time.Time
}

type Account interface {
	IdentityLifetime() (context.Context, account.Installation)
	UseDevice(context.Context, func(context.Context, cloudclient.Credentials) error) error
}

type Cloud interface {
	Computers(context.Context, cloudclient.Credentials) ([]remoteproto.AccountComputer, error)
	ComputerGrant(
		context.Context,
		cloudclient.Credentials,
		remoteproto.ComputerID,
		remoteproto.RouteKind,
	) (string, error)
	UnlinkComputer(context.Context, remoteproto.ComputerID, cloudclient.Credentials) error
}

// ForwardResolver returns the loopback socket of an established SSH forward.
type ForwardResolver func(context.Context, store.ComputerRoute) (string, error)

type Failure struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
}

func (f *Failure) Error() string { return f.Code }

const (
	CodeDeviceRevoked  = "device_revoked"
	CodePlanRequired   = "plan_required"
	CodeLeaseExpired   = "remote_entitlement_unavailable"
	CodeSessionExpired = "remote_session_expired"
)

var (
	ErrNotFound      = &Failure{Status: http.StatusNotFound, Code: "computer_not_found", Message: "Computer not found."}
	ErrRouteNotFound = &Failure{
		Status: http.StatusNotFound, Code: "computer_route_not_found", Message: "Learned route not found.",
	}
	ErrOffline = &Failure{
		Status: http.StatusServiceUnavailable, Code: "computer_offline", Message: "The computer is offline.",
	}
	ErrDisabled = &Failure{
		Status: http.StatusConflict, Code: "computer_disabled", Message: "This computer is turned off here.",
	}
	ErrPlanRequired = &Failure{
		Status: http.StatusPaymentRequired, Code: CodePlanRequired, Message: "Hosted route is part of Pro.",
	}
	ErrIdentityMismatch = &Failure{
		Status: http.StatusBadGateway, Code: "route_identity_mismatch", Message: "Different computer at this address",
	}
	ErrPinMismatch = &Failure{
		Status: http.StatusBadGateway, Code: "tls_pin_mismatch", Message: "The computer's TLS identity has changed.",
	}
	ErrIncompatible = &Failure{
		Status: http.StatusConflict, Code: "computer_incompatible", Message: "Update the computer to connect.",
	}
	ErrClaimToken = &Failure{
		Status: http.StatusBadRequest, Code: "claim_token_not_remote",
		Message: "Task leases stay on the computer that owns them.",
	}
	ErrRouteNotRemote = &Failure{
		Status: http.StatusNotFound, Code: "route_not_remote", Message: "This route is available locally only.",
	}
)
