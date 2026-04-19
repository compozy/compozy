package ui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

func TestHandleKeyRequestsShutdownWithoutQuittingWhileRunActive(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	var quitRequests []uiQuitRequest
	m.onQuit = func(req uiQuitRequest) {
		quitRequests = append(quitRequests, req)
	}

	cmd := m.handleKey(keyText("q"))
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("expected active run to request shutdown without quitting the UI")
		}
	}
	if got := len(quitRequests); got != 1 {
		t.Fatalf("expected first shutdown callback to be invoked once, got %d", got)
	}
	if got := quitRequests[0]; got != uiQuitRequestDrain {
		t.Fatalf("expected first quit request to start draining, got %v", got)
	}
	if got := m.shutdown.Phase; got != shutdownPhaseDraining {
		t.Fatalf("expected active quit request to mark the UI as draining, got %s", got)
	}

	m.handleKey(keyText("q"))
	if got := len(quitRequests); got != 2 {
		t.Fatalf("expected second quit request to escalate force shutdown, got %d calls", got)
	}
	if got := quitRequests[1]; got != uiQuitRequestForce {
		t.Fatalf("expected second quit request to force shutdown, got %v", got)
	}
	if got := m.shutdown.Phase; got != shutdownPhaseForcing {
		t.Fatalf("expected second quit request to enter forcing state, got %s", got)
	}
}

func TestHandleKeyQuitsOnceRunCompletes(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	quitCalls := 0
	m.onQuit = func(uiQuitRequest) {
		quitCalls++
	}
	m.handleJobFinished(jobFinishedMsg{Index: 0, Success: true})

	cmd := m.handleKey(keyText("q"))
	if cmd == nil {
		t.Fatal("expected completed run to return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected quit command after completion, got %T", cmd())
	}
	if quitCalls != 0 {
		t.Fatalf("expected completion quit to bypass shutdown callback, got %d calls", quitCalls)
	}
}

func TestHandleShutdownStatusUpdatesCountdownState(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	deadline := time.Now().Add(2 * time.Second)

	m.handleShutdownStatus(shutdownStatusMsg{
		State: shutdownState{
			Phase:       shutdownPhaseDraining,
			Source:      shutdownSourceSignal,
			RequestedAt: time.Now(),
			DeadlineAt:  deadline,
		},
	})

	if got := m.shutdown.Phase; got != shutdownPhaseDraining {
		t.Fatalf("expected draining phase from shutdown status, got %s", got)
	}
	if !m.shutdown.DeadlineAt.Equal(deadline) {
		t.Fatalf("expected shutdown deadline to be stored, got %v", m.shutdown.DeadlineAt)
	}
}

func TestHandleUsageUpdateAggregatesUsage(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleUsageUpdate(usageUpdateMsg{
		Index: 0,
		Usage: model.Usage{
			InputTokens:  7,
			OutputTokens: 3,
			TotalTokens:  10,
			CacheReads:   2,
			CacheWrites:  1,
		},
	})

	if got := m.jobs[0].tokenUsage; got == nil || got.TotalTokens != 10 || got.CacheReads != 2 || got.CacheWrites != 1 {
		t.Fatalf("unexpected per-job usage: %#v", got)
	}
	if got := m.aggregateUsage; got == nil || got.TotalTokens != 10 || got.CacheWrites != 1 {
		t.Fatalf("unexpected aggregate usage: %#v", got)
	}
}

func TestHandleJobUpdateStoresSnapshotAndSelectsLatestEntry(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	snapshot := buildSnapshotWithEntries(
		t,
		TranscriptEntry{
			ID:     "assistant-1",
			Kind:   transcriptEntryAssistantMessage,
			Title:  "Assistant",
			Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "hello"})},
		},
		TranscriptEntry{
			ID:            "tool-1",
			Kind:          transcriptEntryToolCall,
			Title:         "read_file",
			ToolCallID:    "tool-1",
			ToolCallState: model.ToolCallStateInProgress,
		},
	)

	m.handleJobUpdate(jobUpdateMsg{Index: 0, Snapshot: snapshot})

	if got := len(m.jobs[0].snapshot.Entries); got != 2 {
		t.Fatalf("expected 2 stored entries, got %d", got)
	}
	if got := m.jobs[0].selectedEntry; got != 1 {
		t.Fatalf("expected selected entry to follow the latest transcript entry, got %d", got)
	}
	if m.jobs[0].expandedEntryIDs["tool-1"] {
		t.Fatalf("expected in-progress tool entry to stay compact by default, got %#v", m.jobs[0].expandedEntryIDs)
	}
}

