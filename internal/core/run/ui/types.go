package ui

import (
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	uiDispatchInterval     = time.Second / 60
	uiSpinnerTickInterval  = 100 * time.Millisecond
	uiClockTickInterval    = time.Second
	quitDialogMaxWidth     = 72
	sidebarWidthRatio      = 0.25
	sidebarMinWidth        = 30
	sidebarMaxWidth        = 50
	mainMinWidth           = 60
	timelineMinWidth       = 44
	minContentHeight       = 10
	mainHorizontalPadding  = 2
	logViewportMinHeight   = 6
	sidebarViewportMinRows = 6
	headerSectionHeight    = 1
	helpSectionHeight      = 1
	separatorSectionHeight = 1
	// chromeHeightStandalone reserves the single-row brand+tabs header, the single-row
	// footer, and the two horizontal dividers that bracket the body (one under the
	// header, one above the footer) — mirroring the wizard's header / HR / body / HR /
	// footer rhythm.
	chromeHeightStandalone = headerSectionHeight + helpSectionHeight + 2*separatorSectionHeight
	// chromeHeightEmbedded applies when the cockpit is rendered as a child of the
	// tabbed multi-run/review-watch shells: the parent owns the brand+tabs row and the
	// divider beneath it, so the child only reserves its footer and the divider above
	// it.
	chromeHeightEmbedded = helpSectionHeight + separatorSectionHeight
	// sidebarRowLines is the rendered height of one job card: a bordered box of
	// top border + title + meta + bottom border.
	sidebarRowLines = 4
	// sidebarRowStride is the distance from one stacked card's top border to the
	// next. Adjacent cards share one separator border row, so each card after the
	// first adds one fewer line than a standalone card.
	sidebarRowStride = sidebarRowLines - 1
	// sidebarHeaderRows is the number of rows reserved at the top of the sidebar
	// panel for the JOB status line and progress meter (kept in sync with renderSidebar).
	sidebarHeaderRows = 2
)

type jobState int

const (
	jobPending jobState = iota
	jobRunning
	jobPausing
	jobPaused
	jobRetrying
	jobSuccess
	jobFailed
)

const (
	statusLabelRunning  = "RUNNING"
	statusLabelFailed   = "FAILED"
	statusLabelCrashed  = "CRASHED"
	statusLabelCanceled = "CANCELED"
	statusLabelDone     = "DONE"
)

type uiJob struct {
	codeFile             string
	codeFiles            []string
	issues               int
	taskNumber           int
	taskTitle            string
	taskType             string
	childRunID           string
	worktreePath         string
	safeName             string
	ide                  string
	model                string
	reasoningEffort      string
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
	sidebarCacheKey      sidebarRowCacheKey
	sidebarCacheRow      string
	sidebarCacheValid    bool
}

type timelineMountState struct {
	jobIndex          int
	width             int
	revision          int
	selectedEntry     int
	expansionRevision int
	valid             bool
}

type sidebarRowCacheKey struct {
	selected       bool
	width          int
	index          int
	taskNumber     int
	state          jobState
	safeName       string
	taskTitle      string
	taskType       string
	attempt        int
	maxAttempts    int
	elapsedSeconds int64
	inputTokens    int
	outputTokens   int
	totalTokens    int
	spinnerFrame   int
}

type clockTickMsg struct {
	at time.Time
}

type spinnerTickMsg struct {
	at time.Time
}

type jobQueuedMsg struct {
	Index           int
	CodeFile        string
	CodeFiles       []string
	Issues          int
	TaskNumber      int
	TaskTitle       string
	TaskType        string
	SafeName        string
	IDE             string
	Model           string
	ReasoningEffort string
	OutLog          string
	ErrLog          string
	OutBuffer       *lineBuffer
	ErrBuffer       *lineBuffer
}

type jobStartedMsg struct {
	Index           int
	Attempt         int
	MaxAttempts     int
	IDE             string
	Model           string
	ReasoningEffort string
}

