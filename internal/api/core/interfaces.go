package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/pkg/compozy/events"
)

const defaultHeartbeatInterval = contract.DefaultHeartbeatInterval

// HandlerConfig wires the shared daemon transport handlers.
type HandlerConfig struct {
	TransportName     string
	Logger            *slog.Logger
	Now               func() time.Time
	HeartbeatInterval time.Duration
	StreamDone        <-chan struct{}

	Daemon     DaemonService
	Workspaces WorkspaceService
	Tasks      TaskService
	Reviews    ReviewService
	Runs       RunService
	Sync       SyncService
	Exec       ExecService
}

// DaemonService exposes daemon-wide status, health, metrics, and shutdown control.
type DaemonService interface {
	Status(context.Context) (DaemonStatus, error)
	Health(context.Context) (DaemonHealth, error)
	Metrics(context.Context) (MetricsPayload, error)
	Stop(context.Context, bool) error
}

// WorkspaceService exposes workspace registration and lookup.
type WorkspaceService interface {
	Register(context.Context, string, string) (WorkspaceRegisterResult, error)
	List(context.Context) ([]Workspace, error)
	Get(context.Context, string) (Workspace, error)
	Update(context.Context, string, WorkspaceUpdateInput) (Workspace, error)
	Delete(context.Context, string) error
	Resolve(context.Context, string) (Workspace, error)
	Sync(context.Context) (WorkspaceSyncResult, error)
}

// TaskService exposes workflow summary, rich read-model, validation, and run-start surfaces.
type TaskService interface {
	Dashboard(context.Context, string) (DashboardPayload, error)
	ListWorkflows(context.Context, string) ([]WorkflowSummary, error)
	GetWorkflow(context.Context, string, string) (WorkflowSummary, error)
	WorkflowOverview(context.Context, string, string) (WorkflowOverviewPayload, error)
	ListItems(context.Context, string, string) ([]TaskItem, error)
	TaskBoard(context.Context, string, string) (TaskBoardPayload, error)
	WorkflowSpec(context.Context, string, string) (WorkflowSpecDocument, error)
	WorkflowMemoryIndex(context.Context, string, string) (WorkflowMemoryIndex, error)
	WorkflowMemoryFile(context.Context, string, string, string) (MarkdownDocument, error)
	TaskDetail(context.Context, string, string, string) (TaskDetailPayload, error)
	Validate(context.Context, string, string) (ValidationSuccess, error)
	StartRun(context.Context, string, string, TaskRunRequest) (Run, error)
	Archive(context.Context, string, string) (ArchiveResult, error)
}

// ReviewService exposes review round state, review detail reads, and review-fix run starts.
type ReviewService interface {
	Fetch(context.Context, string, string, ReviewFetchRequest) (ReviewFetchResult, error)
	GetLatest(context.Context, string, string) (ReviewSummary, error)
	GetRound(context.Context, string, string, int) (ReviewRound, error)
	ListIssues(context.Context, string, string, int) ([]ReviewIssue, error)
	ReviewDetail(context.Context, string, string, int, string) (ReviewDetailPayload, error)
	StartRun(context.Context, string, string, int, ReviewRunRequest) (Run, error)
}

// RunService exposes run snapshots, rich run detail, pagination, streaming, and cancellation.
type RunService interface {
	List(context.Context, RunListQuery) ([]Run, error)
	Get(context.Context, string) (Run, error)
	Snapshot(context.Context, string) (RunSnapshot, error)
	RunDetail(context.Context, string) (RunDetailPayload, error)
	Events(context.Context, string, RunEventPageQuery) (RunEventPage, error)
	OpenStream(context.Context, string, StreamCursor) (RunStream, error)
	Cancel(context.Context, string) error
}

// SyncService exposes explicit workflow reconciliation.
type SyncService interface {
	Sync(context.Context, SyncRequest) (SyncResult, error)
}

// ExecService exposes ad-hoc daemon-backed exec starts.
type ExecService interface {
	Start(context.Context, ExecRequest) (Run, error)
}

// RunStream is the live run event subscription surfaced to the transport layer.
type RunStream interface {
	Events() <-chan RunStreamItem
	Errors() <-chan error
	Close() error
}

// RunStreamItem carries one live stream delivery or an overflow notice.
type RunStreamItem struct {
	Event    *events.Event
	Overflow *RunStreamOverflow
}

// RunStreamOverflow notifies the transport that the client must reconnect from the last cursor.
type RunStreamOverflow struct {
	Reason string
}

// MetricsPayload carries pre-rendered metrics text.
type MetricsPayload struct {
	Body        string
	ContentType string
}

