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

	useCoreAdapterDispatcher(t, dispatcher)

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
	if got := err.Error(); got != "run: dispatch run failed" {
		t.Fatalf("dispatchRunAdapter error = %q, want wrapped context", got)
	}
	if !handler.called {
		t.Fatal("expected run adapter to dispatch")
	}
	runtimeCfg := handler.got.RuntimeConfig()
	if runtimeCfg.Mode != model.ExecutionModePRReview {
		t.Fatalf("unexpected mode: %q", runtimeCfg.Mode)
	}
	if runtimeCfg.ReviewsDir != "/workspace/.compozy/tasks/demo/reviews-001" {
		t.Fatalf("unexpected reviews dir: %q", runtimeCfg.ReviewsDir)
	}
}

func TestDispatchPrepareAdapterBuildsWorkflowPrepareCommand(t *testing.T) {
	dispatcher := NewDispatcher()
	wantPrep := &core.Preparation{ResolvedName: "demo"}
	handler := &workflowPrepareCaptureHandler{
		result: commands.WorkflowPrepareResult{Preparation: wantPrep},
	}
	Register(dispatcher, handler)

	useCoreAdapterDispatcher(t, dispatcher)

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
	runtimeCfg := handler.got.RuntimeConfig()
	if runtimeCfg.Name != "demo" {
		t.Fatalf("unexpected workflow name: %q", runtimeCfg.Name)
	}
	if runtimeCfg.Mode != model.ExecutionModePRDTasks {
		t.Fatalf("unexpected mode: %q", runtimeCfg.Mode)
	}
	if !runtimeCfg.IncludeCompleted {
		t.Fatal("expected include completed to pass through")
	}
}