func TestHandleJobRetryMarksRetryingStateWithoutIncrementingFailures(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobRetry(jobRetryMsg{
		Index:       0,
		Attempt:     2,
		MaxAttempts: 3,
		Reason:      "temporary setup failure",
	})

	job := m.jobs[0]
	if got := job.state; got != jobRetrying {
		t.Fatalf("expected retrying state, got %v", got)
	}
	if !job.retrying {
		t.Fatal("expected retrying flag to be true")
	}
	if job.attempt != 2 || job.maxAttempts != 3 {
		t.Fatalf("unexpected retry attempt metadata: %#v", job)
	}
	if job.retryReason != "temporary setup failure" {
		t.Fatalf("unexpected retry reason: %q", job.retryReason)
	}
	if m.failed != 0 {
		t.Fatalf("expected retry state not to increment failed count, got %d", m.failed)
	}
}

func TestPaneNavigationCyclesVisiblePanes(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 160, Height: 40})
	if got := m.focusedPane; got != uiPaneJobs {
		t.Fatalf("expected initial focus on jobs, got %s", got)
	}

	m.handleKey(keyCode(tea.KeyTab))
	if got := m.focusedPane; got != uiPaneTimeline {
		t.Fatalf("expected tab to move focus to timeline, got %s", got)
	}

	m.handleKey(keyCode(tea.KeyTab))
	if got := m.focusedPane; got != uiPaneJobs {
		t.Fatalf("expected second tab to wrap focus back to jobs, got %s", got)
	}

	m.handleKey(keyText("shift+tab"))
	if got := m.focusedPane; got != uiPaneTimeline {
		t.Fatalf("expected shift+tab to move focus back to timeline, got %s", got)
	}
}

func TestEnterTogglesSelectedEntryExpansion(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focusedPane = uiPaneTimeline
	m.jobs[0].selectedEntry = 1

	entry := m.jobs[0].snapshot.Entries[1]
	if m.isEntryExpanded(&m.jobs[0], entry) {
		t.Fatalf("expected completed tool entry to start collapsed: %#v", entry)
	}

	m.handleKey(keyCode(tea.KeyEnter))
	if !m.isEntryExpanded(&m.jobs[0], entry) {
		t.Fatal("expected enter to expand the selected entry")
	}

	m.handleKey(keyCode(tea.KeyEnter))
	if m.isEntryExpanded(&m.jobs[0], entry) {
		t.Fatal("expected second enter to collapse the selected entry")
	}
}

func TestMoveFocusedSelectionNavigatesTimelineEntries(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 160, Height: 40})
	m.focusedPane = uiPaneTimeline
	m.jobs[0].selectedEntry = 0

	m.handleKey(keyCode(tea.KeyDown))
	if got := m.jobs[0].selectedEntry; got != 1 {
		t.Fatalf("expected down to move timeline selection, got %d", got)
	}

	m.handleKey(keyCode(tea.KeyUp))
	if got := m.jobs[0].selectedEntry; got != 0 {
		t.Fatalf("expected up to restore timeline selection, got %d", got)
	}
}

func TestHandleClockTickRefreshesSidebarWhileJobRunning(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.jobs[0].state = jobRunning
	m.jobs[0].startedAt = time.Unix(0, 0)
	m.now = time.Unix(65, 0)
	m.refreshSidebarContent()
	before := m.sidebarViewport.View()

	m.handleClockTick(clockTickMsg{at: time.Unix(66, 0)})
	after := m.sidebarViewport.View()

	if before == after {
		t.Fatalf("expected running sidebar content to refresh on tick, got %q", after)
	}
}

func TestHandleClockTickSkipsSidebarRefreshWhenIdleAndClean(t *testing.T) {
	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.jobs[0].state = jobSuccess
	m.refreshSidebarContent()
	m.sidebarDirty = false

	previous := setSidebarViewportContent
	t.Cleanup(func() {
		setSidebarViewportContent = previous
	})

	calls := 0
	setSidebarViewportContent = func(vp *viewport.Model, content string) {
		calls++
		previous(vp, content)
	}

	m.handleClockTick(clockTickMsg{at: time.Unix(66, 0)})
	if calls != 0 {
		t.Fatalf("expected idle tick to skip sidebar refresh, got %d SetContent calls", calls)
	}
}

func TestHandleSpinnerTickStopsWhenNoActiveJobsRemain(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.spinnerRunning = true
	m.jobs[0].state = jobSuccess

	if cmd := m.handleSpinnerTick(spinnerTickMsg{at: time.Unix(10, 0)}); cmd != nil {
		t.Fatalf("expected no follow-up spinner command without active jobs, got %v", cmd)
	}
	if m.spinnerRunning {
		t.Fatal("expected spinner loop to stop without active jobs")
	}
}

