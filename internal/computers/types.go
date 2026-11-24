// Package computers owns the local controller's remote computer registry and connections.
package computers

import (
	"context"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
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
	DefaultWeight = 50
	PlanOK        = "ok"
	PlanLimited   = "limited"
)

type Computer struct {
	ID         remoteproto.ComputerID
	AccountID  string
	Name       string
	Enabled    bool
	Platform   string
	Version    string
	TLSRootSHA string
	Weight     int
	Routes     []Route
	AddedAt    time.Time
	LastConnAt *time.Time
}

type Route struct {
	ID       string
	Kind     remoteproto.RouteKind
	Address  string
	Position int
	Learned  bool
	SSH      *SSHTarget
}

type SSHTarget struct {
	User  string
	Port  int
	Alias string
}

type Patch struct {
	Name    *string
	Enabled *bool
	Weight  *int
}

type RemoveOptions struct {
	Unlink    bool
	Uninstall bool
}

type View struct {
	Computer
	Status      Status
	PlanState   string
	RouteInUse  *Route
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

type Store interface {
	ControllerComputers(context.Context, string) ([]Computer, error)
	AddComputer(context.Context, Computer) error
	UpdateComputer(context.Context, string, remoteproto.ComputerID, Patch) error
	RemoveComputer(context.Context, string, remoteproto.ComputerID) error
	AddComputerRoute(context.Context, string, remoteproto.ComputerID, Route) error
	ForgetComputerRoute(context.Context, string, remoteproto.ComputerID, string) error
	LearnComputerRoutes(context.Context, string, remoteproto.ComputerID, []Route) ([]Route, error)
	RecordComputerConnection(context.Context, string, remoteproto.Descriptor, time.Time) error
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
type ForwardResolver func(context.Context, Route) (string, error)

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
