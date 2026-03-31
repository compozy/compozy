package run

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUIModeNavigateSelectsJobAndEntersTerminalMode(t *testing.T) {
	t.Parallel()

	model := newTestUIModel(2)
	model.jobs = []uiJob{
		{safeName: "job-1", state: jobPending},
		{safeName: "job-2", state: jobPending},
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := next.(*uiModel)
	if updated.selectedJob != 1 {
		t.Fatalf("selectedJob = %d, want 1", updated.selectedJob)
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated = next.(*uiModel)
	if updated.mode != modeTerminal {
		t.Fatalf("mode = %v, want %v", updated.mode, modeTerminal)
	}
}

func TestUIModeTerminalEscReturnsToNavigateMode(t *testing.T) {
	t.Parallel()

	model := newTestUIModel(1)
	model.jobs = []uiJob{{safeName: "job-1", state: jobRunning}}
	model.mode = modeTerminal

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(*uiModel)
	if updated.mode != modeNavigate {
		t.Fatalf("mode = %v, want %v", updated.mode, modeNavigate)
	}
}

func TestUIModeTerminalForwardsKeysToSelectedTerminal(t *testing.T) {
	t.Parallel()

	term := newStartedTerminal(t, "echo-loop")
	model := newTestUIModel(1)
	model.jobs = []uiJob{{safeName: "job-1", state: jobRunning}}
	model.terminals[0] = term
	model.mode = modeTerminal

	keyEvents := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("h")},
		{Type: tea.KeyRunes, Runes: []rune("i")},
		{Type: tea.KeyEnter},
	}
	for _, keyEvent := range keyEvents {
		next, _ := model.Update(keyEvent)
		model = next.(*uiModel)
	}

	waitForRenderContains(t, term, "echo:hi")
}

func TestTerminalOutputMsgRefreshesRenderedTerminalView(t *testing.T) {
	t.Parallel()

	term := NewTerminal(80, 24, "job-1")
	mustWriteSafeEmulator(t, term.emu, []byte("Claude Code\n> hello"))

	model := newTestUIModel(1)
	model.jobs = []uiJob{{safeName: "job-1", state: jobRunning}}
	model.terminals[0] = term

	next, _ := model.Update(terminalOutputMsg{Index: 0, Data: []byte("hello")})
	updated := next.(*uiModel)
	view := updated.View()
	if !strings.Contains(view, "hello") {
		t.Fatalf("View() = %q, want terminal output", view)
	}
}

func TestJobDoneSignalMarksCompletedSelectsNextPendingAndKeepsTerminalAccessible(t *testing.T) {
	t.Parallel()

	first := NewTerminal(80, 24, "job-1")
	second := NewTerminal(80, 24, "job-2")
	mustWriteSafeEmulator(t, first.emu, []byte("completed terminal"))
	mustWriteSafeEmulator(t, second.emu, []byte("pending terminal"))

	model := newTestUIModel(2)
	model.jobs = []uiJob{
		{safeName: "job-1", state: jobRunning, startedAt: time.Now().Add(-2 * time.Second)},
		{safeName: "job-2", state: jobPending},
	}
	model.terminals[0] = first
	model.terminals[1] = second

	next, _ := model.Update(jobDoneSignalMsg{JobID: "job-1"})
	updated := next.(*uiModel)
	if updated.jobs[0].state != jobSuccess {
		t.Fatalf("job[0] state = %v, want %v", updated.jobs[0].state, jobSuccess)
	}
	if updated.completed != 1 {
		t.Fatalf("completed = %d, want 1", updated.completed)
	}
	if updated.selectedJob != 1 {
		t.Fatalf("selectedJob = %d, want 1", updated.selectedJob)
	}

	updated.selectedJob = 0
	view := updated.View()
	if !strings.Contains(view, "completed terminal") {
		t.Fatalf("View() = %q, want completed terminal screen", view)
	}
}

func TestUISummaryAndFailureViewsHandleNavigationKeys(t *testing.T) {
	t.Parallel()

	model := newTestUIModel(1)
	model.jobs = []uiJob{{safeName: "job-1", state: jobSuccess}}
	model.completed = 1
	model.total = 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	updated := next.(*uiModel)
	if updated.currentView != uiViewSummary {
		t.Fatalf("currentView = %v, want %v", updated.currentView, uiViewSummary)
	}
	if !strings.Contains(updated.View(), "All Jobs Complete") {
		t.Fatalf("summary view = %q, want completion text", updated.View())
	}

	updated.failures = []failInfo{
		{codeFile: "internal/app/service.go", exitCode: 7, outLog: "out.log", errLog: "err.log"},
	}
	updated.currentView = uiViewFailures
	if !strings.Contains(updated.View(), "Failure Details") {
		t.Fatalf("failures view = %q, want failure details", updated.View())
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated = next.(*uiModel)
	if updated.currentView != uiViewJobs {
		t.Fatalf("currentView after esc = %v, want %v", updated.currentView, uiViewJobs)
	}
}

func TestUILayoutHelpersClampDimensionsAndTruncateText(t *testing.T) {
	t.Parallel()

	model := newTestUIModel(1)

	sidebar, main := model.computePaneWidths(40)
	if sidebar <= 0 || main <= 0 {
		t.Fatalf("pane widths = (%d, %d), want positive values", sidebar, main)
	}

	if got := model.computeContentHeight(1); got != minContentHeight {
		t.Fatalf("computeContentHeight(1) = %d, want %d", got, minContentHeight)
	}

	if got := truncateString("abcdef", 4); got != "abc…" {
		t.Fatalf("truncateString() = %q, want %q", got, "abc…")
	}

	model.mainWidth = 0
	model.width = 80
	mainWidth, contentHeight := model.mainDimensions()
	if mainWidth < mainMinWidth {
		t.Fatalf("mainWidth = %d, want >= %d", mainWidth, mainMinWidth)
	}
	if contentHeight < minContentHeight {
		t.Fatalf("contentHeight = %d, want >= %d", contentHeight, minContentHeight)
	}
}

func newTestUIModel(total int) *uiModel {
	model := newUIModel(context.Background(), total, make([]*Terminal, total), nil)
	model.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 40})
	return model
}
