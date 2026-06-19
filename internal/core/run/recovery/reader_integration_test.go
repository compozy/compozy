package recovery_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/executor"
	"github.com/compozy/compozy/internal/core/run/recovery"
)

func TestReadRunOutcomeFromExecutorPersistedResult(t *testing.T) {
	t.Parallel()

	t.Run("Should read the executor-persisted run result", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		runArtifacts := model.NewRunArtifacts(workspaceRoot, "executor-generated-run")
		job := model.Job{
			CodeFiles: []string{"task_02.md"},
			Groups: map[string][]model.IssueEntry{
				"task_02.md": {{Name: "task_02.md", CodeFile: "task_02.md", Content: "Implement recovery primitives"}},
			},
			SafeName:      "task_02",
			Prompt:        []byte("Implement recovery primitives"),
			OutPromptPath: filepath.Join(runArtifacts.JobsDir, "task_02.prompt.md"),
			OutLog:        filepath.Join(runArtifacts.JobsDir, "task_02.out.log"),
			ErrLog:        filepath.Join(runArtifacts.JobsDir, "task_02.err.log"),
		}
		cfg := &model.RuntimeConfig{
			WorkspaceRoot:          workspaceRoot,
			Mode:                   model.ExecutionModePRDTasks,
			DryRun:                 true,
			OutputFormat:           model.OutputFormatText,
			DaemonOwned:            true,
			RetryBackoffMultiplier: 1.5,
		}

		if err := executor.Execute(
			context.Background(),
			[]model.Job{job},
			runArtifacts,
			nil,
			nil,
			cfg,
			nil,
		); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		outcome, err := recovery.ReadRunOutcome(runArtifacts)
		if err != nil {
			t.Fatalf("ReadRunOutcome() error = %v", err)
		}
		if outcome == nil {
			t.Fatal("expected outcome")
		}
		if outcome.RunID != runArtifacts.RunID {
			t.Fatalf("unexpected run ID: %q", outcome.RunID)
		}
		if outcome.Status != recovery.StatusSucceeded {
			t.Fatalf("unexpected status: %q", outcome.Status)
		}
		if outcome.ResultPath != runArtifacts.ResultPath {
			t.Fatalf("unexpected result path: %q", outcome.ResultPath)
		}
		if len(outcome.Jobs) != 1 {
			t.Fatalf("expected one job, got %d", len(outcome.Jobs))
		}
		if got := outcome.Jobs[0].SafeName; got != "task_02" {
			t.Fatalf("unexpected job safe name: %q", got)
		}
		if got := outcome.Jobs[0].ExitCode; got != 0 {
			t.Fatalf("unexpected job exit code: %d", got)
		}
	})
}
