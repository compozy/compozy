package run

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/core/model"

	tea "charm.land/bubbletea/v2"
)

func TestHandleKeyRequestsShutdownWithoutQuittingWhileRunActive(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 1, 40)
	quitCalls := 0
	m.onQuit = func() {
		quitCalls++
	}

	cmd := m.handleKey(keyText("q"))
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatal("expected active run to request shutdown without quitting the UI")
		}
	}
	if quitCalls != 1 {
		t.Fatalf("expected shutdown callback to be invoked once, got %d", quitCalls)
	}
}

func TestHandleKeyQuitsOnceRunCompletes(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 1, 40)
	quitCalls := 0
	m.onQuit = func() {
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

func TestHandleUsageUpdateAggregatesUsage(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 1, 10)
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

func TestHandleJobUpdateStoresBlocksAndRefreshesViewport(t *testing.T) {
	t.Parallel()

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
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})

	block, err := model.NewContentBlock(model.TextBlock{Text: "hello from typed blocks"})
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}

	m.handleJobUpdate(jobUpdateMsg{Index: 0, Blocks: []model.ContentBlock{block}})

	if got := len(m.jobs[0].blocks); got != 1 {
		t.Fatalf("expected 1 stored block, got %d", got)
	}
	content, hasContent := m.buildViewportContent(&m.jobs[0], 120)
	if !hasContent {
		t.Fatal("expected viewport content after job update")
	}
	if !strings.Contains(content, "hello from typed blocks") {
		t.Fatalf("expected rendered viewport content, got %q", content)
	}
}

func TestWaitEventAndTickProduceMessages(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	events := make(chan uiMsg, 1)
	m.setEventSource(events)
	events <- jobStartedMsg{Index: 0}

	waitCmd := m.waitEvent()
	if waitCmd == nil {
		t.Fatal("expected waitEvent to return a command")
	}
	if _, ok := waitCmd().(jobStartedMsg); !ok {
		t.Fatalf("expected waitEvent command to yield jobStartedMsg, got %T", waitCmd())
	}

	tickCmd := m.tick()
	if tickCmd == nil {
		t.Fatal("expected tick to return a command")
	}
	if _, ok := tickCmd().(tickMsg); !ok {
		t.Fatalf("expected tick command to yield tickMsg, got %T", tickCmd())
	}
}

func TestHandleViewSwitchKeysTogglesJobsAndSummary(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 1, 10)
	m.completed = 1

	m.handleViewSwitchKeys("s")
	if got := m.currentView; got != uiViewSummary {
		t.Fatalf("expected summary view, got %s", got)
	}

	m.handleViewSwitchKeys("esc")
	if got := m.currentView; got != uiViewJobs {
		t.Fatalf("expected jobs view, got %s", got)
	}
}

func TestHandleTickStopsAfterCompletion(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 1, 10)
	cmd := m.handleTick()
	if cmd == nil {
		t.Fatal("expected active run tick command")
	}

	m.completed = 1
	if cmd := m.handleTick(); cmd != nil {
		t.Fatalf("expected completed run tick to stop, got %T", cmd)
	}
}

func TestUpdateRoutesAdditionalMessageTypes(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	m.setEventSource(make(chan uiMsg, 8))

	textBlock, err := model.NewContentBlock(model.TextBlock{Text: "hello"})
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}

	msgs := []tea.Msg{
		jobQueuedMsg{
			Index:     0,
			CodeFile:  "task_01",
			CodeFiles: []string{"task_01"},
			Issues:    1,
			SafeName:  "task_01-safe",
			OutLog:    "task_01.out.log",
			ErrLog:    "task_01.err.log",
			OutBuffer: newLineBuffer(0),
			ErrBuffer: newLineBuffer(0),
		},
		jobStartedMsg{Index: 0},
		jobUpdateMsg{Index: 0, Blocks: []model.ContentBlock{textBlock}},
		usageUpdateMsg{Index: 0, Usage: model.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}},
		jobFinishedMsg{Index: 0, Success: true},
		jobFailureMsg{Failure: failInfo{codeFile: "task_01", exitCode: 1, err: fmt.Errorf("boom")}},
		drainMsg{},
	}

	for _, msg := range msgs {
		if _, cmd := m.Update(msg); msg == nil && cmd != nil {
			t.Fatalf("unexpected command for nil message")
		}
	}
	if len(m.jobs) != 1 || m.jobs[0].state != jobSuccess {
		t.Fatalf("expected routed updates to leave one successful job, got %#v", m.jobs)
	}
	if len(m.failures) != 1 {
		t.Fatalf("expected routed failure message, got %#v", m.failures)
	}
}

