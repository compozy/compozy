package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/rundb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

// QueryService exposes the daemon-side read-model assembly required by the web UI.
type QueryService interface {
	Dashboard(ctx context.Context, workspaceRef string) (WorkspaceDashboard, error)
	WorkflowOverview(ctx context.Context, workspaceRef string, workflowSlug string) (WorkflowOverviewPayload, error)
	TaskBoard(ctx context.Context, workspaceRef string, workflowSlug string) (TaskBoardPayload, error)
	WorkflowSpec(ctx context.Context, workspaceRef string, workflowSlug string) (WorkflowSpecDocument, error)
	WorkflowMemoryIndex(ctx context.Context, workspaceRef string, workflowSlug string) (WorkflowMemoryIndex, error)
	WorkflowMemoryFile(
		ctx context.Context,
		workspaceRef string,
		workflowSlug string,
		fileID string,
	) (MarkdownDocument, error)
	TaskDetail(ctx context.Context, workspaceRef string, workflowSlug string, taskID string) (TaskDetailPayload, error)
	ReviewDetail(
		ctx context.Context,
		workspaceRef string,
		workflowSlug string,
		round int,
		issueRef string,
	) (ReviewDetailPayload, error)
	RunDetail(ctx context.Context, runID string) (RunDetailPayload, error)
}

// WorkspaceDashboard is the daemon-side dashboard aggregate payload.
type WorkspaceDashboard struct {
	Workspace      apicore.Workspace
	Daemon         apicore.DaemonStatus
	Health         apicore.DaemonHealth
	Queue          DashboardQueueSummary
	Workflows      []WorkflowCard
	ActiveRuns     []apicore.Run
	PendingReviews int
}

// DashboardQueueSummary summarizes the workspace run queue state.
type DashboardQueueSummary struct {
	Total     int
	Active    int
	Completed int
	Failed    int
	Canceled  int
}

// WorkflowCard is the dashboard-friendly workflow summary.
type WorkflowCard struct {
	Workflow         apicore.WorkflowSummary
	TaskTotal        int
	TaskCompleted    int
	TaskPending      int
	LatestReview     *apicore.ReviewSummary
	ReviewRoundCount int
	ActiveRuns       int
}

// WorkflowOverviewPayload is the daemon-side workflow summary aggregate.
type WorkflowOverviewPayload struct {
	Workspace       apicore.Workspace
	Workflow        apicore.WorkflowSummary
	TaskCounts      WorkflowTaskCounts
	LatestReview    *apicore.ReviewSummary
	RecentRuns      []apicore.Run
	ArchiveEligible bool
	ArchiveReason   string
}

// WorkflowTaskCounts summarizes task state for one workflow.
type WorkflowTaskCounts struct {
	Total     int
	Completed int
	Pending   int
}

// TaskBoardPayload captures the kanban/list task read model.
type TaskBoardPayload struct {
	Workspace  apicore.Workspace
	Workflow   apicore.WorkflowSummary
	TaskCounts WorkflowTaskCounts
	Lanes      []TaskLane
}

// TaskLane groups task cards by normalized status.
type TaskLane struct {
	Status string
	Title  string
	Items  []TaskCard
}

// TaskCard is the transport-neutral task row used by workflow and detail views.
type TaskCard struct {
	TaskNumber int
	TaskID     string
	Title      string
	Status     string
	Type       string
	DependsOn  []string
	UpdatedAt  time.Time
}

// MarkdownDocument is the normalized daemon-side markdown payload.
type MarkdownDocument struct {
	ID        string
	Kind      string
	Title     string
	UpdatedAt time.Time
	Markdown  string
	Metadata  map[string]any
}

// WorkflowSpecDocument captures the canonical workflow spec artifacts.
type WorkflowSpecDocument struct {
	Workspace apicore.Workspace
	Workflow  apicore.WorkflowSummary
	PRD       *MarkdownDocument
	TechSpec  *MarkdownDocument
	ADRs      []MarkdownDocument
}

// WorkflowMemoryIndex lists memory files using opaque IDs.
type WorkflowMemoryIndex struct {
	Workspace apicore.Workspace
	Workflow  apicore.WorkflowSummary
	Entries   []WorkflowMemoryEntry
}

