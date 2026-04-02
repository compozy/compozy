package run

import (
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	exitCodeTimeout               = -2
	exitCodeCanceled              = -1
	processTerminationGracePeriod = 5 * time.Second
	gracefulShutdownTimeout       = 30 * time.Second
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
	status   jobAttemptStatus
	exitCode int
	failure  *failInfo
}

func (r jobAttemptResult) Successful() bool {
	return r.status == attemptStatusSuccess
}

func (r jobAttemptResult) NeedsRetry() bool {
	return r.status == attemptStatusFailure || r.status == attemptStatusTimeout
}

func (r jobAttemptResult) IsCanceled() bool {
	return r.status == attemptStatusCanceled
}

type jobState int

const (
	jobPending jobState = iota
	jobRunning
	jobSuccess
	jobFailed
)

const (
	sidebarWidthRatio      = 0.25
	sidebarMinWidth        = 30
	sidebarMaxWidth        = 50
	mainMinWidth           = 60
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
	codeFile        string
	codeFiles       []string
	issues          int
	safeName        string
	outLog          string
	errLog          string
	state           jobState
	exitCode        int
	outBuffer       *lineBuffer
	errBuffer       *lineBuffer
	followTail      bool
	viewportYOffset int
	viewportXOffset int
	startedAt       time.Time
	completedAt     time.Time
	duration        time.Duration
	tokenUsage      *model.Usage
	blocks          []model.ContentBlock
}

type tickMsg struct{}

type jobQueuedMsg struct {
	Index     int
	CodeFile  string
	CodeFiles []string
	Issues    int
	SafeName  string
	OutLog    string
	ErrLog    string
	OutBuffer *lineBuffer
	ErrBuffer *lineBuffer
}

type jobStartedMsg struct{ Index int }

type jobFinishedMsg struct {
	Index    int
	Success  bool
	ExitCode int
}

type jobUpdateMsg struct {
	Index  int
	Blocks []model.ContentBlock
}

type drainMsg struct{}

type usageUpdateMsg struct {
	Index int
	Usage model.Usage
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

type uiSession interface {
	events() chan uiMsg
	setQuitHandler(func())
	closeEvents()
	shutdown()
	wait() error
}

type config struct {
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
	grouped                bool
	tailLines              int
	reasoningEffort        string
	mode                   model.ExecutionMode
	includeCompleted       bool
	includeResolved        bool
	timeout                time.Duration
	maxRetries             int
	retryBackoffMultiplier float64
}

type job struct {
	codeFiles     []string
	groups        map[string][]model.IssueEntry
	safeName      string
	prompt        []byte
	systemPrompt  string
	outPromptPath string
	outLog        string
	errLog        string
	outBuffer     *lineBuffer
	errBuffer     *lineBuffer
}

func newConfig(src *model.RuntimeConfig) *config {
	if src == nil {
		return nil
	}
	return &config{
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
		grouped:                src.Grouped,
		tailLines:              src.TailLines,
		reasoningEffort:        src.ReasoningEffort,
		mode:                   src.Mode,
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