func TestDispatchCoreWorkflowAdaptersBuildTypedCommands(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, dispatcher *Dispatcher)
	}{
		{
			name: "Should build reviews fetch commands",
			run: func(t *testing.T, dispatcher *Dispatcher) {
				t.Helper()

				handler := &reviewsFetchCaptureHandler{
					result: commands.ReviewsFetchResult{Result: &model.FetchResult{Name: "demo", Round: 3}},
				}
				Register(dispatcher, handler)

				result, err := dispatchFetchReviewsAdapter(context.Background(), core.Config{
					WorkspaceRoot: "/workspace",
					Name:          "demo",
					Round:         3,
					Provider:      "coderabbit",
					PR:            "259",
				})
				if err != nil {
					t.Fatalf("dispatchFetchReviewsAdapter: %v", err)
				}
				if result == nil || result.Round != 3 {
					t.Fatalf("unexpected fetch result: %#v", result)
				}
				if handler.got.Name != "demo" || handler.got.Provider != "coderabbit" || handler.got.PR != "259" {
					t.Fatalf("unexpected fetch command: %#v", handler.got)
				}
			},
		},
		{
			name: "Should build workspace migrate commands",
			run: func(t *testing.T, dispatcher *Dispatcher) {
				t.Helper()

				handler := &workspaceMigrateCaptureHandler{
					result: commands.WorkspaceMigrateResult{
						Result: &model.MigrationResult{Target: "/workspace/.compozy/tasks/demo"},
					},
				}
				Register(dispatcher, handler)

				result, err := dispatchMigrateAdapter(context.Background(), core.MigrationConfig{
					WorkspaceRoot: "/workspace",
					RootDir:       "/workspace/.compozy/tasks",
					Name:          "demo",
					TasksDir:      "/workspace/.compozy/tasks/demo",
					ReviewsDir:    "/workspace/.compozy/tasks/demo/reviews-001",
					DryRun:        true,
				})
				if err != nil {
					t.Fatalf("dispatchMigrateAdapter: %v", err)
				}
				if result == nil || result.Target == "" {
					t.Fatalf("unexpected migrate result: %#v", result)
				}
				if handler.got.RootDir != "/workspace/.compozy/tasks" || handler.got.ReviewsDir == "" ||
					!handler.got.DryRun {
					t.Fatalf("unexpected migrate command: %#v", handler.got)
				}
			},
		},
		{
			name: "Should build workflow sync commands",
			run: func(t *testing.T, dispatcher *Dispatcher) {
				t.Helper()

				handler := &workflowSyncCaptureHandler{
					result: commands.WorkflowSyncResult{
						Result: &model.SyncResult{Target: "/workspace/.compozy/tasks/demo"},
					},
				}
				Register(dispatcher, handler)

				result, err := dispatchSyncAdapter(context.Background(), core.SyncConfig{
					WorkspaceRoot: "/workspace",
					RootDir:       "/workspace/.compozy/tasks",
					Name:          "demo",
					TasksDir:      "/workspace/.compozy/tasks/demo",
				})
				if err != nil {
					t.Fatalf("dispatchSyncAdapter: %v", err)
				}
				if result == nil || result.Target == "" {
					t.Fatalf("unexpected sync result: %#v", result)
				}
				if handler.got.RootDir != "/workspace/.compozy/tasks" || handler.got.TasksDir == "" {
					t.Fatalf("unexpected sync command: %#v", handler.got)
				}
			},
		},
		{
			name: "Should build workflow archive commands",
			run: func(t *testing.T, dispatcher *Dispatcher) {
				t.Helper()

				handler := &workflowArchiveCaptureHandler{
					result: commands.WorkflowArchiveResult{
						Result: &model.ArchiveResult{Target: "/workspace/.compozy/tasks/demo"},
					},
				}
				Register(dispatcher, handler)

				result, err := dispatchArchiveAdapter(context.Background(), core.ArchiveConfig{
					WorkspaceRoot: "/workspace",
					RootDir:       "/workspace/.compozy/tasks",
					Name:          "demo",
					TasksDir:      "/workspace/.compozy/tasks/demo",
				})
				if err != nil {
					t.Fatalf("dispatchArchiveAdapter: %v", err)
				}
				if result == nil || result.Target == "" {
					t.Fatalf("unexpected archive result: %#v", result)
				}
				if handler.got.RootDir != "/workspace/.compozy/tasks" || handler.got.TasksDir == "" {
					t.Fatalf("unexpected archive command: %#v", handler.got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher := NewDispatcher()
			useCoreAdapterDispatcher(t, dispatcher)
			tc.run(t, dispatcher)
		})
	}
}

func useCoreAdapterDispatcher(t *testing.T, dispatcher *Dispatcher) {
	t.Helper()

	previous := coreAdapterDispatcherFn
	coreAdapterDispatcherFn = func() (*Dispatcher, error) { return dispatcher, nil }
	t.Cleanup(func() {
		coreAdapterDispatcherFn = previous
	})
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

type reviewsFetchCaptureHandler struct {
	got    commands.ReviewsFetchCommand
	result commands.ReviewsFetchResult
	err    error
}

func (h *reviewsFetchCaptureHandler) Handle(
	_ context.Context,
	cmd commands.ReviewsFetchCommand,
) (commands.ReviewsFetchResult, error) {
	h.got = cmd
	return h.result, h.err
}

type workspaceMigrateCaptureHandler struct {
	got    commands.WorkspaceMigrateCommand
	result commands.WorkspaceMigrateResult
	err    error
}

func (h *workspaceMigrateCaptureHandler) Handle(
	_ context.Context,
	cmd commands.WorkspaceMigrateCommand,
) (commands.WorkspaceMigrateResult, error) {
	h.got = cmd
	return h.result, h.err
}

type workflowSyncCaptureHandler struct {
	got    commands.WorkflowSyncCommand
	result commands.WorkflowSyncResult
	err    error
}

func (h *workflowSyncCaptureHandler) Handle(
	_ context.Context,
	cmd commands.WorkflowSyncCommand,
) (commands.WorkflowSyncResult, error) {
	h.got = cmd
	return h.result, h.err
}

type workflowArchiveCaptureHandler struct {
	got    commands.WorkflowArchiveCommand
	result commands.WorkflowArchiveResult
	err    error
}

func (h *workflowArchiveCaptureHandler) Handle(
	_ context.Context,
	cmd commands.WorkflowArchiveCommand,
) (commands.WorkflowArchiveResult, error) {
	h.got = cmd
	return h.result, h.err
}
