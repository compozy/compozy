package run

import (
	"fmt"
	"testing"
	"unicode/utf8"

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

	m, buffers := newScrollableUIModelForTest(t, 1, 80)
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
		buffers[0].appendLine(fmt.Sprintf("job00 appended %03d", i))
	}
	m.handleJobLogUpdate(jobLogUpdateMsg{Index: 0})
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
		buffers[0].appendLine(fmt.Sprintf("job00 appended tail %03d", i))
	}
	m.handleJobLogUpdate(jobLogUpdateMsg{Index: 0})
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
		for lineIndex := 0; lineIndex < linesPerJob; lineIndex++ {
			outBuffer.appendLine(fmt.Sprintf("job%02d line %03d", jobIndex, lineIndex))
		}
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
	}
	m.handleWindowSize(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m, buffers
}

func keyText(text string) tea.KeyPressMsg {
	r, _ := utf8.DecodeRuneInString(text)
	return tea.KeyPressMsg(tea.Key{Text: text, Code: r})
}

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