type DaemonStatus = contract.DaemonStatus
type DaemonHealth = contract.DaemonHealth
type HealthDetail = contract.HealthDetail
type DaemonModeCount = contract.DaemonModeCount
type DaemonDatabaseDiagnostics = contract.DaemonDatabaseDiagnostics
type DaemonReconcileDiagnostics = contract.DaemonReconcileDiagnostics
type Workspace = contract.Workspace
type WorkspaceRegisterResult = contract.WorkspaceRegisterResult
type WorkspaceUpdateInput = contract.WorkspaceUpdateInput
type WorkspaceSyncResult = contract.WorkspaceSyncResult
type WorkflowSummary = contract.WorkflowSummary
type TaskItem = contract.TaskItem
type ValidationSuccess = contract.ValidationSuccess
type ArchiveResult = contract.ArchiveResult

// DashboardPayload is the workspace-scoped dashboard aggregate for the daemon web UI.
type DashboardPayload struct {
	Workspace      Workspace             `json:"workspace"`
	Daemon         DaemonStatus          `json:"daemon"`
	Health         DaemonHealth          `json:"health"`
	Queue          DashboardQueueSummary `json:"queue"`
	Workflows      []WorkflowCard        `json:"workflows,omitempty"`
	ActiveRuns     []Run                 `json:"active_runs,omitempty"`
	PendingReviews int                   `json:"pending_reviews"`
}

// DashboardQueueSummary captures the current run queue health for one workspace.
type DashboardQueueSummary struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Canceled  int `json:"canceled"`
}

// WorkflowCard is the dashboard-friendly workflow summary card.
type WorkflowCard struct {
	Workflow         WorkflowSummary `json:"workflow"`
	TaskTotal        int             `json:"task_total"`
	TaskCompleted    int             `json:"task_completed"`
	TaskPending      int             `json:"task_pending"`
	LatestReview     *ReviewSummary  `json:"latest_review,omitempty"`
	ReviewRoundCount int             `json:"review_round_count"`
	ActiveRuns       int             `json:"active_runs"`
}

// WorkflowOverviewPayload is the richer workflow summary aggregate used by browser reads.
type WorkflowOverviewPayload struct {
	Workspace       Workspace          `json:"workspace"`
	Workflow        WorkflowSummary    `json:"workflow"`
	TaskCounts      WorkflowTaskCounts `json:"task_counts"`
	LatestReview    *ReviewSummary     `json:"latest_review,omitempty"`
	RecentRuns      []Run              `json:"recent_runs,omitempty"`
	ArchiveEligible bool               `json:"archive_eligible"`
	ArchiveReason   string             `json:"archive_reason,omitempty"`
}

// WorkflowTaskCounts summarizes task progress for one workflow.
type WorkflowTaskCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
}

type ReviewFetchRequest = contract.ReviewFetchRequest
type ReviewFetchResult = contract.ReviewFetchResult
type ReviewSummary = contract.ReviewSummary
type ReviewRound = contract.ReviewRound
type ReviewIssue = contract.ReviewIssue

// TaskBoardPayload captures the workflow task-board read model.
type TaskBoardPayload struct {
	Workspace  Workspace          `json:"workspace"`
	Workflow   WorkflowSummary    `json:"workflow"`
	TaskCounts WorkflowTaskCounts `json:"task_counts"`
	Lanes      []TaskLane         `json:"lanes,omitempty"`
}

// TaskLane groups task cards under one normalized status lane.
type TaskLane struct {
	Status string     `json:"status"`
	Title  string     `json:"title"`
	Items  []TaskCard `json:"items,omitempty"`
}

