package gateway

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// IngressSubjectKind identifies a resource that can receive public callbacks.
type IngressSubjectKind string

const (
	IngressSubjectWebhookTrigger IngressSubjectKind = "webhook_trigger"
	IngressSubjectBridgeInstance IngressSubjectKind = "bridge_instance"
)

func (k IngressSubjectKind) Validate() error {
	switch k {
	case IngressSubjectWebhookTrigger, IngressSubjectBridgeInstance:
		return nil
	default:
		return fmt.Errorf("gateway: invalid ingress subject kind %q", k)
	}
}

// IngressScopeKind is the server-resolved ownership scope of an ingress subject.
type IngressScopeKind string

const (
	IngressScopeGlobal    IngressScopeKind = "global"
	IngressScopeWorkspace IngressScopeKind = "workspace"
)

// IngressSubjectRef identifies one bindable webhook trigger or bridge instance.
type IngressSubjectRef struct {
	Kind IngressSubjectKind
	ID   string
}

func (r IngressSubjectRef) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("gateway: ingress subject id is required")
	}
	return nil
}

// IngressSubject is trusted ownership and routing metadata resolved by the server.
type IngressSubject struct {
	IngressSubjectRef
	Scope             IngressScopeKind
	WorkspaceID       string
	Path              string
	TargetUnavailable bool
}

// IngressCaller is a daemon-validated agent identity. A nil caller is the local operator.
type IngressCaller struct {
	SessionID   string
	WorkspaceID string
	AgentName   string
}

// IngressBindRequest confirms one subject against the currently verified endpoint.
type IngressBindRequest struct {
	Subject   IngressSubjectRef
	Confirmed bool
	Caller    *IngressCaller
}

// IngressUnbindRequest removes one binding after the same ownership authorization.
type IngressUnbindRequest struct {
	Subject IngressSubjectRef
	Caller  *IngressCaller
}

// IngressBinding is one durable subject confirmation.
type IngressBinding struct {
	Subject            IngressSubjectRef
	Scope              IngressScopeKind
	WorkspaceID        string
	EndpointGeneration uint64
	ConfirmedAt        time.Time
	Changed            bool
}

// IngressMutationEvent carries one binding change and its validated actor attribution.
type IngressMutationEvent struct {
	Binding   IngressBinding
	ActorKind string
	ActorID   string
}

// UnbindResult makes an idempotent no-op distinct from a state change.
type UnbindResult struct {
	Subject IngressSubjectRef
	Changed bool
}

// IngressEndpointState tracks the verified address independently from desired-state fencing.
type IngressEndpointState struct {
	Tier       Tier
	URL        string
	Generation uint64
	Reachable  bool
	UpdatedAt  time.Time
}

// IngressReachability is the honest public projection for one subject.
type IngressReachability string

const (
	IngressReachabilityOff            IngressReachability = "off"
	IngressReachabilityUnconfirmed    IngressReachability = "unconfirmed"
	IngressReachabilityLive           IngressReachability = "live"
	IngressReachabilityBroken         IngressReachability = "broken"
	IngressReachabilityReconfirmation IngressReachability = "reconfirmation_required"
)

// IngressProjection is safe to expose through trigger, bridge, and status APIs.
type IngressProjection struct {
	Subject                     IngressSubjectRef
	Scope                       IngressScopeKind
	WorkspaceID                 string
	URL                         string
	Reachability                IngressReachability
	EndpointGeneration          uint64
	ConfirmedEndpointGeneration uint64
	ConfirmedAt                 time.Time
	EnablePath                  string
}

// IngressBindingStatus is the operator status projection for a durable binding.
type IngressBindingStatus = IngressProjection