func TestCurrentJobHandlesSelectionBounds(t *testing.T) {
	t.Parallel()

	m := newUIModel(2)
	if got := m.currentJob(); got != nil {
		t.Fatalf("expected nil current job without queued jobs, got %#v", got)
	}

	m.jobs = []uiJob{{codeFile: "task_01"}, {codeFile: "task_02"}}
	m.selectedJob = 1
	if got := m.currentJob(); got != &m.jobs[1] {
		t.Fatalf("expected selected job pointer, got %#v", got)
	}

	m.selectedJob = 5
	if got := m.currentJob(); got != nil {
		t.Fatalf("expected nil for out-of-range selection, got %#v", got)
	}
}

func TestHandleJobStartedMarksRunningState(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.handleJobStarted(jobStartedMsg{
		Index:       0,
		Attempt:     2,
		MaxAttempts: 3,
	})

	job := m.currentJob()
	if job == nil {
		t.Fatal("expected selected job")
	}
	if got := job.state; got != jobRunning {
		t.Fatalf("expected running state, got %v", got)
	}
	if got := job.attempt; got != 2 {
		t.Fatalf("expected attempt 2, got %d", got)
	}
	if got := job.maxAttempts; got != 3 {
		t.Fatalf("expected max attempts 3, got %d", got)
	}
	if job.startedAt.IsZero() {
		t.Fatal("expected startedAt to be set")
	}
}

func TestHandleJobStartedCreatesPlaceholderForRemoteAttachGap(t *testing.T) {
	t.Parallel()

	m := newUIModel(0)
	m.handleJobStarted(jobStartedMsg{
		Index:           0,
		Attempt:         1,
		MaxAttempts:     1,
		IDE:             "codex",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
	})

	if got := len(m.jobs); got != 1 {
		t.Fatalf("expected placeholder job slice length 1, got %d", got)
	}
	if got := m.total; got != 1 {
		t.Fatalf("expected total 1, got %d", got)
	}
	job := &m.jobs[0]
	if got := job.safeName; got != "job-000" {
		t.Fatalf("expected placeholder safe name job-000, got %q", got)
	}
	if got := job.state; got != jobRunning {
		t.Fatalf("expected running placeholder job, got %v", got)
	}
	if job.startedAt.IsZero() {
		t.Fatal("expected placeholder startedAt to be set")
	}
	if got := job.ide; got != "codex" {
		t.Fatalf("expected ide codex, got %q", got)
	}
}

func TestScrollSidebarViewportRoutesNavigationKeys(t *testing.T) {
	t.Parallel()

	vp := &testScrollableViewport{}
	scrollSidebarViewport(vp, keyPageUp)
	scrollSidebarViewport(vp, keyPageDown)
	scrollSidebarViewport(vp, keyHome)
	scrollSidebarViewport(vp, keyEnd)

	if vp.pageUpCalls != 1 || vp.pageDownCalls != 1 || vp.gotoTopCalls != 1 || vp.gotoBottomCalls != 1 {
		t.Fatalf("unexpected key routing counts: %#v", vp)
	}
}

func TestScrollFocusedPaneUpdatesSelectedViewportState(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.currentView = uiViewJobs
	m.focusedPane = uiPaneJobs
	m.sidebarViewport.SetContent(strings.Repeat("sidebar row\n", 20))
	m.sidebarViewport.SetHeight(3)
	m.sidebarViewport.GotoBottom()
	m.scrollFocusedPane(keyHome)
	if got := m.sidebarViewport.YOffset(); got != 0 {
		t.Fatalf("expected sidebar viewport home offset 0, got %d", got)
	}

	m.focusedPane = uiPaneTimeline
	job := m.currentJob()
	if job == nil {
		t.Fatal("expected current job")
	}
	m.transcriptViewport.SetContent(strings.Repeat("timeline row\n", 20))
	m.transcriptViewport.SetHeight(3)
	m.transcriptViewport.GotoBottom()
	job.transcriptFollowTail = true
	m.scrollFocusedPane(keyHome)
	if got := m.transcriptViewport.YOffset(); got != 0 {
		t.Fatalf("expected transcript viewport home offset 0, got %d", got)
	}
	if job.transcriptFollowTail {
		t.Fatal("expected follow-tail to disable after manual timeline scroll")
	}
}

