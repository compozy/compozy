package run

import (
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	exitCodeTimeout               = -2
	exitCodeCanceled              = -1
	activityCheckInterval         = 5 * time.Second
	processTerminationGracePeriod = 3 * time.Second
	gracefulShutdownTimeout       = 3 * time.Second
	uiTickInterval                = 120 * time.Millisecond
)

type failInfo struct {
	codeFile string
	exitCode int
	outLog   string
	errLog   string
	err      error
}

type jobPhase string

const (
	jobPhaseQueued    jobPhase = "queued"
	jobPhaseScheduled jobPhase = "scheduled"
	jobPhaseRunning   jobPhase = "running"
	jobPhaseRetrying  jobPhase = "retrying"
	jobPhaseSucceeded jobPhase = "succeeded"
	jobPhaseFailed    jobPhase = "failed"
	jobPhaseCanceled  jobPhase = "canceled"
)

type jobAttemptStatus string

const (
	attemptStatusSuccess     jobAttemptStatus = "success"
	attemptStatusFailure     jobAttemptStatus = "failure"
	attemptStatusTimeout     jobAttemptStatus = "timeout"
	attemptStatusCanceled    jobAttemptStatus = "canceled"
	attemptStatusSetupFailed jobAttemptStatus = "setup_failed"
)

type jobAttemptResult struct {
	status    jobAttemptStatus
	exitCode  int
	failure   *failInfo
	retryable bool
}

func (r jobAttemptResult) Successful() bool {
	return r.status == attemptStatusSuccess
}

func (r jobAttemptResult) NeedsRetry() bool {
	return r.retryable
}

func (r jobAttemptResult) IsCanceled() bool {
	return r.status == attemptStatusCanceled
}

type jobState int

const (
	jobPending jobState = iota
	jobRunning
	jobRetrying
	jobSuccess
	jobFailed
)

const (
	sidebarWidthRatio      = 0.25
	sidebarMinWidth        = 30
	sidebarMaxWidth        = 50
	mainMinWidth           = 60
	timelineMinWidth       = 44
	minContentHeight       = 10
	mainHorizontalPadding  = 2
	logViewportMinHeight   = 6
	sidebarViewportMinRows = 5
	headerSectionHeight    = 3
	helpSectionHeight      = 2
	separatorSectionHeight = 1
	chromeHeight           = headerSectionHeight + helpSectionHeight + separatorSectionHeight
)

type uiJob struct {
	codeFile             string
	codeFiles            []string
	issues               int
	taskTitle            string
	taskType             string
	safeName             string
	outLog               string
	errLog               string
	state                jobState
	exitCode             int
	outBuffer            *lineBuffer
	errBuffer            *lineBuffer
	startedAt            time.Time
	completedAt          time.Time
	duration             time.Duration
	attempt              int
	maxAttempts          int
	retrying             bool
	retryReason          string
	tokenUsage           *model.Usage
	snapshot             SessionViewSnapshot
	selectedEntry        int
	expandedEntryIDs     map[string]bool
	expansionRevision    int
	transcriptFollowTail bool
	transcriptYOffset    int
	transcriptXOffset    int
	timelineCache        timelineRender
	timelineCacheWidth   int
	timelineCacheRev     int
	timelineCacheSel     int
	timelineCacheExpand  int
	timelineCacheValid   bool
}

type shutdownPhase string

const (
	shutdownPhaseIdle     shutdownPhase = ""
	shutdownPhaseDraining shutdownPhase = "draining"
	shutdownPhaseForcing  shutdownPhase = "forcing"
)

type shutdownSource string

const (
	shutdownSourceUI     shutdownSource = "ui"
	shutdownSourceSignal shutdownSource = "signal"
	shutdownSourceTimer  shutdownSource = "timer"
)

type shutdownState struct {
	Phase       shutdownPhase
	Source      shutdownSource
	RequestedAt time.Time
	DeadlineAt  time.Time
}

func (s shutdownState) active() bool {
	return s.Phase != shutdownPhaseIdle
}

type uiQuitRequest int

const (
	uiQuitRequestDrain uiQuitRequest = iota
	uiQuitRequestForce
)

type tickMsg struct{}

type jobQueuedMsg struct {
	Index     int
	CodeFile  string
	CodeFiles []string
	Issues    int
	TaskTitle string
	TaskType  string
	SafeName  string
	OutLog    string
	ErrLog    string
	OutBuffer *lineBuffer
	ErrBuffer *lineBuffer
}

type jobStartedMsg struct {
	Index       int
	Attempt     int
	MaxAttempts int
}

type jobRetryMsg struct {
	Index       int
	Attempt     int
	MaxAttempts int
	Reason      string
}

type jobFinishedMsg struct {
	Index    int
	Success  bool
	ExitCode int
}

