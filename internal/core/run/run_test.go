package run

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestExecuteDelegatesToExecutorPackage(t *testing.T) {
	t.Parallel()

	previous := execute
	defer func() { execute = previous }()

	wantErr := errors.New("executor called")
	called := false
	execute = func(
		_ context.Context,
		jobs []model.Job,
		runArtifacts model.RunArtifacts,
		_ *journal.Journal,
		_ *events.Bus[events.Event],
		cfg *model.RuntimeConfig,
		_ model.RuntimeManager,
	) error {
		called = true
		if len(jobs) != 1 || jobs[0].SafeName != "task_01" {
			t.Fatalf("unexpected delegated jobs: %#v", jobs)
		}
		if runArtifacts.RunID != "run-123" {
			t.Fatalf("unexpected delegated artifacts: %#v", runArtifacts)
		}
		if cfg == nil || cfg.WorkspaceRoot != "/tmp/workspace" {
			t.Fatalf("unexpected delegated config: %#v", cfg)
		}
		return wantErr
	}

	err := Execute(
		context.Background(),
		[]model.Job{{SafeName: "task_01"}},
		model.RunArtifacts{RunID: "run-123"},
		nil,
		nil,
		&model.RuntimeConfig{WorkspaceRoot: "/tmp/workspace"},
		nil,
	)
	if !called {
		t.Fatal("expected Execute to delegate to executor package")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected delegated error: %v", err)
	}
}

func TestExecuteExecDelegatesToExecPackage(t *testing.T) {
	t.Parallel()

	previous := executeExec
	defer func() { executeExec = previous }()

	wantErr := errors.New("exec called")
	called := false
	executeExec = func(_ context.Context, cfg *model.RuntimeConfig, scope model.RunScope) error {
		called = true
		if cfg == nil || cfg.PromptText != "run it" {
			t.Fatalf("unexpected delegated config: %#v", cfg)
		}
		if scope != nil {
			t.Fatalf("unexpected delegated scope: %#v", scope)
		}
		return wantErr
	}

	err := ExecuteExec(context.Background(), &model.RuntimeConfig{PromptText: "run it"}, nil)
	if !called {
		t.Fatal("expected ExecuteExec to delegate to exec package")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected delegated error: %v", err)
	}
}
