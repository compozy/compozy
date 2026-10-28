package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

// SubagentChildActivity describes a delegated child session's liveness,
// independent of the row's (possibly terminal) status. Unknown covers
// provider-native rows and rows written before the field existed. It is a byte
// (persisted as its String form) so SessionSubagent stays within the
// pass-by-value size budget.
type SubagentChildActivity uint8

const (
	SubagentChildActivityUnknown SubagentChildActivity = iota
	SubagentChildActivityWorking
	SubagentChildActivityIdle
	SubagentChildActivityStopped
)

var subagentChildActivityNames = [...]string{"", "working", "idle", "stopped"}

// String returns the persisted and wire form; unknown is empty.
func (a SubagentChildActivity) String() string {
	if int(a) < len(subagentChildActivityNames) {
		return subagentChildActivityNames[a]
	}
	return ""
}

// ParseSubagentChildActivity reads the persisted form; empty is unknown.
func ParseSubagentChildActivity(value string) (SubagentChildActivity, error) {
	for i, name := range subagentChildActivityNames {
		if name == value {
			return SubagentChildActivity(i), nil
		}
	}
	return SubagentChildActivityUnknown, fmt.Errorf("store: invalid subagent child activity %q", value)
}

// SubagentChildActivityValues returns the known (non-empty) child activities.
func SubagentChildActivityValues() []string {
	return slices.Clone(subagentChildActivityNames[1:])
}

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
	Isolation string
	Worktree  *SubagentWorktreeState

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
	// PendingTask holds the child's first-prompt text from reserve until the
	// first prompt is admitted (MarkFirstPromptAdmitted clears it), so boot
	// recovery can re-admit it. NULL afterwards; the task then lives in the
	// child transcript.
	PendingTask        *string
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
	ChildActivity      SubagentChildActivity
	WakePolicy         string
	Delivery           string
	WakeMessageID      *string
	AcknowledgedTurnID string
	StartedAt          *time.Time
	SettledAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SubagentWorktreeState is present only after an isolated checkout is associated.
type SubagentWorktreeState struct {
	ID, Name, Branch, BaseRef, Path, BaseSHA, Cleanup string
	Facts                                             SubagentWorktreeFacts
}

// WorktreeState returns the optional checkout state as a value, safe for shared rows.
func (s SessionSubagent) WorktreeState() SubagentWorktreeState {
	if s.Worktree == nil {
		return SubagentWorktreeState{}
	}
	return *s.Worktree
}

