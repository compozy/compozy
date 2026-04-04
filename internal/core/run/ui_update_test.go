package run

import (
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/core/model"

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

func TestHandleTickRefreshesSidebarWhileJobRunning(t *testing.T) {
	t.Parallel()

	m := newTestUIModelWithSnapshot(t, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.jobs[0].state = jobRunning
	m.jobs[0].startedAt = time.Now().Add(-65 * time.Second)
	m.refreshSidebarContent()
	before := m.sidebarViewport.View()

	m.handleTick()
	after := m.sidebarViewport.View()

	if before == after {
		t.Fatalf("expected running sidebar content to refresh on tick, got %q", after)
	}
}

func newTestUIModelWithSnapshot(t *testing.T, size tea.WindowSizeMsg) *uiModel {
	t.Helper()

	m := newUIModel(1)
	m.handleJobQueued(&jobQueuedMsg{
		Index:     0,
		CodeFile:  "task_01",
		CodeFiles: []string{"task_01"},
		Issues:    1,
		SafeName:  "task_01-safe",
		OutLog:    "task_01.out.log",
		ErrLog:    "task_01.err.log",
		OutBuffer: newLineBuffer(0),
		ErrBuffer: newLineBuffer(0),
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

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
