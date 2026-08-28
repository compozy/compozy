package calls

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/task"
)

// AdmissionStore owns atomic call admission.
type AdmissionStore interface {
	AdmitCall(context.Context, Admission) (AdmissionResult, error)
}

// CallLookupStore owns lifecycle lookups by call or child identity.
type CallLookupStore interface {
	GetCall(context.Context, CallScope, string) (CallRecord, error)
	GetCallByChild(context.Context, CallScope, string) (CallRecord, error)
	GetCallForSettlement(context.Context, CallScope, string) (CallRecord, error)
	GetOpenCallForChild(context.Context, CallScope, string) (CallRecord, error)
}

// ActivationMutationStore owns activation terminal state changes.
type ActivationMutationStore interface {
	BindActivationChild(context.Context, ActivationBinding) (CallRecord, error)
	FailActivation(context.Context, ActivationFailure) (CallRecord, error)
}

// SettlementStore owns repair and terminal settlement mutations.
type SettlementStore interface {
	RecordRepair(context.Context, RepairMutation) (CallRecord, error)
	SettleCall(context.Context, SettlementMutation) (CallRecord, error)
}

// DeadlineStore owns deadline recovery reads.
type DeadlineStore interface {
	ListDueCalls(context.Context, time.Time, int) ([]CallRecord, error)
}

// SubtreeStore owns subtree drain fences and projections.
type SubtreeStore interface {
	FenceSessionDrain(context.Context, string, time.Time) error
	UnfenceSessionDrain(context.Context, string, time.Time) error
	ListOpenSubtreeCalls(context.Context, string) ([]CallRecord, error)
	CountPreservedSubtreeResults(context.Context, string) (int, error)
}

// ActivationQueueStore owns durable activation queue recovery.
type ActivationQueueStore interface {
	ListQueuedActivationRunIDs(context.Context, int) ([]string, error)
	LoadActivation(context.Context, string) (CallRecord, ActivationSpec, []byte, PermissionAtoms, error)
	ReconcileActivations(context.Context, time.Time) ([]string, error)
}

// OperatorStore owns the stable operator caller binding.
type OperatorStore interface {
	ResolveOperatorCaller(context.Context, OperatorCallerBinding) (OperatorCallerBinding, error)
	IsOperatorCallerSession(context.Context, string) (bool, error)
}

// TargetContextStore resolves persisted caller and target ownership.
type TargetContextStore interface {
	ResolveCallTargetContext(context.Context, CreateInput) (TargetContext, error)
}

// Store composes the small persistence ports required by call lifecycle services.
type Store interface {
	contracts.RegistryStore
	AdmissionStore
	CallLookupStore
	ActivationMutationStore
	SettlementStore
	DeadlineStore
	SubtreeStore
	ActivationQueueStore
	OperatorStore
	TargetContextStore
}

// MailboxStore owns durable messages, deliveries, and child reap fences.
type MailboxStore interface {
	ChildParkingStore
	AcceptMessage(context.Context, MessageAdmission) (MessageRecord, error)
	GetMessage(context.Context, CallScope, string) (MessageRecord, error)
	ListPendingDeliveries(context.Context, string, int) ([]DeliveryRecord, error)
	RecordDelivery(context.Context, DeliveryUpdate) (DeliveryRecord, error)
	GetCallPayload(context.Context, string, string) ([]byte, error)
	FailPendingDeliveriesForRecipient(context.Context, string, string, time.Time) error
	FenceSessionReap(context.Context, SessionReapFence) (SessionReapFenceResult, error)
	FinalizeReapedSession(context.Context, string, string, time.Time) error
}

// ChildParkingStore owns the settled-child idle clock.
type ChildParkingStore interface {
	ParkCallChild(context.Context, string, time.Time, time.Time) (bool, error)
	ClearCallChildIdleClock(context.Context, string, time.Time) error
}

// PayloadStore reads durable call prompt and result blobs.
type PayloadStore interface {
	GetCallPayload(context.Context, string, string) ([]byte, error)
	GetCallPayloads(context.Context, string, []string) (map[string][]byte, error)
}

// CallListStore owns profile-scoped public call pages.
type CallListStore interface {
	ListCalls(context.Context, CallListQuery) (CallPage, error)
}

// CallReadStore owns profile-scoped public call detail.
type CallReadStore interface {
	GetCallRead(context.Context, CallReadQuery, string) (CallRecord, error)
}

// MessageReadStore owns profile-scoped mailbox pages.
type MessageReadStore interface {
	ListMessages(context.Context, MessageListQuery) (MessagePage, error)
}

