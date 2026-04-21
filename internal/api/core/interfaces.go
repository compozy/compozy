package core

import (
	"context"
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
type Workspace = contract.Workspace
type WorkspaceRegisterResult = contract.WorkspaceRegisterResult
type WorkspaceUpdateInput = contract.WorkspaceUpdateInput
type WorkflowSummary = contract.WorkflowSummary
type TaskItem = contract.TaskItem
type ValidationSuccess = contract.ValidationSuccess
type ArchiveResult = contract.ArchiveResult
type ReviewFetchRequest = contract.ReviewFetchRequest
type ReviewFetchResult = contract.ReviewFetchResult
type ReviewSummary = contract.ReviewSummary
type ReviewRound = contract.ReviewRound
type ReviewIssue = contract.ReviewIssue
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
type RunListQuery = contract.RunListQuery
type RunEventPageQuery = contract.RunEventPageQuery
type RunEventPage = contract.RunEventPage
type TaskRunRequest = contract.TaskRunRequest
type ReviewRunRequest = contract.ReviewRunRequest
type SyncRequest = contract.SyncRequest
type SyncResult = contract.SyncResult
type ExecRequest = contract.ExecRequest
