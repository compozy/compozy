// Package calls owns durable agent-call admission, activation, settlement, and waiting.
package calls

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
)

type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeWorkspace Scope = "workspace"
)

type State string

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

func (s State) Terminal() bool {
	switch s {
	case StateCompleted, StateInvalidResult, StateCompletedWithoutResult,
		StateFailed, StateCanceled, StateTimeout, StateExpired:
		return true
	default:
		return false
	}
}

type Verdict string

const (
	VerdictReturned  Verdict = "returned"
	VerdictExtracted Verdict = "extracted"
	VerdictRepaired  Verdict = "repaired"
)

type Actor struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Target struct {
	Agent     string `json:"agent,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

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

type BatchOutcome struct {
	Call  *CallRecord
	Error error
}

type ReturnInput struct {
	CallID         string
	ChildSessionID string
	Result         json.RawMessage
	FinalText      string
	ChildLive      bool
	Actor          SettlementActor
}

type Settlement struct {
	Call         CallRecord
	RepairPrompt string
	Issues       []contracts.ValidationIssue
}

type SettlementActor struct {
	Kind string
	ID   string
}

type AwaitInput struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
	CallIDs     []string
	Timeout     time.Duration
	Resume      string
}

type AwaitOutcome struct {
	Settled        []CallRecord
	Pending        []string
	Outcome        string
	Resume         string
	ClampedTimeout time.Duration
}

type ChildSpec struct {
	CallID          string
	ParentSessionID string
	AgentName       string
	Prompt          string
	WorkspaceID     string
	IdleTTL         time.Duration
	Runtime         RuntimeSpec
	Permissions     PermissionAtoms
}

type SessionRef struct {
	ID string
}

type Delivery struct {
	CallID             string
	RecipientSessionID string
	Body               string
	Kind               string
}

type DeliveryOutcome struct {
	State  string
	Reason string
}

type TargetContext struct {
	ProfileID       string
	WorkspaceID     string
	ParentSessionID string
	ChildSessionID  string
	AgentName       string
	GovernedRootID  string
	Depth           int
	LiveChildren    int
	State           string
	ExpiredAt       time.Time
	Runtime         RuntimeSpec
	CallerPolicy    store.SessionPermissionPolicy
	Allowed         bool
}

type AgentRosterEntry struct {
	Name        string
	Description string
}

type SweepReport struct {
	TimedOut []string
}

type DrainReport struct {
	RootSessionID string
	Stopped       []string
	CanceledCalls []string
}

type OperatorCallerBinding struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
	SessionID   string
	Created     bool
}