func TestHandleMouseWheelUpdatesFocusedViewport(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.currentView = uiViewJobs

	m.focusedPane = uiPaneJobs
	m.sidebarViewport.SetContent(strings.Repeat("sidebar row\n", 20))
	m.sidebarViewport.SetHeight(3)
	m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	if got := m.sidebarViewport.YOffset(); got == 0 {
		t.Fatalf("expected sidebar mouse wheel to scroll, got offset %d", got)
	}

	m.focusedPane = uiPaneTimeline
	job := m.currentJob()
	if job == nil {
		t.Fatal("expected current job")
	}
	m.transcriptViewport.SetContent(strings.Repeat("timeline row\n", 20))
	m.transcriptViewport.SetHeight(3)
	m.transcriptViewport.GotoBottom()
	job.transcriptFollowTail = true
	m.handleMouseWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if job.transcriptFollowTail {
		t.Fatal("expected timeline mouse wheel to disable follow-tail")
	}
}

func TestSelectNextRunningJobPrefersRunningThenPending(t *testing.T) {
	t.Parallel()

	m := newUIModel(3)
	m.jobs = []uiJob{
		{state: jobSuccess},
		{state: jobRetrying},
		{state: jobPending},
	}
	m.selectNextRunningJob()
	if got := m.selectedJob; got != 1 {
		t.Fatalf("expected retrying job to be selected first, got %d", got)
	}

	m.jobs[1].state = jobSuccess
	m.selectNextRunningJob()
	if got := m.selectedJob; got != 2 {
		t.Fatalf("expected pending job fallback, got %d", got)
	}
}

func TestHandleJobFinishedUpdatesCountsAndViewState(t *testing.T) {
	t.Parallel()

	t.Run("success clears shutdown when run completes", func(t *testing.T) {
		t.Parallel()

		m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
		m.jobs[0].startedAt = time.Now().Add(-2 * time.Second)
		m.total = 1
		m.shutdown = shutdownState{Phase: shutdownPhaseDraining}

		m.handleJobFinished(jobFinishedMsg{Index: 0, Success: true})

		if got := m.jobs[0].state; got != jobSuccess {
			t.Fatalf("expected success state, got %v", got)
		}
		if got := m.completed; got != 1 {
			t.Fatalf("expected completed count 1, got %d", got)
		}
		if m.shutdown.Active() {
			t.Fatalf("expected shutdown state to clear when run completes, got %#v", m.shutdown)
		}
		if m.jobs[0].duration <= 0 {
			t.Fatalf("expected job duration to be recorded, got %v", m.jobs[0].duration)
		}
	})

	t.Run("failure switches to summary when run completes with errors", func(t *testing.T) {
		t.Parallel()

		m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
		m.total = 1

		m.handleJobFinished(jobFinishedMsg{Index: 0, Success: false, ExitCode: 23})

		if got := m.jobs[0].state; got != jobFailed {
			t.Fatalf("expected failed state, got %v", got)
		}
		if got := m.jobs[0].exitCode; got != 23 {
			t.Fatalf("expected exit code 23, got %d", got)
		}
		if got := m.failed; got != 1 {
			t.Fatalf("expected failed count 1, got %d", got)
		}
		if got := m.currentView; got != uiViewSummary {
			t.Fatalf("expected summary view on completed failure, got %s", got)
		}
	})
}

func TestHandleJobQueuedStoresTaskMetadata(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	m.handleJobQueued(&jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01",
		CodeFiles: []string{"task_01"},
		Issues:    1,
		TaskTitle: "acp agent layer",
		TaskType:  "backend",
		SafeName:  "task_01-safe",
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
		OutBuffer: runshared.NewLineBuffer(0),
		ErrBuffer: runshared.NewLineBuffer(0),
	})

	if got, want := m.jobs[0].taskTitle, "acp agent layer"; got != want {
		t.Fatalf("expected task title %q, got %q", want, got)
	}
	if got, want := m.jobs[0].taskType, "backend"; got != want {
		t.Fatalf("expected task type %q, got %q", want, got)
	}
}

func TestHandleJobQueuedExpandsTotalForRemoteAttach(t *testing.T) {
	t.Parallel()

	m := newUIModel(0)
	m.handleJobQueued(&jobQueuedMsg{
		Index:     2,
		CodeFile:  "task_03",
		CodeFiles: []string{"task_03"},
		OutBuffer: runshared.NewLineBuffer(0),
		ErrBuffer: runshared.NewLineBuffer(0),
	})

	if got := m.total; got != 3 {
		t.Fatalf("expected total jobs to expand to 3, got %d", got)
	}
	if got := len(m.jobs); got != 3 {
		t.Fatalf("expected job slice to expand to 3, got %d", got)
	}
}

