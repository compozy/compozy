package run

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestUIModelInitReturnsCommands(t *testing.T) {
	t.Parallel()

	m := newUIModel(1)
	events := make(chan uiMsg, 1)
	m.setEventSource(events)

	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected Init to return a command")
	}
}

func TestUIControllerHelpers(t *testing.T) {
	t.Parallel()

	done := make(chan error)
	close(done)

	ctrl := &uiController{
		ch:   make(chan uiMsg, 1),
		done: done,
	}
	if got := ctrl.events(); got != ctrl.ch {
		t.Fatal("expected events to expose controller channel")
	}

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

	if ui := setupUI(context.Background(), nil, nil, false); ui != nil {
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

	m := newUIModel(1)
	running := &uiJob{state: jobRunning, startedAt: time.Now().Add(-2 * time.Minute)}
	success := &uiJob{state: jobSuccess, duration: 42 * time.Second}
	failed := &uiJob{state: jobFailed, duration: 15 * time.Second}

	for _, tc := range []struct {
		state jobState
		label string
	}{
		{jobPending, "PENDING"},
		{jobRunning, "RUNNING"},
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
