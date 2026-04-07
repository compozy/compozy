package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
)

type runStartCaptureHandler struct {
	got    commands.RunStartCommand
	result commands.RunStartResult
	err    error
	called bool
}

func (h *runStartCaptureHandler) Handle(
	_ context.Context,
	cmd commands.RunStartCommand,
) (commands.RunStartResult, error) {
	h.called = true
	h.got = cmd
	return h.result, h.err
}

type reviewsFetchCaptureHandler struct {
	got    commands.ReviewsFetchCommand
	result commands.ReviewsFetchResult
	err    error
	called bool
}

func (h *reviewsFetchCaptureHandler) Handle(
	_ context.Context,
	cmd commands.ReviewsFetchCommand,
) (commands.ReviewsFetchResult, error) {
	h.called = true
	h.got = cmd
	return h.result, h.err
}

type migrateCaptureHandler struct {
	got    commands.WorkspaceMigrateCommand
	result commands.WorkspaceMigrateResult
	err    error
	called bool
}

func (h *migrateCaptureHandler) Handle(
	_ context.Context,
	cmd commands.WorkspaceMigrateCommand,
) (commands.WorkspaceMigrateResult, error) {
	h.called = true
	h.got = cmd
	return h.result, h.err
}

type syncCaptureHandler struct {
	got    commands.WorkflowSyncCommand
	result commands.WorkflowSyncResult
	err    error
	called bool
}

func (h *syncCaptureHandler) Handle(
	_ context.Context,
	cmd commands.WorkflowSyncCommand,
) (commands.WorkflowSyncResult, error) {
	h.called = true
	h.got = cmd
	return h.result, h.err
}

type archiveCaptureHandler struct {
	got    commands.WorkflowArchiveCommand
	result commands.WorkflowArchiveResult
	err    error
	called bool
}

func (h *archiveCaptureHandler) Handle(
	_ context.Context,
	cmd commands.WorkflowArchiveCommand,
) (commands.WorkflowArchiveResult, error) {
	h.called = true
	h.got = cmd
	return h.result, h.err
}

