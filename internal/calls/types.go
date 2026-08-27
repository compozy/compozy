// Package calls owns durable agent-call admission, activation, settlement, and waiting.
package calls

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
)

// Scope identifies the ownership boundary for a call.
type Scope string

// Call ownership scopes.
const (
	ScopeGlobal    Scope = "global"
	ScopeWorkspace Scope = "workspace"
)

const (
	actorKindAgentSession        = "agent_session"
	permissionKindTools          = "tools"
	permissionKindSkills         = "skills"
	permissionKindMCPServers     = "mcp_servers"
	permissionKindWorkspacePaths = "workspace_paths"
	permissionKindNetwork        = "network_channels"
	permissionKindSandbox        = "sandbox_profiles"
)

// State is the durable lifecycle state of a call.
type State string

// Durable call lifecycle states.
const (
	StateQueued                 State = "queued"
	StateRunning                State = "running"
	StateCompleted              State = "completed"
	StateInvalidResult          State = "invalid-result"
	StateCompletedWithoutResult State = "completed-without-result"
	StateFailed                 State = "failed"
	StateCanceled               State = "canceled"
	StateTimeout                State = "timeout"
	StateExpired                State = "expired"
)

// Terminal reports whether no further primary settlement is possible.
func (s State) Terminal() bool {
	switch s {
	case StateCompleted, StateInvalidResult, StateCompletedWithoutResult,
		StateFailed, StateCanceled, StateTimeout, StateExpired:
		return true
	default:
		return false
	}
}

// Verdict records how a valid structured result was obtained.
type Verdict string

// Structured-result verdicts.
const (
	VerdictReturned  Verdict = "returned"
	VerdictExtracted Verdict = "extracted"
	VerdictRepaired  Verdict = "repaired"
)

