package store

import (
	"context"
	"errors"
	"time"
)

// Subagent origins.
const (
	SubagentOriginDelegated      = "delegated"
	SubagentOriginProviderNative = "provider_native"
)

// Subagent statuses. Terminal statuses are final.
const (
	SubagentStatusQueued      = "queued"
	SubagentStatusRunning     = "running"
	SubagentStatusWaiting     = "waiting"
	SubagentStatusCompleted   = "completed"
	SubagentStatusFailed      = "failed"
	SubagentStatusCanceled    = "canceled"
	SubagentStatusInterrupted = "interrupted"
)

// Subagent work states.
const (
	SubagentWorkStateWorking            = "working"
	SubagentWorkStateWaitingForChildren = "waiting_for_children"
	SubagentWorkStateResultAvailable    = "result_available"
)

// Subagent delivery states. Delivered, acknowledged, and disposed are final.
const (
	SubagentDeliveryNone         = "none"
	SubagentDeliveryPending      = "pending"
	SubagentDeliveryClaimed      = "claimed"
	SubagentDeliveryDelivered    = "delivered"
	SubagentDeliveryAcknowledged = "acknowledged"
	SubagentDeliveryDisposed     = "disposed"
)

// Subagent wake policies.
const (
	SubagentWakePolicyAlways      = "always"
	SubagentWakePolicySettledOnly = "settled_only"
)

// Subagent wake batch states and routes.
const (
	SubagentWakeStateOpen       = "open"
	SubagentWakeStateDispatched = "dispatched"
	SubagentWakeStateSettled    = "settled"
	SubagentWakeStateCanceled   = "canceled"

	SubagentWakeRouteQueue = "queue"
	SubagentWakeRouteSteer = "steer"
)

// SubagentSpawnRole marks child sessions created by subagent delegation.
const SubagentSpawnRole = "subagent"

var (
	// ErrSubagentNotFound reports a missing or out-of-scope subagent row.
	ErrSubagentNotFound = errors.New("store: subagent not found")
	// ErrSubagentWakeNotFound reports a missing subagent wake batch.
	ErrSubagentWakeNotFound = errors.New("store: subagent wake not found")
	// ErrSubagentIdempotencyConflict reports a reused idempotency key with a different request.
	ErrSubagentIdempotencyConflict = errors.New("store: subagent idempotency key reused with a different request")
)

// SessionSubagent is the durable parent-side record of one subagent.
type SessionSubagent struct {
	ID                 string
	WorkspaceID        string
	ParentSessionID    string
	ParentTurnID       string
	ParentToolCallID   string
	ChildSessionID     *string
	Origin             string
	ProviderToolCallID string
	IdempotencyKey     string
	RequestFingerprint string
	Title              string
	Role               string
	TaskChars          int
	RuntimeAgent       string
	RuntimeProvider    string
	RuntimeModel       string
	RuntimeEffort      string
	RuntimeSpeed       string
	Depth              int
	Status             string
	WorkState          string
	Progress           string
	Result             *string
	Error              *string
	ResultTruncated    bool
	WakePolicy         string
	Delivery           string
	WakeMessageID      *string
	AcknowledgedTurnID string
	StartedAt          *time.Time
	SettledAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SessionSubagentWake is one batched parent wake (mailbox pointer turn or steer).
type SessionSubagentWake struct {
	WakeMessageID   string
	WorkspaceID     string
	ParentSessionID string
	State           string
	Route           string
	InputEntryID    string
	SteerRequeued   bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SubagentSummary is the per-parent roll-up exposed on session payloads.
type SubagentSummary struct {
	Live       int
	Total      int
	Failed     int
	Attention  int
	MostUrgent string
}

// SubagentListQuery pages a parent's (or a workspace's) subagent rows, newest first.
type SubagentListQuery struct {
	WorkspaceID     string
	ParentSessionID string
	Origins         []string
	Statuses        []string
	Cursor          string
	Limit           int
}

// SubagentPage is one page of subagent rows.
type SubagentPage struct {
	Items      []SessionSubagent
	NextCursor string
}

// SubagentFinalize settles a row to a terminal status.
type SubagentFinalize struct {
	ID              string
	Status          string
	WorkState       string
	Result          *string
	Error           *string
	ResultTruncated bool
	SettledAt       time.Time
}

// SubagentDisposeFilter selects non-final deliveries to dispose.
type SubagentDisposeFilter struct {
	ParentSessionID string
	ParentTurnID    string   // "" = all turns of the parent
	IDs             []string // optional explicit set
}

// SubagentStore persists subagent rows and wake batches. Every wake/delivery
// transition runs in one transaction; callers hold the parent's mutex.
type SubagentStore interface {
	ReserveSubagent(ctx context.Context, row SessionSubagent) (SessionSubagent, bool, error)
	LinkChild(ctx context.Context, id, childSessionID string, startedAt time.Time) (SessionSubagent, error)
	GetSubagent(ctx context.Context, workspaceID, id string) (SessionSubagent, error)
	GetSubagentByChild(ctx context.Context, childSessionID string) (SessionSubagent, error)
	ListSubagents(ctx context.Context, q SubagentListQuery) (SubagentPage, error)
	Summaries(ctx context.Context, parentIDs []string) (map[string]SubagentSummary, error)
	UpdateProgress(ctx context.Context, id, progress string, at time.Time) error
	FinalizeSubagent(ctx context.Context, in SubagentFinalize) (SessionSubagent, bool, error)

	OpenOrJoinWake(ctx context.Context, parentID string, ids []string, newWakeID string) (SessionSubagentWake, error)
	GetWake(ctx context.Context, wakeID string) (SessionSubagentWake, []SessionSubagent, error)
	SetWakeInput(ctx context.Context, wakeID, route, inputEntryID string) error
	MarkWakeSteerRequeued(ctx context.Context, wakeID string) error
	MarkWakeDispatched(ctx context.Context, wakeID string) error
	SettleWake(ctx context.Context, wakeID string, canceled bool) ([]SessionSubagent, error)
	SetPending(ctx context.Context, ids []string) error
	Acknowledge(ctx context.Context, id, observedTurnID string) (SessionSubagent, *SessionSubagentWake, error)
	Dispose(ctx context.Context, filter SubagentDisposeFilter) ([]SessionSubagent, error)
	UpgradeWakePolicy(ctx context.Context, id string) (SessionSubagent, error)

	ListStaleReserved(ctx context.Context, olderThan time.Time) ([]SessionSubagent, error)
	ListUnfinalizedDelegated(ctx context.Context) ([]SessionSubagent, error)
	ListOpenWakes(ctx context.Context) ([]SessionSubagentWake, error)
	ListPending(ctx context.Context) ([]SessionSubagent, error)
	ListOrphanSubagentSessions(ctx context.Context) ([]string, error)
}

// IsSubagentStatusTerminal reports whether status is final.
func IsSubagentStatusTerminal(status string) bool {
	switch status {
	case SubagentStatusCompleted, SubagentStatusFailed, SubagentStatusCanceled, SubagentStatusInterrupted:
		return true
	default:
		return false
	}
}

// IsSubagentDeliveryFinal reports whether a delivery state can no longer change.
func IsSubagentDeliveryFinal(delivery string) bool {
	switch delivery {
	case SubagentDeliveryDelivered, SubagentDeliveryAcknowledged, SubagentDeliveryDisposed:
		return true
	default:
		return false
	}
}
