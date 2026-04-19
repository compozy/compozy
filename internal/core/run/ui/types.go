package ui

import (
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	uiDispatchInterval     = time.Second / 60
	uiSpinnerTickInterval  = 100 * time.Millisecond
	uiClockTickInterval    = time.Second
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

type jobState int

const (
	jobPending jobState = iota
	jobRunning
	jobRetrying
	jobSuccess
	jobFailed
)

type uiJob struct {
	codeFile             string
	codeFiles            []string
	issues               int
	taskTitle            string
	taskType             string
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

type sidebarRowCacheKey struct {
	selected       bool
	width          int
	state          jobState
	safeName       string
	issues         int
	fileCount      int
	attempt        int
	maxAttempts    int
	retrying       bool
	retryReason    string
	elapsedSeconds int64
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

type dispatchBatchMsg struct {
	msgs []uiMsg
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