// Actor identifies the principal performing a call operation.
type Actor struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Target selects either an agent definition or an existing child session.
type Target struct {
	Agent     string `json:"agent,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// CreateInput contains one call admission request.
type CreateInput struct {
	ProfileID      string
	Scope          Scope
	WorkspaceID    string
	Caller         participation.OwnerRef
	Target         Target
	Prompt         string
	Expect         json.RawMessage
	IdleTTL        time.Duration
	Deadline       *time.Time
	Strict         bool
	ResultBudget   *contracts.ByteBudget
	IdempotencyKey string
	Runtime        *RuntimeSpec
	Narrow         PermissionAtoms
	Actor          Actor
	BatchID        string
}

// CallRecord is the durable public record of one admitted call.
type CallRecord struct {
	CallID            string
	ProfileID         string
	Scope             Scope
	WorkspaceID       string
	Caller            participation.OwnerRef
	Actor             Actor
	ActivationRunID   string
	ParentSessionID   string
	AgentName         string
	ChildSessionID    string
	GovernedRootID    string
	Depth             int
	State             State
	Verdict           Verdict
	ExpectDigest      string
	PromptRef         string
	ResultRef         string
	ResultBytes       int
	ResultBudget      contracts.ByteBudget
	Strict            bool
	IdleTTL           time.Duration
	Runtime           RuntimeSpec
	FailureCode       string
	FailureDetail     string
	RepairAttempts    int
	FirstIssueText    string
	SecondIssueText   string
	FinalProsePreview string
	SupersededRef     string
	IdempotencyKey    string
	RequestDigest     string
	BatchID           string
	DeadlineAt        time.Time
	CreatedAt         time.Time
	StartedAt         time.Time
	SettledAt         time.Time
	UpdatedAt         time.Time
	Replayed          bool
}

// BatchOutcome contains either one admitted call or its typed admission error.
type BatchOutcome struct {
	Call  *CallRecord
	Error error
}

// ReturnInput carries a child-owned settlement attempt.
type ReturnInput struct {
	CallID         string
	ChildSessionID string
	Result         json.RawMessage
	FinalText      string
	ChildLive      bool
	Actor          SettlementActor
}

// Settlement contains a settled call or one repair round.
type Settlement struct {
	Call         CallRecord
	RepairPrompt string
	Issues       []contracts.ValidationIssue
}

// SettlementActor identifies the child session claiming settlement authority.
type SettlementActor struct {
	Kind string
	ID   string
}

// AwaitInput selects calls and a bounded wait duration.
type AwaitInput struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
	CallIDs     []string
	Timeout     time.Duration
	Resume      string
}

// AwaitOutcomeKind describes the bounded wait result.
type AwaitOutcomeKind string

// Await result kinds.
const (
	AwaitOutcomeComplete AwaitOutcomeKind = "complete"
	AwaitOutcomePartial  AwaitOutcomeKind = "partial"
	AwaitOutcomeTimeout  AwaitOutcomeKind = "timeout"
)

// AwaitOutcome is a snapshot of settled and pending calls after a bounded wait.
type AwaitOutcome struct {
	Settled        []CallRecord
	Pending        []string
	Outcome        AwaitOutcomeKind
	Resume         string
	ClampedTimeout time.Duration
}

// ChildSpec contains the governed runtime inputs for a new child session.
type ChildSpec struct {
	CallID          string
	ParentSessionID string
	AgentName       string
	Prompt          string
	WorkspaceID     string
	IdleTTL         time.Duration
	Runtime         RuntimeSpec
	Permissions     PermissionAtoms
	RemainingDepth  int
}

// SessionRef identifies a managed child session.
type SessionRef struct {
	ID string
}

// DeliveryKind identifies the durable payload carried to a session boundary.
type DeliveryKind string

// Durable delivery kinds.
const (
	DeliveryKindMessage    DeliveryKind = "message"
	DeliveryKindCompletion DeliveryKind = "completion"
	DeliveryKindRepair     DeliveryKind = "repair"
)

// DeliveryState is the durable delivery-attempt state.
type DeliveryState string

// Durable delivery states.
const (
	DeliveryStatePending  DeliveryState = "pending"
	DeliveryStateInjected DeliveryState = "injected"
	DeliveryStateWoken    DeliveryState = "woken"
	DeliveryStateFailed   DeliveryState = "failed"
)

// Valid reports whether the state belongs to the durable delivery vocabulary.
func (s DeliveryState) Valid() bool {
	switch s {
	case DeliveryStatePending, DeliveryStateInjected, DeliveryStateWoken, DeliveryStateFailed:
		return true
	default:
		return false
	}
}

// Delivery projects one durable payload into a child session.
type Delivery struct {
	CallID             string
	RecipientSessionID string
	Body               string
	Kind               DeliveryKind
	WakeEventID        string
	Metadata           acp.PromptSyntheticMeta
}

// DeliveryOutcome describes what happened at the runtime boundary.
type DeliveryOutcome struct {
	State  DeliveryState
	Reason string
}

// MessageSender identifies the operator or session that authored a message.
type MessageSender struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// SendMessageInput contains one mailbox admission request.
type SendMessageInput struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
	From        MessageSender
	To          string
	CallID      string
	Body        string
}

// MessageAdmission carries a validated message and its transport limits.
type MessageAdmission struct {
	Record      MessageRecord
	Target      string
	DedupWindow time.Duration
	RateLimit   int
	PendingCap  int
}

// MessageDelivery is the public receipt state for a mailbox message.
type MessageDelivery string

// Public mailbox delivery receipts.
const (
	MessageDeliveryQueued            MessageDelivery = "queued"
	MessageDeliveryDeliveredIntoTurn MessageDelivery = "delivered-into-turn"
	MessageDeliveryWoke              MessageDelivery = "woke"
	MessageDeliveryFailed            MessageDelivery = "failed"
)

// MessageRecord is the durable public record of one mailbox message.
type MessageRecord struct {
	MessageID        string          `json:"message_id"`
	ProfileID        string          `json:"profile_id"`
	Scope            Scope           `json:"scope"`
	WorkspaceID      string          `json:"workspace_id,omitempty"`
	From             MessageSender   `json:"from"`
	FromAgentName    string          `json:"from_agent_name,omitempty"`
	ToSessionID      string          `json:"to_session_id"`
	CallID           string          `json:"call_id,omitempty"`
	Body             string          `json:"body"`
	DedupHash        string          `json:"-"`
	Delivery         MessageDelivery `json:"delivery"`
	DeliveryReason   string          `json:"delivery_reason,omitempty"`
	DeliveryAttempts int             `json:"delivery_attempts"`
	CreatedAt        time.Time       `json:"created_at"`
	DeliveredAt      time.Time       `json:"delivered_at,omitzero"`
}

// DeliveryRecord is one durable delivery attempt stream.
type DeliveryRecord struct {
	DeliveryID         string
	Kind               DeliveryKind
	SubjectID          string
	RecipientSessionID string
	OwnerKey           string
	WakeEventID        string
	State              DeliveryState
	Reason             string
	Attempts           int
	CreatedAt          time.Time
}

// DeliveryUpdate advances one durable delivery attempt.
type DeliveryUpdate struct {
	DeliveryID  string
	State       DeliveryState
	Reason      string
	At          time.Time
	MaxAttempts int
}

// TargetState is the authoritative resolved lifecycle state of a call target.
type TargetState string

// Resolved call target states.
const (
	TargetStateActive  TargetState = "active"
	TargetStateParked  TargetState = "parked"
	TargetStateExpired TargetState = "expired"
	TargetStateMissing TargetState = "missing"
)

// TargetContext is the authoritative resolved target used at admission.
type TargetContext struct {
	ProfileID       string
	WorkspaceID     string
	ParentSessionID string
	ChildSessionID  string
	AgentName       string
	GovernedRootID  string
	Depth           int
	LiveChildren    int
	State           TargetState
	ExpiredAt       time.Time
	Runtime         RuntimeSpec
	CallerPolicy    store.SessionPermissionPolicy
	Allowed         bool
}

// AgentRosterEntry is one agent name and its author-provided description.
type AgentRosterEntry struct {
	Name        string
	Description string
}

// SweepReport records calls terminalized by a deadline sweep.
type SweepReport struct {
	TimedOut []string
}

// PublishInput selects one channel-thread conversation for one-way result evidence.
type PublishInput struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
	CallID      string
	Actor       Actor
	Channel     string
	ThreadID    string
}

// ResultEvidence is the Network-neutral payload consumed by the daemon bridge.
type ResultEvidence struct {
	CallID          string
	WorkspaceID     string
	SourceSessionID string
	Channel         string
	ThreadID        string
	MessageID       string
	ResultPreview   json.RawMessage
	ResultBytes     int
	FetchPath       string
}

// PublishReceipt records whether this request created or replayed a publication.
type PublishReceipt struct {
	NetworkMessageID string
	Published        bool
}

// Publication is the durable idempotency row for one call and conversation.
type Publication struct {
	CallID           string
	Channel          string
	ThreadID         string
	NetworkMessageID string
	CreatedAt        time.Time
}

// DrainReport summarizes durable subtree cancellation and runtime stops.
type DrainReport struct {
	RootSessionID    string
	Stopped          []string
	CanceledCalls    []string
	PreservedResults int
}

// OperatorCallerBinding identifies the caller session for an operator call.
type OperatorCallerBinding struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
	SessionID   string
	Created     bool
}
