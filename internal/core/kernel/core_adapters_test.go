package kernel

import (
	"context"
	"errors"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
)

func TestDispatchRunAdapterBuildsRunStartCommand(t *testing.T) {
	dispatcher := NewDispatcher()
	wantErr := errors.New("dispatch run failed")
	handler := &coreAdapterRunStartCaptureHandler{err: wantErr}
	Register(dispatcher, handler)

	previous := coreAdapterDispatcherFn
	coreAdapterDispatcherFn = func() *Dispatcher { return dispatcher }
	t.Cleanup(func() {
		coreAdapterDispatcherFn = previous
	})

	err := dispatchRunAdapter(context.Background(), core.Config{
		WorkspaceRoot:          "/workspace",
		Name:                   "demo",
		TasksDir:               "/workspace/.compozy/tasks/demo",
		Mode:                   core.ModePRReview,
		IDE:                    core.IDECodex,
		Model:                  "gpt-5.4",
		ReviewsDir:             "/workspace/.compozy/tasks/demo/reviews-001",
		IncludeResolved:        true,
		Timeout:                time.Minute,
		MaxRetries:             1,
		RetryBackoffMultiplier: 1.5,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("dispatchRunAdapter error = %v, want %v", err, wantErr)
	}
	if !handler.called {
		t.Fatal("expected run adapter to dispatch")
	}
	if handler.got.Mode != model.ExecutionModePRReview {
		t.Fatalf("unexpected mode: %q", handler.got.Mode)
	}
	if handler.got.ReviewsDir != "/workspace/.compozy/tasks/demo/reviews-001" {
		t.Fatalf("unexpected reviews dir: %q", handler.got.ReviewsDir)
	}
}

func TestDispatchPrepareAdapterBuildsWorkflowPrepareCommand(t *testing.T) {
	dispatcher := NewDispatcher()
	wantPrep := &core.Preparation{ResolvedName: "demo"}
	handler := &workflowPrepareCaptureHandler{
		result: commands.WorkflowPrepareResult{Preparation: wantPrep},
	}
	Register(dispatcher, handler)

	previous := coreAdapterDispatcherFn
	coreAdapterDispatcherFn = func() *Dispatcher { return dispatcher }
	t.Cleanup(func() {
		coreAdapterDispatcherFn = previous
	})

	prep, err := dispatchPrepareAdapter(context.Background(), core.Config{
		WorkspaceRoot:    "/workspace",
		Name:             "demo",
		TasksDir:         "/workspace/.compozy/tasks/demo",
		Mode:             core.ModePRDTasks,
		IDE:              core.IDECodex,
		IncludeCompleted: true,
		Timeout:          time.Minute,
	})
	if err != nil {
		t.Fatalf("dispatchPrepareAdapter: %v", err)
	}
	if prep != wantPrep {
		t.Fatalf("unexpected preparation pointer: %#v", prep)
	}
	if handler.got.Name != "demo" {
		t.Fatalf("unexpected workflow name: %q", handler.got.Name)
	}
	if handler.got.Mode != model.ExecutionModePRDTasks {
		t.Fatalf("unexpected mode: %q", handler.got.Mode)
	}
	if !handler.got.IncludeCompleted {
		t.Fatal("expected include completed to pass through")
	}
}

type coreAdapterRunStartCaptureHandler struct {
	got    commands.RunStartCommand
	result commands.RunStartResult
	err    error
	called bool
}

func (h *coreAdapterRunStartCaptureHandler) Handle(
	_ context.Context,
	cmd commands.RunStartCommand,
) (commands.RunStartResult, error) {
	h.called = true
	h.got = cmd
	return h.result, h.err
}

type workflowPrepareCaptureHandler struct {
	got    commands.WorkflowPrepareCommand
	result commands.WorkflowPrepareResult
	err    error
}

func (h *workflowPrepareCaptureHandler) Handle(
	_ context.Context,
	cmd commands.WorkflowPrepareCommand,
) (commands.WorkflowPrepareResult, error) {
	h.got = cmd
	return h.result, h.err
}
