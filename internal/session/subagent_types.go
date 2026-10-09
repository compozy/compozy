package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// Subagent error sentinels. API/tool layers map them to deterministic codes.
var (
	ErrSubagentParentNotActive   = errors.New("session: subagent parent turn is not active")
	ErrSubagentInvalidRequest    = errors.New("session: invalid subagent request")
	ErrSubagentTargetUnavailable = errors.New("session: subagent target unavailable")
	ErrSubagentCapabilityDenied  = errors.New("session: subagent capability denied")
	ErrSubagentNotFound          = errors.New("session: subagent not found")
	ErrSubagentNotCancelable     = errors.New("session: subagent not cancelable")
	ErrSubagentArchiveFollows    = errors.New("session: subagent archive follows parent")
)

// Subagent modes.
const (
	SubagentModeAsync = "async"
	SubagentModeWait  = "wait"
)

// SubagentCaller identifies the agent tool call that invokes a subagent operation.
type SubagentCaller struct {
	WorkspaceID string
	SessionID   string
	TurnID      string
	ToolCallID  string
	AgentName   string
}

// SubagentActor is the caller of operator-reachable operations (CLI/HTTP/UDS or agent).
type SubagentActor struct {
	Kind   string // "agent" | "operator"
	ID     string
	Caller *SubagentCaller // set when Kind == "agent"
}

// SubagentTarget is the requested (or resolved) runtime of a subagent.
type SubagentTarget struct {
	Agent           string
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           string
	ACPOptions      []acp.SessionConfigOptionSelection
}

// SubagentPermissionNarrowing narrows the caller's permission atoms. A nil list
// inherits the caller's list; a non-nil list must be a subset of it.
type SubagentPermissionNarrowing struct {
	Tools          []string
	Skills         []string
	MCPServers     []string
	WorkspacePaths []string
}

// SubagentRequest is one delegation request.
type SubagentRequest struct {
	Caller         SubagentCaller
	Task           string
	Title          string
	Role           string
	Target         SubagentTarget
	Mode           string                       // async|wait
	Timeout        time.Duration                // wait only
	IdempotencyKey string                       // "" → Caller.ToolCallID
	PermissionMode compozyconfig.PermissionMode // "" = inherit
	Narrowing      *SubagentPermissionNarrowing // nil = inherit
}

// SubagentAgentOption is one delegable agent in the capabilities payload.
type SubagentAgentOption struct {
	Name        string
	Provider    string
	CanDelegate bool
	Constraints []string
}

// SubagentModelOption advertises model-specific runtime choices.
type SubagentModelOption struct {
	ID               string
	Label            string
	ReasoningEfforts []string
	Speeds           []string
}

// SubagentError preserves a public failure code and message across transport boundaries.
type SubagentError struct {
	Code    string
	Message string
	Err     error
}

var _ error = (*SubagentError)(nil) //nolint:errcheck // Compile-time interface assertion.

func (e *SubagentError) Error() string { return e.Message }
func (e *SubagentError) Unwrap() error { return e.Err }

// SubagentProviderOption is one delegable provider in the capabilities payload.
type SubagentProviderOption struct {
	Provider    string
	DisplayName string
	Models      []SubagentModelOption
	CanDelegate bool
	Constraints []string
}

// SubagentCapabilities answers compozy__subagent_capabilities.
type SubagentCapabilities struct {
	ParentSessionID string
	Inherited       SubagentTarget
	PermissionMode  compozyconfig.PermissionMode
	Depth           int
	Live            int
	Agents          []SubagentAgentOption
	Providers       []SubagentProviderOption
}

// Subagent is a store row plus call-scoped presentation fields.
type Subagent struct {
	store.SessionSubagent
	WaitTimedOut  bool   // mode=wait only
	Hint          string // set when ResultTruncated
	ResultPreview string // first line of Result, ≤ 280
}

// SubagentCancelOutcome reports a cancel request.
type SubagentCancelOutcome struct {
	ID     string
	Status string // cancel_requested | <terminal status>
}

// NativeSubagentEvent is a provider-native (Claude Agent/Task) tool event observed
// on a parent session.
type NativeSubagentEvent struct {
	WorkspaceID        string
	ParentTurnID       string
	ProviderToolCallID string
	ToolName           string // Agent | Task
	Title              string
	Model              string
	Status             string // ACP tool-call status: pending|in_progress|completed|failed
	Result             string
	Error              string
	At                 time.Time
}

// SubagentService owns subagent records, delivery, and lifecycle. Transition
// owners are called from the session manager call sites listed in the spec.
type SubagentService interface {
	Capabilities(ctx context.Context, caller SubagentCaller) (SubagentCapabilities, error)
	Delegate(ctx context.Context, req SubagentRequest) (Subagent, error)
	Status(ctx context.Context, caller SubagentCaller, id string) (Subagent, error)
	Cancel(ctx context.Context, actor SubagentActor, id, reason string) (SubagentCancelOutcome, error)
	Get(ctx context.Context, workspaceID, id string) (Subagent, error)
	List(ctx context.Context, q store.SubagentListQuery) (store.SubagentPage, error)
	Summaries(ctx context.Context, parentIDs []string) (map[string]store.SubagentSummary, error)

	OnChildActivity(ctx context.Context, childSessionID string, progress string)
	OnChildSettled(ctx context.Context, childSessionID string) error
	OnWakeDispatched(ctx context.Context, parentID, wakeMessageID string) error
	OnWakeTurnSettled(ctx context.Context, parentID, wakeMessageID string, canceled bool) error
	OnWakeCanceled(ctx context.Context, parentID, wakeMessageID string) error
	OnSteerOutcome(ctx context.Context, parentID, wakeMessageID string, injected bool) error
	OnParentTurnSettled(ctx context.Context, parentID, turnID string) error
	OnParentTurnInterrupted(ctx context.Context, parentID, turnID string) error
	OnParentStopped(ctx context.Context, parentID string) error
	UpgradeWakePolicy(ctx context.Context, id string) error
	OnNativeToolEvent(ctx context.Context, parentID string, ev NativeSubagentEvent) error
	Recover(ctx context.Context) error
}

// SubagentUpdate is published after every committed change to a subagent row.
// The session stream forwards it as `subagent_updated` on the parent's stream.
type SubagentUpdate struct {
	ParentSessionID string
	Subagent        Subagent
}

// SubagentUpdateSubscriber streams committed subagent row changes for one parent session.
// The returned cancel func releases the subscription; the channel closes after cancel.
type SubagentUpdateSubscriber interface {
	SubscribeSubagentUpdates(ctx context.Context, parentSessionID string) (<-chan SubagentUpdate, func(), error)
}
