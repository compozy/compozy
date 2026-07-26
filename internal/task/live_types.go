package task

import (
	"context"
	"encoding/json"
	"time"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/store"
)

// LiveService exposes task-native live and run-detail reads for downstream API handlers.
type LiveService interface {
	Timeline(ctx context.Context, taskID string, query TimelineQuery, actor ActorContext) ([]TimelineItem, error)
	Stream(ctx context.Context, taskID string, query StreamQuery, actor ActorContext) (<-chan StreamEvent, error)
	Tree(ctx context.Context, taskID string, actor ActorContext) (*TreeView, error)
	RunDetail(ctx context.Context, runID string, actor ActorContext) (*RunDetailView, error)
}

// TimelineQuery captures reconnect-friendly task timeline windowing semantics.
type TimelineQuery struct {
	AfterSequence int64 `json:"after_sequence,omitempty"`
	Limit         int   `json:"limit,omitempty"`
}

// StreamQuery captures reconnect-friendly task stream replay semantics.
type StreamQuery struct {
	AfterSequence int64 `json:"after_sequence,omitempty"`
}

// EventRecordQuery captures low-level task event record reads that include a stable sequence.
type EventRecordQuery struct {
	TaskID        string `json:"task_id,omitempty"`
	AfterSequence int64  `json:"after_sequence,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Descending    bool   `json:"descending,omitempty"`
	AllTasks      bool   `json:"-"`
}

// EventRecord is one immutable task event plus its stable stream sequence.
type EventRecord struct {
	Sequence int64 `json:"sequence"`
	Event    Event `json:"event"`
}

// TimelineItem is the normalized task event row consumed by live task surfaces.
type TimelineItem struct {
	Sequence  int64           `json:"sequence"`
	EventID   string          `json:"event_id"`
	Task      Reference       `json:"task"`
	Run       *RunSummary     `json:"run,omitempty"`
	EventType string          `json:"event_type"`
	Actor     ActorIdentity   `json:"actor"`
	Origin    Origin          `json:"origin"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// StreamEvent is one task-scoped replayable live event suitable for SSE transport.
type StreamEvent struct {
	Sequence int64        `json:"sequence"`
	Type     string       `json:"type"`
	Timeline TimelineItem `json:"timeline"`
}

// EventObserver receives immutable task events after durable persistence.
type EventObserver interface {
	OnTaskEvent(ctx context.Context, record EventRecord)
}

// TreeView is the manager-owned live snapshot for one task tree.
type TreeView struct {
	Root        TreeNode   `json:"root"`
	Descendants []TreeNode `json:"descendants,omitempty"`
}

// TreeNode is one node inside a task-tree live snapshot.
type TreeNode struct {
	Task           Reference   `json:"task"`
	ParentTaskID   string      `json:"parent_task_id,omitempty"`
	Depth          int         `json:"depth"`
	ChildCount     int         `json:"child_count,omitempty"`
	ActiveRun      *RunSummary `json:"active_run,omitempty"`
	LastActivityAt time.Time   `json:"last_activity_at"`
}

// RunDetailView is one run detail payload with an optional task anchor.
type RunDetailView struct {
	Run     Run                   `json:"run"`
	Task    *Reference            `json:"task,omitempty"`
	Session *RunSessionRef        `json:"session,omitempty"`
	Summary RunOperationalSummary `json:"summary"`
}

// RunSessionRef links one run to its backing session when available.
type RunSessionRef struct {
	SessionID   string    `json:"session_id"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	AgentName   string    `json:"agent_name,omitempty"`
	Name        string    `json:"name,omitempty"`
	State       string    `json:"state,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RunOperationalSummary captures run-detail metrics aggregated from runtime data when available.
type RunOperationalSummary struct {
	LastActivityAt time.Time `json:"last_activity_at"`
	LastEventType  string    `json:"last_event_type,omitempty"`
	ToolCallCount  *int64    `json:"tool_call_count,omitempty"`
	TurnCount      *int64    `json:"turn_count,omitempty"`
	InputTokens    *int64    `json:"input_tokens,omitempty"`
	OutputTokens   *int64    `json:"output_tokens,omitempty"`
	TotalTokens    *int64    `json:"total_tokens,omitempty"`
	TotalCost      *float64  `json:"total_cost,omitempty"`
	CostCurrency   *string   `json:"cost_currency,omitempty"`
	CostStatus     string    `json:"cost_status,omitempty"`
	CostSource     string    `json:"cost_source,omitempty"`
}

// InspectTarget identifies whether an inspect response was requested by task or run id.
type InspectTarget string

const (
	// InspectTargetTask reports an inspect request rooted at a task id.
	InspectTargetTask InspectTarget = "task"
	// InspectTargetRun reports an inspect request rooted at a run id.
	InspectTargetRun InspectTarget = "run"
)

// InspectNextAction is the deterministic next-step hint emitted by task inspect.
type InspectNextAction string

const (
	InspectNextActionClaimAvailable    InspectNextAction = "claim_available"
	InspectNextActionWaitingForSession InspectNextAction = "waiting_for_session"
	InspectNextActionStranded          InspectNextAction = "stranded"
	InspectNextActionRunning           InspectNextAction = "running"
	InspectNextActionRecoveryRequired  InspectNextAction = "recovery_required"
	InspectNextActionTerminal          InspectNextAction = "terminal"
)

// InspectSchedulerState captures the read-only scheduler pause state used by diagnostics.
type InspectSchedulerState struct {
	Paused    bool      `json:"paused"`
	PausedBy  string    `json:"paused_by,omitempty"`
	PausedAt  time.Time `json:"paused_at"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InspectRunSummary is the redacted run projection used by task inspect.
type InspectRunSummary struct {
	RunID                   string    `json:"run_id"`
	TaskID                  string    `json:"task_id"`
	Status                  RunStatus `json:"status"`
	ClaimTokenHashTruncated string    `json:"claim_token_hash_truncated,omitempty"`
	LeaseUntil              time.Time `json:"lease_until"`
	HeartbeatAt             time.Time `json:"heartbeat_at"`
	HeartbeatAgeSeconds     *int64    `json:"heartbeat_age_seconds,omitempty"`
	Retries                 int       `json:"retries,omitempty"`
	LastErrorSummary        string    `json:"last_error_summary,omitempty"`
	FailureKind             string    `json:"failure_kind,omitempty"`
	BoundSessionID          string    `json:"bound_session_id,omitempty"`
	StartedAt               time.Time `json:"started_at"`
	EndedAt                 time.Time `json:"ended_at"`
	PreviousRunID           string    `json:"previous_run_id,omitempty"`
	QueuedAt                time.Time `json:"queued_at"`
	Attempt                 int       `json:"attempt"`
	RecoveryCount           int       `json:"recovery_count"`
}

// InspectSessionSummary is the session projection used by task inspect.
type InspectSessionSummary struct {
	SessionID      string    `json:"session_id"`
	State          string    `json:"state,omitempty"`
	AgentName      string    `json:"agent_name,omitempty"`
	ProviderName   string    `json:"provider_name,omitempty"`
	WorkspaceID    string    `json:"workspace_id,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
	StopReason     string    `json:"stop_reason,omitempty"`
	FailureKind    string    `json:"failure_kind,omitempty"`
}

// InspectEventSummary is the recent event summary projection used by task inspect.
type InspectEventSummary struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	RunID     string    `json:"run_id,omitempty"`
	Outcome   string    `json:"outcome,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// InspectView is the task-domain read-only snapshot behind CLI/API inspect.
type InspectView struct {
	Target                InspectTarget                       `json:"target"`
	Task                  Summary                             `json:"task"`
	CurrentRun            *InspectRunSummary                  `json:"current_run,omitempty"`
	BoundSession          *InspectSessionSummary              `json:"bound_session,omitempty"`
	RecentRuns            []InspectRunSummary                 `json:"recent_runs,omitempty"`
	RecentEvents          []InspectEventSummary               `json:"recent_events,omitempty"`
	Scheduler             InspectSchedulerState               `json:"scheduler"`
	Diagnostics           []diagnosticcontract.DiagnosticItem `json:"diagnostics,omitempty"`
	NextAction            InspectNextAction                   `json:"next_action"`
	AsOf                  time.Time                           `json:"as_of"`
	EligibleSessionCount  int                                 `json:"-"`
	SessionCatalogPresent bool                                `json:"-"`
}

// RuntimeViewReader enriches run-detail reads with session and usage telemetry when available.
type RuntimeViewReader interface {
	GetSession(ctx context.Context, sessionID string) (*RunSessionRef, error)
	ListSessionEvents(ctx context.Context, sessionID string, query store.EventQuery) ([]store.SessionEvent, error)
	ListSessionTokenStats(ctx context.Context, sessionID string) ([]store.TokenStats, error)
}

// InspectStateReader supplies read-only runtime state for task inspect diagnostics.
type InspectStateReader interface {
	ListSessions(ctx context.Context, query store.SessionListQuery) ([]store.SessionInfo, error)
	ListEventSummaries(ctx context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error)
	GetSchedulerPauseState(ctx context.Context) (InspectSchedulerState, error)
}