func TestNewRunWorkflowDispatchesStartCommand(t *testing.T) {
	t.Parallel()

	dispatcher := kernel.NewDispatcher()
	wantErr := errors.New("run-start boom")
	handler := &runStartCaptureHandler{err: wantErr}
	kernel.Register(dispatcher, handler)

	runWorkflow := newRunWorkflow(dispatcher)
	err := runWorkflow(context.Background(), core.Config{
		WorkspaceRoot:          "/workspace",
		Name:                   "demo",
		TasksDir:               "/workspace/.compozy/tasks/demo",
		IncludeCompleted:       true,
		Mode:                   core.ModePRDTasks,
		IDE:                    core.IDECodex,
		Model:                  "gpt-5.4",
		Concurrent:             2,
		BatchSize:              1,
		ReasoningEffort:        "high",
		AccessMode:             core.AccessModeFull,
		Timeout:                time.Minute,
		MaxRetries:             1,
		RetryBackoffMultiplier: 2,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWorkflow error = %v, want %v", err, wantErr)
	}
	if !handler.called {
		t.Fatal("expected dispatcher handler to be called")
	}
	runtime := handler.got.RuntimeConfig()
	if runtime == nil {
		t.Fatal("expected runtime config")
	}
	if runtime.WorkspaceRoot != "/workspace" {
		t.Fatalf("unexpected workspace root: %q", runtime.WorkspaceRoot)
	}
	if runtime.Name != "demo" {
		t.Fatalf("unexpected workflow name: %q", runtime.Name)
	}
	if runtime.TasksDir != "/workspace/.compozy/tasks/demo" {
		t.Fatalf("unexpected tasks dir: %q", runtime.TasksDir)
	}
	if !runtime.IncludeCompleted {
		t.Fatal("expected include completed to pass through")
	}
	if runtime.Mode != model.ExecutionModePRDTasks {
		t.Fatalf("unexpected mode: %q", runtime.Mode)
	}
	if runtime.IDE != model.IDECodex {
		t.Fatalf("unexpected ide: %q", runtime.IDE)
	}
	if runtime.Model != "gpt-5.4" {
		t.Fatalf("unexpected model: %q", runtime.Model)
	}
}

func TestNewRunWorkflowUsesPRReviewModeForFixReviews(t *testing.T) {
	t.Parallel()

	dispatcher := kernel.NewDispatcher()
	handler := &runStartCaptureHandler{}
	kernel.Register(dispatcher, handler)

	runWorkflow := newRunWorkflow(dispatcher)
	if err := runWorkflow(context.Background(), core.Config{
		WorkspaceRoot:      "/workspace",
		Name:               "demo",
		ReviewsDir:         "/workspace/.compozy/tasks/demo/reviews-001",
		IncludeResolved:    true,
		BatchSize:          3,
		Concurrent:         2,
		Mode:               core.ModePRReview,
		IDE:                core.IDEClaude,
		ReasoningEffort:    "medium",
		AccessMode:         core.AccessModeDefault,
		Timeout:            time.Minute,
		OutputFormat:       core.OutputFormatText,
		ResolvedPromptText: "",
	}); err != nil {
		t.Fatalf("runWorkflow: %v", err)
	}
	if !handler.called {
		t.Fatal("expected dispatcher handler to be called")
	}
	runtime := handler.got.RuntimeConfig()
	if runtime == nil {
		t.Fatal("expected runtime config")
	}
	if runtime.Mode != model.ExecutionModePRReview {
		t.Fatalf("unexpected mode: %q", runtime.Mode)
	}
	if runtime.ReviewsDir != "/workspace/.compozy/tasks/demo/reviews-001" {
		t.Fatalf("unexpected reviews dir: %q", runtime.ReviewsDir)
	}
	if !runtime.IncludeResolved {
		t.Fatal("expected include resolved to pass through")
	}
}

func TestNewRunWorkflowDispatchesExecPromptSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		cfg                 core.Config
		wantPromptText      string
		wantPromptFile      string
		wantReadPromptStdin bool
		wantResolved        string
	}{
		{
			name: "prompt text",
			cfg: core.Config{
				WorkspaceRoot:      "/workspace",
				Mode:               core.ModeExec,
				IDE:                core.IDECodex,
				OutputFormat:       core.OutputFormatJSON,
				PromptText:         "summarize",
				ResolvedPromptText: "summarize",
			},
			wantPromptText: "summarize",
			wantResolved:   "summarize",
		},
		{
			name: "prompt file",
			cfg: core.Config{
				WorkspaceRoot:      "/workspace",
				Mode:               core.ModeExec,
				IDE:                core.IDECodex,
				OutputFormat:       core.OutputFormatJSON,
				PromptFile:         "prompt.md",
				ResolvedPromptText: "from file",
			},
			wantPromptFile: "prompt.md",
			wantResolved:   "from file",
		},
		{
			name: "stdin",
			cfg: core.Config{
				WorkspaceRoot:      "/workspace",
				Mode:               core.ModeExec,
				IDE:                core.IDECodex,
				OutputFormat:       core.OutputFormatJSON,
				ReadPromptStdin:    true,
				ResolvedPromptText: "from stdin",
			},
			wantReadPromptStdin: true,
			wantResolved:        "from stdin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dispatcher := kernel.NewDispatcher()
			handler := &runStartCaptureHandler{}
			kernel.Register(dispatcher, handler)

			if err := newRunWorkflow(dispatcher)(context.Background(), tt.cfg); err != nil {
				t.Fatalf("runWorkflow: %v", err)
			}
			if !handler.called {
				t.Fatal("expected dispatcher handler to be called")
			}
			runtime := handler.got.RuntimeConfig()
			if runtime == nil {
				t.Fatal("expected runtime config")
			}
			if runtime.Mode != model.ExecutionModeExec {
				t.Fatalf("unexpected mode: %q", runtime.Mode)
			}
			if runtime.PromptText != tt.wantPromptText {
				t.Fatalf("unexpected prompt text: %q", runtime.PromptText)
			}
			if runtime.PromptFile != tt.wantPromptFile {
				t.Fatalf("unexpected prompt file: %q", runtime.PromptFile)
			}
			if runtime.ReadPromptStdin != tt.wantReadPromptStdin {
				t.Fatalf("unexpected stdin flag: %t", runtime.ReadPromptStdin)
			}
			if runtime.ResolvedPromptText != tt.wantResolved {
				t.Fatalf("unexpected resolved prompt text: %q", runtime.ResolvedPromptText)
			}
		})
	}
}

