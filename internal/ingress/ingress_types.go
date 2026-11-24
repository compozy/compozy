package ingress

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// SubjectKind identifies a resource that can receive public callbacks.
type SubjectKind string

const (
	SubjectWebhookTrigger SubjectKind = "webhook_trigger"
)

func (k SubjectKind) Validate() error {
	switch k {
	case SubjectWebhookTrigger:
		return nil
	default:
		return fmt.Errorf("gateway: invalid ingress subject kind %q", k)
	}
}

// ScopeKind is the server-resolved ownership scope of an ingress subject.
type ScopeKind string

const (
	ScopeGlobal    ScopeKind = "global"
	ScopeWorkspace ScopeKind = "workspace"
)

// SubjectRef identifies one bindable webhook trigger.
type SubjectRef struct {
	Kind SubjectKind
	ID   string
}

func (r SubjectRef) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("gateway: ingress subject id is required")
	}
	return nil
}

// Subject is trusted ownership and routing metadata resolved by the server.
type Subject struct {
	ProfileID string
	SubjectRef
	Scope             ScopeKind
	WorkspaceID       string
	Path              string
	TargetUnavailable bool
}

// Caller is a daemon-validated agent identity. A nil caller is the local operator.
type Caller struct {
	SessionID   string
	WorkspaceID string
	AgentName   string
}

// BindRequest confirms one subject against the currently verified endpoint.
type BindRequest struct {
	Subject   SubjectRef
	Confirmed bool
	Caller    *Caller
}

// UnbindRequest removes one binding after the same ownership authorization.
type UnbindRequest struct {
	Subject SubjectRef
	Caller  *Caller
}

// Binding is one durable subject confirmation.
type Binding struct {
	Subject            SubjectRef
	Scope              ScopeKind
	WorkspaceID        string
	EndpointGeneration uint64
	ConfirmedAt        time.Time
	Changed            bool
}

// MutationEvent carries one binding change and its validated actor attribution.
type MutationEvent struct {
	Binding   Binding
	ActorKind string
	ActorID   string
}

// UnbindResult makes an idempotent no-op distinct from a state change.
type UnbindResult struct {
	Subject SubjectRef
	Changed bool
}

// EndpointState tracks the verified address independently from desired-state fencing.
type EndpointState struct {
	URL        string
	Generation uint64
	Reachable  bool
	Remote     bool
	UpdatedAt  time.Time
}

// Reachability is the honest public projection for one subject.
type Reachability string

const (
	ReachabilityOff            Reachability = "off"
	ReachabilityUnconfirmed    Reachability = "unconfirmed"
	ReachabilityLive           Reachability = "live"
	ReachabilityBroken         Reachability = "broken"
	ReachabilityReconfirmation Reachability = "reconfirmation_required"
)

// Projection is safe to expose through trigger and status APIs.
type Projection struct {
	Subject                     SubjectRef
	Scope                       ScopeKind
	WorkspaceID                 string
	URL                         string
	Reachability                Reachability
	EndpointGeneration          uint64
	ConfirmedEndpointGeneration uint64
	ConfirmedAt                 time.Time
	EnablePath                  string
}
