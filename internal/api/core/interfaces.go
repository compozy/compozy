package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/core/run/transcript"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const defaultHeartbeatInterval = 15 * time.Second

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
}

// TaskService exposes task workflow summary, validation, and run start surfaces.
type TaskService interface {
	ListWorkflows(context.Context, string) ([]WorkflowSummary, error)
	GetWorkflow(context.Context, string, string) (WorkflowSummary, error)
	ListItems(context.Context, string, string) ([]TaskItem, error)
	Validate(context.Context, string, string) (ValidationSuccess, error)
	StartRun(context.Context, string, string, TaskRunRequest) (Run, error)
	Archive(context.Context, string, string) (ArchiveResult, error)
}

// ReviewService exposes review round state and review-fix run starts.
type ReviewService interface {
	Fetch(context.Context, string, string, ReviewFetchRequest) (ReviewFetchResult, error)
	GetLatest(context.Context, string, string) (ReviewSummary, error)
	GetRound(context.Context, string, string, int) (ReviewRound, error)
	ListIssues(context.Context, string, string, int) ([]ReviewIssue, error)
	StartRun(context.Context, string, string, int, ReviewRunRequest) (Run, error)
}

// RunService exposes run snapshots, pagination, streaming, and cancellation.
type RunService interface {
	List(context.Context, RunListQuery) ([]Run, error)
	Get(context.Context, string) (Run, error)
	Snapshot(context.Context, string) (RunSnapshot, error)
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
	Reason string `json:"reason,omitempty"`
}