// PublicationStore owns the per-conversation publication idempotency fence.
type PublicationStore interface {
	GetPublication(context.Context, string, string, string) (Publication, error)
	RecordPublication(context.Context, Publication) (Publication, bool, error)
}

// Directory resolves one call target and its safe roster projection.
type Directory interface {
	ResolveCallTarget(context.Context, CreateInput) (TargetContext, []AgentRosterEntry, error)
}

// ActivationClaimer claims and releases task-backed call activations.
type ActivationClaimer interface {
	ClaimNextRun(context.Context, task.ClaimCriteria, task.ActorContext) (*task.ClaimResult, error)
	ReleaseRunLease(context.Context, task.LeaseRelease, task.ActorContext) (*task.Run, error)
}

// ActivationRunCanceler cancels one task-backed call activation.
type ActivationRunCanceler interface {
	CancelActivationRun(context.Context, string, string) (CancelOutcome, error)
}

// CancelOutcome reports the durable activation cancellation result.
type CancelOutcome = task.ActivationCancelOutcome

// SessionInvoker controls call-owned child sessions and boundary delivery.
type SessionInvoker interface {
	SpawnChild(context.Context, ChildSpec) (SessionRef, error)
	Revive(context.Context, string, string, string) error
	DeliverAtBoundary(context.Context, Delivery) (DeliveryOutcome, error)
	StopManaged(context.Context, string, string) error
}

// PublishBridge posts bounded call evidence without coupling calls to Network types.
type PublishBridge interface {
	PublishResultEvidence(context.Context, ResultEvidence) (string, error)
}

// CallScope selects one exact profile and global or workspace owner.
type CallScope struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
}

// ReapedSession identifies one call-owned child after durable reaping.
type ReapedSession struct {
	ProfileID       string
	Scope           Scope
	WorkspaceID     string
	SessionID       string
	ParentSessionID string
	RootSessionID   string
	AgentName       string
	Reason          string
}

// SessionReapFence carries the durable inputs for fencing one session reap.
type SessionReapFence struct {
	SessionID string
	Reason    string
	At        time.Time
}

// SessionReapFenceResult reports whether the fence won and calls settled with it.
type SessionReapFenceResult struct {
	Allowed      bool
	SettledCalls []CallRecord
}

// Admission contains the atomic durable state for one accepted call.
type Admission struct {
	Record      *CallRecord
	Contract    *contracts.Contract
	Prompt      []byte
	MaxChildren int
	Permissions []string
	Narrow      PermissionAtoms
	Activation  *ActivationSpec
	FollowUp    *Delivery
}

// AdmissionResult reports the stored call and idempotent replay state.
type AdmissionResult struct {
	Record   CallRecord
	Replayed bool
}

// ActivationKind identifies how an admitted call obtains a child runtime.
type ActivationKind string

// Call activation kinds.
const (
	ActivationKindSpawn  ActivationKind = "spawn"
	ActivationKindRevive ActivationKind = "revive"
)

// ActivationSpec is the durable task-backed child activation request.
type ActivationSpec struct {
	RunID           string
	CallID          string
	WorkspaceID     string
	GovernedRootID  string
	Kind            ActivationKind
	ParentSessionID string
	TargetSessionID string
	AgentName       string
	Depth           int
	IdleTTL         time.Duration
	Runtime         RuntimeSpec
}

// ActivationBinding commits the claimed child identity to a call.
type ActivationBinding struct {
	CallID      string
	RunID       string
	ClaimToken  string
	ChildID     string
	ActivatedAt time.Time
}

// ActivationFailure terminalizes a claimed activation with safe detail.
type ActivationFailure struct {
	CallID     string
	RunID      string
	ClaimToken string
	Code       string
	Detail     string
	FailedAt   time.Time
}

// RepairMutation records the single structured-result repair attempt.
type RepairMutation struct {
	CallID    string
	IssueText string
	At        time.Time
}

// SettlementMutation atomically commits one terminal call outcome.
type SettlementMutation struct {
	CallID             string
	ExpectedState      State
	State              State
	Verdict            Verdict
	Result             []byte
	ResultRef          string
	ResultBytes        int
	FailureCode        string
	FailureDetail      string
	SecondIssueText    string
	FinalProsePreview  string
	Superseded         []byte
	SupersededRef      string
	DeliveryID         string
	WakeEventID        string
	RecipientSessionID string
	SettledAt          time.Time
}