// TaskCard is the compact task row used by board and detail reads.
type TaskCard struct {
	TaskNumber int       `json:"task_number"`
	TaskID     string    `json:"task_id"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Type       string    `json:"type"`
	DependsOn  []string  `json:"depends_on,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MarkdownDocument is the normalized daemon-served markdown payload.
type MarkdownDocument struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	UpdatedAt time.Time       `json:"updated_at"`
	Markdown  string          `json:"markdown"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// WorkflowSpecDocument captures the canonical workflow spec artifacts.
type WorkflowSpecDocument struct {
	Workspace Workspace          `json:"workspace"`
	Workflow  WorkflowSummary    `json:"workflow"`
	PRD       *MarkdownDocument  `json:"prd,omitempty"`
	TechSpec  *MarkdownDocument  `json:"techspec,omitempty"`
	ADRs      []MarkdownDocument `json:"adrs,omitempty"`
}

// WorkflowMemoryIndex lists workflow memory files using opaque daemon-issued identifiers.
type WorkflowMemoryIndex struct {
	Workspace Workspace             `json:"workspace"`
	Workflow  WorkflowSummary       `json:"workflow"`
	Entries   []WorkflowMemoryEntry `json:"entries,omitempty"`
}

// WorkflowMemoryEntry describes one memory file without exposing raw filesystem paths.
type WorkflowMemoryEntry struct {
	FileID      string    `json:"file_id"`
	DisplayPath string    `json:"display_path"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	SizeBytes   int64     `json:"size_bytes"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskDetailPayload captures the richer workflow task detail read model.
type TaskDetailPayload struct {
	Workspace         Workspace             `json:"workspace"`
	Workflow          WorkflowSummary       `json:"workflow"`
	Task              TaskCard              `json:"task"`
	Document          MarkdownDocument      `json:"document"`
	MemoryEntries     []WorkflowMemoryEntry `json:"memory_entries,omitempty"`
	RelatedRuns       []Run                 `json:"related_runs,omitempty"`
	LiveTailAvailable bool                  `json:"live_tail_available"`
}

// ReviewIssueDetail captures the detail metadata for one review issue.
type ReviewIssueDetail struct {
	ID          string    `json:"id"`
	IssueNumber int       `json:"issue_number"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ReviewDetailPayload captures the richer review issue detail read model.
type ReviewDetailPayload struct {
	Workspace   Workspace         `json:"workspace"`
	Workflow    WorkflowSummary   `json:"workflow"`
	Round       ReviewRound       `json:"round"`
	Issue       ReviewIssueDetail `json:"issue"`
	Document    MarkdownDocument  `json:"document"`
	RelatedRuns []Run             `json:"related_runs,omitempty"`
}

type SessionViewSnapshot = contract.SessionViewSnapshot
type SessionEntryKind = contract.SessionEntryKind
type SessionEntry = contract.SessionEntry
type SessionPlanState = contract.SessionPlanState
type SessionPlanEntry = contract.SessionPlanEntry
type SessionMetaState = contract.SessionMetaState
type SessionAvailableCommand = contract.SessionAvailableCommand
type SessionStatus = contract.SessionStatus
type ToolCallState = contract.ToolCallState
type ContentBlock = contract.ContentBlock
type ContentBlockType = contract.ContentBlockType
type Run = contract.Run
type RunJobSummary = contract.RunJobSummary
type RunJobState = contract.RunJobState
type RunTranscriptMessage = contract.RunTranscriptMessage
type RunShutdownState = contract.RunShutdownState
type RunSnapshot = contract.RunSnapshot

// RunJobCounts summarizes run jobs by status.
type RunJobCounts struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Retrying  int `json:"retrying"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Canceled  int `json:"canceled"`
}

// RunRuntimeSummary captures the distinct runtime settings observed in one run snapshot.
type RunRuntimeSummary struct {
	IDEs              []string `json:"ides,omitempty"`
	Models            []string `json:"models,omitempty"`
	ReasoningEfforts  []string `json:"reasoning_efforts,omitempty"`
	AccessModes       []string `json:"access_modes,omitempty"`
	PresentationModes []string `json:"presentation_modes,omitempty"`
}

// RunArtifactSyncEntry is one artifact sync history row from the run database.
type RunArtifactSyncEntry struct {
	Sequence     uint64    `json:"sequence"`
	RelativePath string    `json:"relative_path"`
	ChangeKind   string    `json:"change_kind"`
	Checksum     string    `json:"checksum,omitempty"`
	SyncedAt     time.Time `json:"synced_at"`
}

// RunDetailPayload captures the richer run detail read model exposed to the browser.
type RunDetailPayload struct {
	Run          Run                    `json:"run"`
	Snapshot     RunSnapshot            `json:"snapshot"`
	JobCounts    RunJobCounts           `json:"job_counts"`
	Runtime      RunRuntimeSummary      `json:"runtime"`
	Timeline     []events.Event         `json:"timeline,omitempty"`
	ArtifactSync []RunArtifactSyncEntry `json:"artifact_sync,omitempty"`
}

type RunListQuery = contract.RunListQuery
type RunEventPageQuery = contract.RunEventPageQuery
type RunEventPage = contract.RunEventPage
type TaskRunRequest = contract.TaskRunRequest
type ReviewRunRequest = contract.ReviewRunRequest
type SyncRequest = contract.SyncRequest
type SyncResult = contract.SyncResult
type ExecRequest = contract.ExecRequest