// DaemonStatus is the primary daemon status payload.
type DaemonStatus struct {
	PID            int       `json:"pid"`
	Version        string    `json:"version,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	SocketPath     string    `json:"socket_path,omitempty"`
	HTTPPort       int       `json:"http_port,omitempty"`
	ActiveRunCount int       `json:"active_run_count"`
	WorkspaceCount int       `json:"workspace_count"`
}

// DaemonHealth is the daemon readiness and degradation view.
type DaemonHealth struct {
	Ready    bool           `json:"ready"`
	Degraded bool           `json:"degraded,omitempty"`
	Details  []HealthDetail `json:"details,omitempty"`
}

// HealthDetail describes one health issue or degraded state.
type HealthDetail struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

// MetricsPayload carries pre-rendered metrics text.
type MetricsPayload struct {
	Body        string
	ContentType string
}

// Workspace is the transport-facing workspace payload.
type Workspace struct {
	ID        string    `json:"id"`
	RootDir   string    `json:"root_dir"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkspaceRegisterResult captures an idempotent register result.
type WorkspaceRegisterResult struct {
	Workspace Workspace
	Created   bool
}

// WorkspaceUpdateInput describes mutable operator-managed workspace fields.
type WorkspaceUpdateInput struct {
	Name string `json:"name,omitempty"`
}

// WorkflowSummary describes one task workflow summary.
type WorkflowSummary struct {
	ID           string     `json:"id"`
	WorkspaceID  string     `json:"workspace_id"`
	Slug         string     `json:"slug"`
	ArchivedAt   *time.Time `json:"archived_at,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

// TaskItem describes one parsed task row for a workflow.
type TaskItem struct {
	ID         string    `json:"id"`
	TaskNumber int       `json:"task_number"`
	TaskID     string    `json:"task_id"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Type       string    `json:"type"`
	DependsOn  []string  `json:"depends_on,omitempty"`
	SourcePath string    `json:"source_path"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ValidationSuccess captures successful validation.
type ValidationSuccess struct {
	Valid     bool      `json:"valid"`
	CheckedAt time.Time `json:"checked_at,omitempty"`
}

// ArchiveResult captures an archive mutation.
type ArchiveResult struct {
	Archived   bool       `json:"archived"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

// ReviewFetchRequest describes one review-fetch operation.
type ReviewFetchRequest struct {
	Workspace string `json:"workspace"`
	Provider  string `json:"provider,omitempty"`
	PRRef     string `json:"pr_ref,omitempty"`
	Round     *int   `json:"round,omitempty"`
}

// ReviewFetchResult captures an idempotent review import.
type ReviewFetchResult struct {
	Summary ReviewSummary
	Created bool
}

// ReviewSummary describes the latest review state for one workflow.
type ReviewSummary struct {
	WorkflowSlug    string    `json:"workflow_slug"`
	RoundNumber     int       `json:"round_number"`
	Provider        string    `json:"provider,omitempty"`
	PRRef           string    `json:"pr_ref,omitempty"`
	ResolvedCount   int       `json:"resolved_count"`
	UnresolvedCount int       `json:"unresolved_count"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ReviewRound describes one persisted review round.
type ReviewRound struct {
	ID              string    `json:"id"`
	WorkflowSlug    string    `json:"workflow_slug"`
	RoundNumber     int       `json:"round_number"`
	Provider        string    `json:"provider,omitempty"`
	PRRef           string    `json:"pr_ref,omitempty"`
	ResolvedCount   int       `json:"resolved_count"`
	UnresolvedCount int       `json:"unresolved_count"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ReviewIssue describes one review issue row.
type ReviewIssue struct {
	ID          string    `json:"id"`
	IssueNumber int       `json:"issue_number"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	SourcePath  string    `json:"source_path"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Run describes the transport-facing run summary.
type Run struct {
	RunID            string     `json:"run_id"`
	WorkspaceID      string     `json:"workspace_id"`
	WorkflowID       *string    `json:"workflow_id,omitempty"`
	WorkflowSlug     string     `json:"workflow_slug,omitempty"`
	Mode             string     `json:"mode"`
	Status           string     `json:"status"`
	PresentationMode string     `json:"presentation_mode"`
	StartedAt        time.Time  `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at,omitempty"`
	ErrorText        string     `json:"error_text,omitempty"`
	RequestID        string     `json:"request_id,omitempty"`
}

// RunJobSummary is the dense per-job snapshot used by attach clients.
type RunJobSummary struct {
	Index           int                            `json:"index"`
	CodeFile        string                         `json:"code_file,omitempty"`
	CodeFiles       []string                       `json:"code_files,omitempty"`
	Issues          int                            `json:"issues,omitempty"`
	TaskTitle       string                         `json:"task_title,omitempty"`
	TaskType        string                         `json:"task_type,omitempty"`
	SafeName        string                         `json:"safe_name,omitempty"`
	IDE             string                         `json:"ide,omitempty"`
	Model           string                         `json:"model,omitempty"`
	ReasoningEffort string                         `json:"reasoning_effort,omitempty"`
	AccessMode      string                         `json:"access_mode,omitempty"`
	OutLog          string                         `json:"out_log,omitempty"`
	ErrLog          string                         `json:"err_log,omitempty"`
	Attempt         int                            `json:"attempt,omitempty"`
	MaxAttempts     int                            `json:"max_attempts,omitempty"`
	RetryReason     string                         `json:"retry_reason,omitempty"`
	ExitCode        int                            `json:"exit_code,omitempty"`
	ErrorText       string                         `json:"error_text,omitempty"`
	Session         transcript.SessionViewSnapshot `json:"session,omitempty"`
	Usage           kinds.Usage                    `json:"usage,omitempty"`
}

// RunJobState is the dense job-state snapshot used by attach clients.
type RunJobState struct {
	Index     int            `json:"index"`
	JobID     string         `json:"job_id"`
	TaskID    string         `json:"task_id,omitempty"`
	Status    string         `json:"status"`
	AgentName string         `json:"agent_name,omitempty"`
	Summary   *RunJobSummary `json:"summary,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// RunTranscriptMessage is one dense transcript row for snapshot rendering.
type RunTranscriptMessage struct {
	Sequence    uint64          `json:"sequence"`
	Stream      string          `json:"stream"`
	Role        string          `json:"role"`
	Content     string          `json:"content"`
	MetadataRaw json.RawMessage `json:"metadata,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
}

// RunShutdownState captures a client-visible shutdown state for remote attach.
type RunShutdownState struct {
	Phase       string    `json:"phase,omitempty"`
	Source      string    `json:"source,omitempty"`
	RequestedAt time.Time `json:"requested_at,omitempty"`
	DeadlineAt  time.Time `json:"deadline_at,omitempty"`
}

// RunSnapshot captures the attach snapshot plus the next cursor.
type RunSnapshot struct {
	Run        Run                    `json:"run"`
	Jobs       []RunJobState          `json:"jobs,omitempty"`
	Transcript []RunTranscriptMessage `json:"transcript,omitempty"`
	Usage      kinds.Usage            `json:"usage,omitempty"`
	Shutdown   *RunShutdownState      `json:"shutdown,omitempty"`
	NextCursor *StreamCursor          `json:"-"`
}

// RunListQuery filters run listing.
type RunListQuery struct {
	Workspace string
	Status    string
	Mode      string
	Limit     int
}

// RunEventPageQuery paginates persisted run events.
type RunEventPageQuery struct {
	After StreamCursor
	Limit int
}

// RunEventPage carries a page of persisted run events plus the next cursor.
type RunEventPage struct {
	Events     []events.Event
	NextCursor *StreamCursor
	HasMore    bool
}

// TaskRunRequest describes a task workflow run start request.
type TaskRunRequest struct {
	Workspace        string          `json:"workspace"`
	PresentationMode string          `json:"presentation_mode,omitempty"`
	RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
}

// ReviewRunRequest describes a review-fix run start request.
type ReviewRunRequest struct {
	Workspace        string          `json:"workspace"`
	PresentationMode string          `json:"presentation_mode,omitempty"`
	RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
	Batching         json.RawMessage `json:"batching,omitempty"`
}

// SyncRequest describes an explicit sync request.
type SyncRequest struct {
	Workspace    string `json:"workspace,omitempty"`
	Path         string `json:"path,omitempty"`
	WorkflowSlug string `json:"workflow_slug,omitempty"`
}

// SyncResult captures sync completion details.
type SyncResult struct {
	WorkspaceID  string     `json:"workspace_id,omitempty"`
	WorkflowSlug string     `json:"workflow_slug,omitempty"`
	SyncedAt     *time.Time `json:"synced_at,omitempty"`
}

// ExecRequest describes one ad-hoc daemon-backed exec request.
type ExecRequest struct {
	WorkspacePath    string          `json:"workspace_path"`
	Prompt           string          `json:"prompt"`
	PresentationMode string          `json:"presentation_mode,omitempty"`
	RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
}