type jobRetryMsg struct {
	Index       int
	Attempt     int
	MaxAttempts int
	Reason      string
}

type jobPausingMsg struct {
	Index int
}

type jobPausedMsg struct {
	Index int
}

type jobResumedMsg struct {
	Index     int
	MessageID string
}

type jobFinishedMsg struct {
	Index    int
	Success  bool
	ExitCode int
}

type jobUpdateMsg struct {
	Index             int
	Snapshot          SessionViewSnapshot
	UpdateKind        model.SessionUpdateKind
	ToolCallID        string
	ToolCallState     model.ToolCallState
	SessionStatus     model.SessionStatus
	HydrateTranslator bool
}

type drainMsg struct{}

type usageUpdateMsg struct {
	Index int
	Usage model.Usage
}

type runStatusMsg struct {
	Status string
}

type shutdownStatusMsg struct {
	State shutdownState
}

type jobFailureMsg struct {
	Failure failInfo
}

type jobControlResultMsg struct {
	Index    int
	Action   uiJobControlAction
	Response model.JobControlResponse
	Err      error
}

type dispatchBatchMsg struct {
	msgs []uiMsg
}

// Parallel task execution messages translated from task.parallel.* events. They
// drive the wave-grouped sidebar and the persistent INTEGRATION pane.
type parallelPlanStartedMsg struct {
	Workflow          string
	IntegrationBranch string
	ParallelLimit     int
	Tasks             []parallelPlanTask
	Waves             []parallelPlanWave
}

type parallelPlanTask struct {
	ID           string
	Number       int
	Title        string
	File         string
	Status       string
	Dependencies []string
	WaveIndex    int
}

type parallelPlanWave struct {
	Index   int
	TaskIDs []string
}

type parallelWaveStartedMsg struct {
	WaveIndex         int
	WaveTotal         int
	TaskID            string
	IntegrationBranch string
}

type parallelTaskStartedMsg struct {
	WaveIndex         int
	WaveTotal         int
	TaskID            string
	ChildRunID        string
	WorktreePath      string
	IntegrationBranch string
}

type parallelMergeStartedMsg struct {
	WaveIndex         int
	WaveTotal         int
	IntegrationBranch string
}

type parallelConflictMsg struct {
	WaveIndex   int
	TaskID      string
	Files       []string
	Attempt     int
	MaxAttempts int
	Resolving   bool
}

type parallelMergedMsg struct {
	WaveIndex int
	TaskID    string
	Status    string
}

type parallelWaveCompletedMsg struct {
	WaveIndex int
	WaveTotal int
}

type parallelRolledBackMsg struct {
	WaveIndex         int
	IntegrationBranch string
}

type parallelFailedMsg struct {
	WaveIndex         int
	IntegrationBranch string
	Err               error
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
	uiPaneComposer uiPane = "composer"
)

type uiLayoutMode string

const (
	uiLayoutSplit         uiLayoutMode = "split"
	uiLayoutResizeBlocked uiLayoutMode = "resize_blocked"
)

type quitDialogAction int

const (
	quitDialogActionClose quitDialogAction = iota
	quitDialogActionStop
	quitDialogActionCancel
)

type quitDialogState struct {
	Active   bool
	Selected quitDialogAction
}

func newQuitDialogState() quitDialogState {
	return quitDialogState{Selected: quitDialogActionClose}
}

func (s *quitDialogState) Open() {
	if s == nil {
		return
	}
	s.Active = true
	s.Selected = quitDialogActionClose
}

func (s *quitDialogState) Close() {
	if s == nil {
		return
	}
	s.Active = false
	s.Selected = quitDialogActionClose
}

func (s *quitDialogState) Move(delta int) {
	if s == nil {
		return
	}
	actions := []quitDialogAction{
		quitDialogActionClose,
		quitDialogActionStop,
		quitDialogActionCancel,
	}
	current := 0
	for idx, action := range actions {
		if action == s.Selected {
			current = idx
			break
		}
	}
	next := (current + delta + len(actions)) % len(actions)
	s.Selected = actions[next]
}