type jobUpdateMsg struct {
	Index    int
	Snapshot SessionViewSnapshot
}

type drainMsg struct{}

type usageUpdateMsg struct {
	Index int
	Usage model.Usage
}

type shutdownStatusMsg struct {
	State shutdownState
}

type jobFailureMsg struct {
	Failure failInfo
}

type uiViewState string

const (
	uiViewJobs     uiViewState = "jobs"
	uiViewSummary  uiViewState = "summary"
	uiViewFailures uiViewState = "failures"
)

type uiMsg any

type uiPane string

const (
	uiPaneJobs     uiPane = "jobs"
	uiPaneTimeline uiPane = "timeline"
)

type uiLayoutMode string

const (
	uiLayoutSplit         uiLayoutMode = "split"
	uiLayoutResizeBlocked uiLayoutMode = "resize_blocked"
)

type uiSession interface {
	enqueue(uiMsg)
	setQuitHandler(func(uiQuitRequest))
	closeEvents()
	shutdown()
	wait() error
}

type config struct {
	workspaceRoot          string
	name                   string
	round                  int
	provider               string
	pr                     string
	reviewsDir             string
	tasksDir               string
	dryRun                 bool
	autoCommit             bool
	concurrent             int
	batchSize              int
	ide                    string
	model                  string
	addDirs                []string
	tailLines              int
	reasoningEffort        string
	accessMode             string
	mode                   model.ExecutionMode
	outputFormat           model.OutputFormat
	verbose                bool
	tui                    bool
	persist                bool
	runID                  string
	runArtifacts           model.RunArtifacts
	includeCompleted       bool
	includeResolved        bool
	timeout                time.Duration
	maxRetries             int
	retryBackoffMultiplier float64
}

type job struct {
	codeFiles     []string
	groups        map[string][]model.IssueEntry
	taskTitle     string
	taskType      string
	safeName      string
	prompt        []byte
	systemPrompt  string
	resumeRunID   string
	resumeSession string
	outPromptPath string
	outLog        string
	errLog        string
	status        string
	failure       string
	exitCode      int
	usage         model.Usage
	outBuffer     *lineBuffer
	errBuffer     *lineBuffer
}

func (cfg *config) humanOutputEnabled() bool {
	return cfg != nil && (cfg.outputFormat == "" || cfg.outputFormat == model.OutputFormatText)
}

func (cfg *config) uiEnabled() bool {
	return cfg != nil && cfg.humanOutputEnabled() && !cfg.dryRun
}

func newConfig(src *model.RuntimeConfig, runArtifacts model.RunArtifacts) *config {
	if src == nil {
		return nil
	}
	return &config{
		workspaceRoot:          src.WorkspaceRoot,
		name:                   src.Name,
		round:                  src.Round,
		provider:               src.Provider,
		pr:                     src.PR,
		reviewsDir:             src.ReviewsDir,
		tasksDir:               src.TasksDir,
		dryRun:                 src.DryRun,
		autoCommit:             src.AutoCommit,
		concurrent:             src.Concurrent,
		batchSize:              src.BatchSize,
		ide:                    src.IDE,
		model:                  src.Model,
		addDirs:                append([]string(nil), src.AddDirs...),
		tailLines:              src.TailLines,
		reasoningEffort:        src.ReasoningEffort,
		accessMode:             src.AccessMode,
		mode:                   src.Mode,
		outputFormat:           src.OutputFormat,
		verbose:                src.Verbose,
		tui:                    src.TUI,
		persist:                src.Persist,
		runID:                  src.RunID,
		runArtifacts:           runArtifacts,
		includeCompleted:       src.IncludeCompleted,
		includeResolved:        src.IncludeResolved,
		timeout:                src.Timeout,
		maxRetries:             src.MaxRetries,
		retryBackoffMultiplier: src.RetryBackoffMultiplier,
	}
}

func newJobs(src []model.Job) []job {
	jobs := make([]job, 0, len(src))
	for i := range src {
		item := &src[i]
		jobs = append(jobs, job{
			codeFiles:     append([]string(nil), item.CodeFiles...),
			groups:        cloneGroups(item.Groups),
			taskTitle:     item.TaskTitle,
			taskType:      item.TaskType,
			safeName:      item.SafeName,
			prompt:        append([]byte(nil), item.Prompt...),
			systemPrompt:  item.SystemPrompt,
			outPromptPath: item.OutPromptPath,
			outLog:        item.OutLog,
			errLog:        item.ErrLog,
		})
	}
	return jobs
}

func cloneGroups(src map[string][]model.IssueEntry) map[string][]model.IssueEntry {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string][]model.IssueEntry, len(src))
	for key, entries := range src {
		items := make([]model.IssueEntry, len(entries))
		copy(items, entries)
		cloned[key] = items
	}
	return cloned
}
