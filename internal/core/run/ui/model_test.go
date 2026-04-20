package ui

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestUIModelInitReturnsCommands(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)

	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected Init to return a command")
	}
}

func TestUIControllerHelpers(t *testing.T) {
	t.Parallel()

	done := make(chan error)
	close(done)
	dispatchDone := make(chan struct{})
	close(dispatchDone)
	dispatchCtx, cancelDispatch := context.WithCancel(context.Background())
	cancelDispatch()

	ctrl := &uiController{
		done:           done,
		dispatchDone:   dispatchDone,
		dispatchCtx:    dispatchCtx,
		cancelDispatch: cancelDispatch,
	}
	ctrl.enqueue(jobStartedMsg{Index: 0})
	if got := len(ctrl.pendingInputs); got != 0 {
		t.Fatalf("expected canceled controller to reject pending inputs, got %d", got)
	}

	dispatchCtx, cancelDispatch = context.WithCancel(context.Background())
	ctrl.dispatchCtx = dispatchCtx
	ctrl.cancelDispatch = cancelDispatch
	ctrl.enqueue(jobStartedMsg{Index: 0})
	if got := len(ctrl.pendingInputs); got != 1 {
		t.Fatalf("expected one pending input, got %d", got)
	}
	if got, ok := ctrl.pendingInputs[0].(jobStartedMsg); !ok || got.Index != 0 {
		t.Fatalf("unexpected pending input: %#v", ctrl.pendingInputs[0])
	}
	cancelDispatch()

	called := 0
	ctrl.setQuitHandler(func(uiQuitRequest) {
		called++
	})
	ctrl.requestQuit(uiQuitRequestDrain)
	if called != 1 {
		t.Fatalf("expected quit handler to be invoked once, got %d", called)
	}

	ctrl.closeEvents()
	ctrl.shutdown()
	if err := ctrl.wait(); err != nil {
		t.Fatalf("unexpected wait error: %v", err)
	}
}

func TestSetupUIDisabledReturnsNil(t *testing.T) {
	t.Parallel()

	if ui := setupUI(context.Background(), nil, nil, nil, false); ui != nil {
		t.Fatalf("expected disabled setupUI to return nil, got %T", ui)
	}
}

func TestFormattingAndStateHelpersCoverBranches(t *testing.T) {
	t.Parallel()

	if got := formatNumber(12345); got != "12,345" {
		t.Fatalf("expected formatted number, got %q", got)
	}
	if got := formatDuration(2*time.Hour + 3*time.Minute + 4*time.Second); got != "02:03:04" {
		t.Fatalf("unexpected long duration format %q", got)
	}
	if got := formatDuration(90*time.Second + 950*time.Millisecond); got != "01:30" {
		t.Fatalf("expected duration formatting to truncate fractional seconds, got %q", got)
	}

	m := newUIModel(1)
	running := &uiJob{state: jobRunning, startedAt: time.Now().Add(-2 * time.Minute)}
	retrying := &uiJob{state: jobRetrying, attempt: 2, maxAttempts: 3}
	success := &uiJob{state: jobSuccess, duration: 42 * time.Second}
	failed := &uiJob{state: jobFailed, duration: 15 * time.Second}

	for _, tc := range []struct {
		state jobState
		label string
	}{
		{jobPending, "PENDING"},
		{jobRunning, "RUNNING"},
		{jobRetrying, "RETRY"},
		{jobSuccess, "SUCCESS"},
		{jobFailed, "FAILED"},
	} {
		if got := m.getStateLabel(tc.state); got != tc.label {
			t.Fatalf("unexpected state label for %v: %q", tc.state, got)
		}
		if got := m.jobStateIcon(tc.state); got == "" {
			t.Fatalf("expected icon for state %v", tc.state)
		}
		if m.jobStateColor(tc.state) == nil {
			t.Fatalf("expected color for state %v", tc.state)
		}
		if m.jobBorderColor(&uiJob{state: tc.state}) == nil {
			t.Fatalf("expected border color for state %v", tc.state)
		}
	}

	for _, rendered := range []string{
		m.elapsedStr(running, colorBgBase),
		m.elapsedStr(retrying, colorBgBase),
		m.elapsedStr(success, colorBgBase),
		m.elapsedStr(failed, colorBgBase),
	} {
		if rendered == "" {
			t.Fatal("expected elapsedStr to render for running/success/failed states")
		}
	}

	m.jobs = []uiJob{{tokenUsage: &model.Usage{InputTokens: 1, OutputTokens: 2}}}
	m.total = 1
	m.currentView = uiViewSummary
	if view := m.View().Content; view == "" {
		t.Fatal("expected summary view content")
	}
}