type SubagentWorktreeFacts struct {
	HeadSHA                  string
	CommitsAhead, DirtyFiles *int
	ObservedAt               time.Time
	PRStatus, PRURL          string
	PRNumber                 *int
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
	Attempts        int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SubagentSummary is the per-parent roll-up exposed on session payloads.
// Live counts nonterminal rows, which gate parent settlement. Resumed counts
// settled delegations whose child session is working again: they display as
// running, so Failed leaves them out and MostUrgent treats them as live.
type SubagentSummary struct {
	Live       int
	Resumed    int
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
	// Order is SubagentOrderNewest (default) or SubagentOrderUrgency; urgency
	// applies to one parent's rows.
	Order  string
	Cursor string
	Limit  int
}

// Subagent list orders.
const (
	SubagentOrderNewest  = "newest"
	SubagentOrderUrgency = "urgency"
)

// SubagentUrgencyRank orders rows most urgent first, as clients display them:
// waiting, failed, working (a resumed child included), queued, then settled.
// It mirrors the ranking in ListSubagentsByParentUrgency.
func SubagentUrgencyRank(row SessionSubagent) int {
	if IsSubagentStatusTerminal(row.Status) && row.ChildActivity == SubagentChildActivityWorking {
		return 2
	}
	switch row.Status {
	case SubagentStatusWaiting:
		return 0
	case SubagentStatusFailed:
		return 1
	case SubagentStatusRunning:
		return 2
	case SubagentStatusQueued:
		return 3
	default:
		return 4
	}
}

// SubagentPage is one page of subagent rows.
type SubagentPage struct {
	Items      []SessionSubagent
	NextCursor string
}

// NativeMissingResultCursor is the (settled_at, id) position of the last
// missing-result row returned; the zero value starts from the newest row.
type NativeMissingResultCursor struct {
	SettledAt time.Time
	ID        string
}

// SubagentFinalize settles a row to a terminal status.
type SubagentFinalize struct {
	WorktreeFacts   *SubagentWorktreeFacts
	ID              string
	Status          string
	WorkState       string
	ChildActivity   SubagentChildActivity
	Result          *string
	Error           *string
	ResultTruncated bool
	SettledAt       time.Time
}

// NativeSubagentResultEnrichment fills the missing result of a settled
// provider-native row from a later provider summary. Every guard must match:
// the row's identity, origin, expected terminal status, and a NULL result.
type NativeSubagentResultEnrichment struct {
	ID                 string
	WorkspaceID        string
	ParentSessionID    string
	ParentTurnID       string
	ProviderToolCallID string
	Status             string
	Result             string
	Error              *string
	ResultTruncated    bool
	At                 time.Time
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
	HasSubagentCommittedAdmission(ctx context.Context, workspaceID, childID, subagentID string) (bool, error)
	AssociateSubagentWorktree(ctx context.Context, row SessionSubagent) error
	SetSubagentWorktreeCleanup(ctx context.Context, id, state string) error
	ListSubagentWorktreeCleanupPending(ctx context.Context) ([]SessionSubagent, error)
	ReserveSubagent(ctx context.Context, row SessionSubagent) (SessionSubagent, bool, error)
	LinkChild(ctx context.Context, id, childSessionID string, startedAt time.Time) (SessionSubagent, error)
	// MarkFirstPromptAdmitted clears PendingTask once the child's first prompt is admitted.
	MarkFirstPromptAdmitted(ctx context.Context, id string) error
	// DeleteReserved removes a row still `queued` with no child (spawn.pre_create denial:
	// no session and no row, IT-022). It returns ErrSubagentNotFound for any other row.
	DeleteReserved(ctx context.Context, id string) error
	GetSubagent(ctx context.Context, workspaceID, id string) (SessionSubagent, error)
	GetSubagentByID(ctx context.Context, id string) (SessionSubagent, error)
	GetSubagentByChild(ctx context.Context, childSessionID string) (SessionSubagent, error)
	ListSubagents(ctx context.Context, q SubagentListQuery) (SubagentPage, error)
	Summaries(ctx context.Context, parentIDs []string) (map[string]SubagentSummary, error)
	UpdateProgress(ctx context.Context, id, progress string, at time.Time) error
	UpdateNativeSubagentTitle(ctx context.Context, id, title string, at time.Time) (SessionSubagent, bool, error)
	UpdateSubagentState(ctx context.Context, id, status, workState string, at time.Time) (SessionSubagent, bool, error)
	FinalizeSubagent(ctx context.Context, in SubagentFinalize) (SessionSubagent, bool, error)
	// UpdateSubagentChildActivity records a delegated child's liveness; it never changes status.
	UpdateSubagentChildActivity(
		ctx context.Context, id string, activity SubagentChildActivity, at time.Time,
	) (SessionSubagent, bool, error)
	// ReconcileStoppedChildActivity marks settled delegated rows whose child session is stopped.
	ReconcileStoppedChildActivity(ctx context.Context) ([]SessionSubagent, error)

	OpenOrJoinWake(ctx context.Context, parentID string, ids []string, newWakeID string) (SessionSubagentWake, error)
	GetWake(ctx context.Context, wakeID string) (SessionSubagentWake, []SessionSubagent, error)
	ListWakesByParent(ctx context.Context, parentID string, states []string) ([]SessionSubagentWake, error)
	SetWakeInput(ctx context.Context, wakeID, route, inputEntryID string) error
	RewriteSubagentWakeInput(ctx context.Context, wakeID, text string, metadata json.RawMessage) error
	MarkWakeSteerRequeued(ctx context.Context, wakeID string) error
	MarkWakeDispatched(ctx context.Context, wakeID string) error
	FailWake(ctx context.Context, wakeID string) (SessionSubagentWake, []SessionSubagent, error)
	SettleWake(ctx context.Context, wakeID string, canceled bool) ([]SessionSubagent, error)
	SetPending(ctx context.Context, ids []string) error
	Acknowledge(ctx context.Context, id, observedTurnID string) (SessionSubagent, *SessionSubagentWake, error)
	Dispose(ctx context.Context, filter SubagentDisposeFilter) ([]SessionSubagent, error)
	UpgradeWakePolicy(ctx context.Context, id string) (SessionSubagent, error)

	ListStaleReserved(ctx context.Context, olderThan time.Time) ([]SessionSubagent, error)
	ListUnfinalizedDelegated(ctx context.Context) ([]SessionSubagent, error)
	ListUnfinalizedNative(ctx context.Context) ([]SessionSubagent, error)
	// EnrichNativeSubagentResult fills a terminal native row's missing result. It
	// never changes status, settlement, delivery, or child activity.
	EnrichNativeSubagentResult(ctx context.Context, in NativeSubagentResultEnrichment) (SessionSubagent, bool, error)
	// ListNativeMissingResult returns at most limit terminal native rows settled
	// since the cutoff whose result is still missing, newest first, strictly
	// after the cursor (zero for the first page).
	ListNativeMissingResult(
		ctx context.Context, since time.Time, after NativeMissingResultCursor, limit int,
	) ([]SessionSubagent, error)
	ListOpenWakes(ctx context.Context) ([]SessionSubagentWake, error)
	ListPending(ctx context.Context, parentID string) ([]SessionSubagent, error)
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