// WorkflowMemoryEntry describes one memory file without exposing a raw source path.
type WorkflowMemoryEntry struct {
	FileID      string
	DisplayPath string
	Kind        string
	Title       string
	SizeBytes   int64
	UpdatedAt   time.Time
}

// TaskDetailPayload captures the daemon-side task detail read model.
type TaskDetailPayload struct {
	Workspace         apicore.Workspace
	Workflow          apicore.WorkflowSummary
	Task              TaskCard
	Document          MarkdownDocument
	MemoryEntries     []WorkflowMemoryEntry
	RelatedRuns       []apicore.Run
	LiveTailAvailable bool
}

// ReviewDetailPayload captures the daemon-side review issue detail read model.
type ReviewDetailPayload struct {
	Workspace   apicore.Workspace
	Workflow    apicore.WorkflowSummary
	Round       apicore.ReviewRound
	Issue       ReviewIssueDetail
	Document    MarkdownDocument
	RelatedRuns []apicore.Run
}

// ReviewIssueDetail captures the detail metadata for one review issue.
type ReviewIssueDetail struct {
	ID          string
	IssueNumber int
	Severity    string
	Status      string
	UpdatedAt   time.Time
}

// RunDetailPayload captures the daemon-side run detail read model.
type RunDetailPayload struct {
	Run          apicore.Run
	Snapshot     apicore.RunSnapshot
	JobCounts    RunJobCounts
	Runtime      RunRuntimeSummary
	Timeline     []eventspkg.Event
	ArtifactSync []rundb.ArtifactSyncRow
}

// RunJobCounts summarizes per-status job counts for one run snapshot.
type RunJobCounts struct {
	Queued    int
	Running   int
	Retrying  int
	Completed int
	Failed    int
	Canceled  int
}

// RunRuntimeSummary captures the distinct runtime settings observed in the run snapshot.
type RunRuntimeSummary struct {
	IDEs              []string
	Models            []string
	ReasoningEfforts  []string
	AccessModes       []string
	PresentationModes []string
}

var (
	// ErrDocumentMissing reports a canonical workflow document missing on disk.
	ErrDocumentMissing = errors.New("daemon: document missing")
	// ErrStaleDocumentReference reports an opaque file id that no longer resolves.
	ErrStaleDocumentReference = errors.New("daemon: stale document reference")
	// ErrReviewIssueNotFound reports a missing review issue within a known round.
	ErrReviewIssueNotFound = errors.New("daemon: review issue not found")
)

// DocumentMissingError reports a missing canonical workflow document.
type DocumentMissingError struct {
	Kind         string
	WorkflowSlug string
	RelativePath string
}

func (e DocumentMissingError) Error() string {
	return fmt.Sprintf(
		"daemon: %s document %q is missing for workflow %q",
		e.Kind,
		e.RelativePath,
		e.WorkflowSlug,
	)
}

func (e DocumentMissingError) Is(target error) bool {
	return target == ErrDocumentMissing
}

// StaleDocumentReferenceError reports an opaque memory file identifier that no longer resolves.
type StaleDocumentReferenceError struct {
	Kind         string
	WorkflowSlug string
	Reference    string
}

func (e StaleDocumentReferenceError) Error() string {
	return fmt.Sprintf(
		"daemon: %s reference %q is stale for workflow %q",
		e.Kind,
		e.Reference,
		e.WorkflowSlug,
	)
}

func (e StaleDocumentReferenceError) Is(target error) bool {
	return target == ErrStaleDocumentReference
}

// ReviewIssueNotFoundError reports a missing issue inside a known review round.
type ReviewIssueNotFoundError struct {
	WorkflowSlug string
	Round        int
	IssueRef     string
}

func (e ReviewIssueNotFoundError) Error() string {
	return fmt.Sprintf(
		"daemon: review issue %q was not found for workflow %q round %d",
		e.IssueRef,
		e.WorkflowSlug,
		e.Round,
	)
}

func (e ReviewIssueNotFoundError) Is(target error) bool {
	return target == ErrReviewIssueNotFound
}