func TestNavigationRestoresPerJobViewportState(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 2, 80)
	initialBottom := m.viewport.YOffset()
	if initialBottom == 0 {
		t.Fatal("expected initial viewport to be scrollable")
	}

	m.handleKey(keyCode(tea.KeyPgUp))
	scrolledOffset := m.viewport.YOffset()
	if scrolledOffset >= initialBottom {
		t.Fatalf("expected pgup to move viewport up, before=%d after=%d", initialBottom, scrolledOffset)
	}
	if m.selectedJob != 0 {
		t.Fatalf("expected pgup to keep selected job 0, got %d", m.selectedJob)
	}

	m.handleKey(keyCode(tea.KeyDown))
	if m.selectedJob != 1 {
		t.Fatalf("expected down to move selection to job 1, got %d", m.selectedJob)
	}

	m.handleKey(keyCode(tea.KeyUp))
	if m.selectedJob != 0 {
		t.Fatalf("expected up to restore selection to job 0, got %d", m.selectedJob)
	}
	if got := m.viewport.YOffset(); got != scrolledOffset {
		t.Fatalf("expected job 0 viewport offset %d to be restored, got %d", scrolledOffset, got)
	}
}

func TestMouseWheelScrollsActiveLogWithoutChangingSelection(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 2, 80)
	initialBottom := m.viewport.YOffset()
	if initialBottom == 0 {
		t.Fatal("expected initial viewport to be scrollable")
	}

	m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if m.selectedJob != 0 {
		t.Fatalf("expected mouse wheel to keep selection on job 0, got %d", m.selectedJob)
	}
	if got := m.viewport.YOffset(); got >= initialBottom {
		t.Fatalf("expected mouse wheel up to scroll log upward, before=%d after=%d", initialBottom, got)
	}
}

func TestFollowTailStopsOnManualScrollAndResumesAtBottom(t *testing.T) {
	t.Parallel()

	m, _ := newScrollableUIModelForTest(t, 1, 80)
	initialBottom := m.viewport.YOffset()
	if initialBottom == 0 {
		t.Fatal("expected initial viewport to be scrollable")
	}

	m.handleKey(keyCode(tea.KeyPgUp))
	scrolledOffset := m.viewport.YOffset()
	if scrolledOffset >= initialBottom {
		t.Fatalf("expected pgup to move viewport up, before=%d after=%d", initialBottom, scrolledOffset)
	}
	if m.jobs[0].followTail {
		t.Fatal("expected manual scroll to disable follow-tail")
	}

	for i := 0; i < 8; i++ {
		appendJobTextUpdate(t, m, 0, fmt.Sprintf("job00 appended %03d", i))
	}
	if got := m.viewport.YOffset(); got != scrolledOffset {
		t.Fatalf("expected manual offset %d to persist while follow-tail is disabled, got %d", scrolledOffset, got)
	}
	if m.jobs[0].followTail {
		t.Fatal("expected follow-tail to remain disabled after new output")
	}

	m.handleKey(keyCode(tea.KeyEnd))
	resumedBottom := m.viewport.YOffset()
	if !m.jobs[0].followTail {
		t.Fatal("expected end to re-enable follow-tail")
	}
	if resumedBottom <= scrolledOffset {
		t.Fatalf("expected end to move viewport to bottom, before=%d after=%d", scrolledOffset, resumedBottom)
	}

	for i := 0; i < 8; i++ {
		appendJobTextUpdate(t, m, 0, fmt.Sprintf("job00 appended tail %03d", i))
	}
	if got := m.viewport.YOffset(); got <= resumedBottom {
		t.Fatalf("expected follow-tail to advance viewport after new output, before=%d after=%d", resumedBottom, got)
	}
	if !m.jobs[0].followTail {
		t.Fatal("expected follow-tail to remain enabled at the bottom")
	}
}

func newScrollableUIModelForTest(t *testing.T, jobCount, linesPerJob int) (*uiModel, []*lineBuffer) {
	t.Helper()

	m := newUIModel(jobCount)
	buffers := make([]*lineBuffer, 0, jobCount)
	for jobIndex := 0; jobIndex < jobCount; jobIndex++ {
		outBuffer := newLineBuffer(0)
		errBuffer := newLineBuffer(0)
		buffers = append(buffers, outBuffer)
		m.handleJobQueued(&jobQueuedMsg{
			Index:     jobIndex,
			CodeFile:  fmt.Sprintf("task_%02d", jobIndex),
			CodeFiles: []string{fmt.Sprintf("task_%02d", jobIndex)},
			Issues:    1,
			SafeName:  fmt.Sprintf("task_%02d-safe", jobIndex),
			OutLog:    fmt.Sprintf("task_%02d.log", jobIndex),
			ErrLog:    fmt.Sprintf("task_%02d.err.log", jobIndex),
			OutBuffer: outBuffer,
			ErrBuffer: errBuffer,
		})
		for lineIndex := 0; lineIndex < linesPerJob; lineIndex++ {
			appendJobTextUpdate(t, m, jobIndex, fmt.Sprintf("job%02d line %03d", jobIndex, lineIndex))
		}
	}
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m, buffers
}

func appendJobTextUpdate(t *testing.T, m *uiModel, index int, text string) {
	t.Helper()

	block, err := model.NewContentBlock(model.TextBlock{Text: text})
	if err != nil {
		t.Fatalf("new content block: %v", err)
	}
	m.handleJobUpdate(jobUpdateMsg{Index: index, Blocks: []model.ContentBlock{block}})
}

func keyText(text string) tea.KeyPressMsg {
	r, _ := utf8.DecodeRuneInString(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: r})
}

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