func TestNewFetchReviewsRunnerDispatchesTypedCommand(t *testing.T) {
	t.Parallel()

	dispatcher := kernel.NewDispatcher()
	wantErr := errors.New("fetch boom")
	handler := &reviewsFetchCaptureHandler{err: wantErr}
	kernel.Register(dispatcher, handler)

	_, err := newFetchReviewsRunner(dispatcher)(context.Background(), core.Config{
		WorkspaceRoot: "/workspace",
		Name:          "demo",
		Provider:      "coderabbit",
		PR:            "259",
		Round:         2,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("fetch runner error = %v, want %v", err, wantErr)
	}
	if !handler.called {
		t.Fatal("expected dispatcher handler to be called")
	}
	if handler.got.Provider != "coderabbit" || handler.got.PR != "259" || handler.got.Round != 2 {
		t.Fatalf("unexpected fetch command: %#v", handler.got)
	}
}

func TestNewMigrateRunnerDispatchesTypedCommand(t *testing.T) {
	t.Parallel()

	dispatcher := kernel.NewDispatcher()
	wantErr := errors.New("migrate boom")
	handler := &migrateCaptureHandler{err: wantErr}
	kernel.Register(dispatcher, handler)

	_, err := newMigrateRunner(dispatcher)(context.Background(), core.MigrationConfig{
		WorkspaceRoot: "/workspace",
		RootDir:       "/workspace/.compozy/tasks",
		Name:          "demo",
		TasksDir:      "/workspace/.compozy/tasks/demo",
		ReviewsDir:    "/workspace/.compozy/tasks/demo/reviews-001",
		DryRun:        true,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("migrate runner error = %v, want %v", err, wantErr)
	}
	if !handler.called {
		t.Fatal("expected dispatcher handler to be called")
	}
	if handler.got.RootDir != "/workspace/.compozy/tasks" {
		t.Fatalf("unexpected root dir: %q", handler.got.RootDir)
	}
	if handler.got.ReviewsDir != "/workspace/.compozy/tasks/demo/reviews-001" {
		t.Fatalf("unexpected reviews dir: %q", handler.got.ReviewsDir)
	}
	if !handler.got.DryRun {
		t.Fatal("expected dry run to pass through")
	}
}

func TestNewSyncRunnerDispatchesTypedCommand(t *testing.T) {
	t.Parallel()

	dispatcher := kernel.NewDispatcher()
	wantErr := errors.New("sync boom")
	handler := &syncCaptureHandler{err: wantErr}
	kernel.Register(dispatcher, handler)

	_, err := newSyncRunner(dispatcher)(context.Background(), core.SyncConfig{
		WorkspaceRoot: "/workspace",
		RootDir:       "/workspace/.compozy/tasks",
		Name:          "demo",
		TasksDir:      "/workspace/.compozy/tasks/demo",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("sync runner error = %v, want %v", err, wantErr)
	}
	if !handler.called {
		t.Fatal("expected dispatcher handler to be called")
	}
	if handler.got.RootDir != "/workspace/.compozy/tasks" || handler.got.TasksDir != "/workspace/.compozy/tasks/demo" {
		t.Fatalf("unexpected sync command: %#v", handler.got)
	}
}

func TestNewArchiveRunnerDispatchesTypedCommand(t *testing.T) {
	t.Parallel()

	dispatcher := kernel.NewDispatcher()
	wantErr := errors.New("archive boom")
	handler := &archiveCaptureHandler{err: wantErr}
	kernel.Register(dispatcher, handler)

	_, err := newArchiveRunner(dispatcher)(context.Background(), core.ArchiveConfig{
		WorkspaceRoot: "/workspace",
		RootDir:       "/workspace/.compozy/tasks",
		Name:          "demo",
		TasksDir:      "/workspace/.compozy/tasks/demo",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("archive runner error = %v, want %v", err, wantErr)
	}
	if !handler.called {
		t.Fatal("expected dispatcher handler to be called")
	}
	if handler.got.RootDir != "/workspace/.compozy/tasks" || handler.got.TasksDir != "/workspace/.compozy/tasks/demo" {
		t.Fatalf("unexpected archive command: %#v", handler.got)
	}
}

func TestNewRootCommandValidatesDispatcherAtStartup(t *testing.T) {
	previous := validateRootDispatcher
	t.Cleanup(func() {
		validateRootDispatcher = previous
	})

	called := false
	validateRootDispatcher = func(*kernel.Dispatcher) error {
		called = true
		return nil
	}

	_ = NewRootCommand()
	if !called {
		t.Fatal("expected root command construction to validate the dispatcher")
	}
}
