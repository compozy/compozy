package calls

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/task"
)

type Store interface {
	contracts.RegistryStore
	AdmitCall(context.Context, Admission) (AdmissionResult, error)
	GetCall(context.Context, CallScope, string) (CallRecord, error)
	GetCallByChild(context.Context, CallScope, string) (CallRecord, error)
	GetCallForSettlement(context.Context, string) (CallRecord, error)
	GetOpenCallForChild(context.Context, string) (CallRecord, error)
	BindActivationChild(context.Context, ActivationBinding) (CallRecord, error)
	FailActivation(context.Context, ActivationFailure) (CallRecord, error)
	RecordRepair(context.Context, RepairMutation) (CallRecord, error)
	SettleCall(context.Context, SettlementMutation) (CallRecord, error)
	ListDueCalls(context.Context, time.Time, int) ([]CallRecord, error)
	FenceSessionDrain(context.Context, string, time.Time) error
	ListOpenSubtreeCalls(context.Context, string) ([]CallRecord, error)
	ListQueuedActivationRunIDs(context.Context, int) ([]string, error)
	LoadActivation(context.Context, string) (CallRecord, ActivationSpec, []byte, PermissionAtoms, error)
	ReconcileActivations(context.Context, time.Time) ([]string, error)
	ResolveOperatorCaller(context.Context, OperatorCallerBinding) (OperatorCallerBinding, error)
	IsOperatorCallerSession(context.Context, string) (bool, error)
	ResolveCallTargetContext(context.Context, CreateInput) (TargetContext, error)
}

type MailboxStore interface {
	AcceptMessage(context.Context, MessageAdmission) (MessageRecord, error)
	GetMessage(context.Context, CallScope, string) (MessageRecord, error)
	ListPendingDeliveries(context.Context, string, int) ([]DeliveryRecord, error)
	RecordDelivery(context.Context, DeliveryUpdate) (DeliveryRecord, error)
	ParkCallChild(context.Context, string, time.Time, time.Time) (bool, error)
	ClearCallChildIdleClock(context.Context, string, time.Time) error
	GetCallPayload(context.Context, string, string) ([]byte, error)
	FailPendingDeliveriesForRecipient(context.Context, string, string, time.Time) error
	FenceSessionReap(context.Context, string, time.Time) (bool, error)
	FinalizeReapedSession(context.Context, string, string, time.Time) error
}

type Directory interface {
	ResolveCallTarget(context.Context, CreateInput) (TargetContext, []AgentRosterEntry, error)
}

type ActivationClaimer interface {
	ClaimNextRun(context.Context, task.ClaimCriteria, task.ActorContext) (*task.ClaimResult, error)
	ReleaseRunLease(context.Context, task.LeaseRelease, task.ActorContext) (*task.Run, error)
}

type ActivationRunCanceler interface {
	CancelActivationRun(context.Context, string, string) (CancelOutcome, error)
}

type CancelOutcome = task.ActivationCancelOutcome

type SessionInvoker interface {
	SpawnChild(context.Context, ChildSpec) (SessionRef, error)
	Revive(context.Context, string, string, string) error
	DeliverAtBoundary(context.Context, Delivery) (DeliveryOutcome, error)
	StopManaged(context.Context, string, string) error
}

type CallScope struct {
	ProfileID   string
	Scope       Scope
	WorkspaceID string
}

type Admission struct {
	Record      CallRecord
	Contract    *contracts.Contract
	Prompt      []byte
	MaxChildren int
	Permissions []string
	Narrow      PermissionAtoms
	Activation  *ActivationSpec
	FollowUp    *Delivery
}

type AdmissionResult struct {
	Record   CallRecord
	Replayed bool
}

type ActivationSpec struct {
	RunID           string
	CallID          string
	WorkspaceID     string
	GovernedRootID  string
	Kind            string
	ParentSessionID string
	TargetSessionID string
	AgentName       string
	Depth           int
	IdleTTL         time.Duration
	Runtime         RuntimeSpec
}

type ActivationBinding struct {
	CallID      string
	RunID       string
	ClaimToken  string
	ChildID     string
	ActivatedAt time.Time
}

type ActivationFailure struct {
	CallID     string
	RunID      string
	ClaimToken string
	Code       string
	Detail     string
	FailedAt   time.Time
}

type RepairMutation struct {
	CallID    string
	IssueText string
	At        time.Time
}

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