func TestHandleJobUpdateCreatesRunningPlaceholderForRemoteAttachGap(t *testing.T) {
	t.Parallel()

	m := newUIModel(0)
	snapshot := buildSnapshotWithEntries(t,
		TranscriptEntry{
			ID:    "assistant-1",
			Kind:  transcriptEntryAssistantMessage,
			Title: "Assistant",
			Blocks: []model.ContentBlock{
				mustContentBlockUITest(t, model.TextBlock{Text: "recover from session update"}),
			},
		},
	)
	snapshot.Session.Status = model.StatusRunning

	m.handleJobUpdate(jobUpdateMsg{
		Index:    0,
		Snapshot: snapshot,
	})

	if got := len(m.jobs); got != 1 {
		t.Fatalf("expected placeholder job slice length 1, got %d", got)
	}
	if got := m.total; got != 1 {
		t.Fatalf("expected total 1, got %d", got)
	}
	job := &m.jobs[0]
	if got := job.safeName; got != "job-000" {
		t.Fatalf("expected placeholder safe name job-000, got %q", got)
	}
	if got := job.state; got != jobRunning {
		t.Fatalf("expected running state from session snapshot, got %v", got)
	}
	if job.startedAt.IsZero() {
		t.Fatal("expected placeholder startedAt to be set from session snapshot")
	}
	if got := len(job.snapshot.Entries); got != 1 {
		t.Fatalf("expected one restored transcript entry, got %#v", job.snapshot.Entries)
	}
}

func newTestUIModelWithSnapshot(t *testing.T, size tea.WindowSizeMsg) *uiModel {
	t.Helper()

	m := newUIModel(1)
	m.cfg = &config{
		IDE:   model.IDEClaude,
		Model: "sonnet-4.5",
	}
	m.handleJobQueued(&jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01",
		CodeFiles: []string{"task_01"},
		Issues:    1,
		SafeName:  "task_01-safe",
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
		OutBuffer: runshared.NewLineBuffer(0),
		ErrBuffer: runshared.NewLineBuffer(0),
	})
	m.handleWindowSize(size)
	m.handleJobUpdate(jobUpdateMsg{
		Index: 0,
		Snapshot: buildSnapshotWithEntries(t,
			TranscriptEntry{
				ID:     "assistant-1",
				Kind:   transcriptEntryAssistantMessage,
				Title:  "Assistant",
				Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "hello from ACP"})},
			},
			TranscriptEntry{
				ID:            "tool-1",
				Kind:          transcriptEntryToolCall,
				Title:         "read_file",
				ToolCallID:    "tool-1",
				ToolCallState: model.ToolCallStateCompleted,
				Blocks: []model.ContentBlock{
					mustContentBlockUITest(t, model.ToolUseBlock{ID: "tool-1", Name: "read_file"}),
					mustContentBlockUITest(t, model.ToolResultBlock{ToolUseID: "tool-1", Content: "loaded README.md"}),
				},
			},
		),
	})
	return m
}

func buildSnapshotWithEntries(t *testing.T, entries ...TranscriptEntry) SessionViewSnapshot {
	t.Helper()
	return SessionViewSnapshot{
		Entries: entries,
		Plan: SessionPlanState{
			Entries: []model.SessionPlanEntry{{
				Content:  "Ship redesign",
				Priority: "high",
				Status:   "in_progress",
			}},
			RunningCount: 1,
		},
		Session: SessionMetaState{
			CurrentModeID: "review",
			AvailableCommands: []model.SessionAvailableCommand{{
				Name:         "run",
				Description:  "Run the task",
				ArgumentHint: "--fast",
			}},
			Status: model.StatusRunning,
		},
	}
}

func mustContentBlockUITest(t *testing.T, payload any) model.ContentBlock {
	t.Helper()

	block, err := model.NewContentBlock(payload)
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}
	return block
}

func keyText(text string) tea.KeyPressMsg {
	r, _ := utf8.DecodeRuneInString(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: r})
}

type testScrollableViewport struct {
	pageUpCalls     int
	pageDownCalls   int
	gotoTopCalls    int
	gotoBottomCalls int
}

func (v *testScrollableViewport) PageUp() {
	v.pageUpCalls++
}

func (v *testScrollableViewport) PageDown() {
	v.pageDownCalls++
}

func (v *testScrollableViewport) GotoTop() []string {
	v.gotoTopCalls++
	return nil
}

func (v *testScrollableViewport) GotoBottom() []string {
	v.gotoBottomCalls++
	return nil
}

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
