package model_test

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestRuntimeConfigApplyDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should apply defaults for an empty runtime config", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{}
		cfg.ApplyDefaults()

		if cfg.Concurrent != 1 {
			t.Fatalf("unexpected concurrent default: %d", cfg.Concurrent)
		}
		if cfg.BatchSize != 1 {
			t.Fatalf("unexpected batch size default: %d", cfg.BatchSize)
		}
		if cfg.IDE != model.IDECodex {
			t.Fatalf("unexpected ide default: %q", cfg.IDE)
		}
		if cfg.ReasoningEffort != "medium" {
			t.Fatalf("unexpected reasoning default: %q", cfg.ReasoningEffort)
		}
		if cfg.AccessMode != model.AccessModeFull {
			t.Fatalf("unexpected access mode default: %q", cfg.AccessMode)
		}
		if cfg.Mode != model.ExecutionModePRReview {
			t.Fatalf("unexpected mode default: %q", cfg.Mode)
		}
		if cfg.OutputFormat != model.OutputFormatText {
			t.Fatalf("unexpected output format default: %q", cfg.OutputFormat)
		}
		if cfg.Timeout != model.DefaultActivityTimeout {
			t.Fatalf("unexpected timeout default: %s", cfg.Timeout)
		}
		if cfg.RetryBackoffMultiplier != 1.5 {
			t.Fatalf("unexpected retry multiplier default: %f", cfg.RetryBackoffMultiplier)
		}
	})
}

func TestPathHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should return the tasks base directory", func(t *testing.T) {
		t.Parallel()

		if got := model.TasksBaseDir(); got != filepath.Join(".compozy", "tasks") {
			t.Fatalf("unexpected tasks base dir: %q", got)
		}
	})

	t.Run("Should build the task workflow directory", func(t *testing.T) {
		t.Parallel()

		if got := model.TaskDirectory("acp-integration"); got != filepath.Join(".compozy", "tasks", "acp-integration") {
			t.Fatalf("unexpected task directory: %q", got)
		}
	})

	t.Run("Should build workspace-aware paths", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := filepath.Join(string(filepath.Separator), "tmp", "workspace")
		if got := model.CompozyDir(workspaceRoot); got != filepath.Join(workspaceRoot, ".compozy") {
			t.Fatalf("unexpected compozy dir: %q", got)
		}
		if got := model.ConfigPathForWorkspace(
			workspaceRoot,
		); got != filepath.Join(
			workspaceRoot,
			".compozy",
			"config.toml",
		) {
			t.Fatalf("unexpected config path: %q", got)
		}
		if got := model.TasksBaseDirForWorkspace(
			workspaceRoot,
		); got != filepath.Join(
			workspaceRoot,
			".compozy",
			"tasks",
		) {
			t.Fatalf("unexpected workspace tasks dir: %q", got)
		}
		if got := model.RunsBaseDirForWorkspace(
			workspaceRoot,
		); got != filepath.Join(
			workspaceRoot,
			".compozy",
			"runs",
		) {
			t.Fatalf("unexpected workspace runs dir: %q", got)
		}
		if got := model.TaskDirectoryForWorkspace(
			workspaceRoot,
			"demo",
		); got != filepath.Join(
			workspaceRoot,
			".compozy",
			"tasks",
			"demo",
		) {
			t.Fatalf("unexpected workspace task dir: %q", got)
		}
	})

	t.Run("Should build the archived tasks directory", func(t *testing.T) {
		t.Parallel()

		baseDir := filepath.Join(string(filepath.Separator), "tmp", "workflows")
		if got := model.ArchivedTasksDir(baseDir); got != filepath.Join(baseDir, "_archived") {
			t.Fatalf("unexpected archived tasks dir: %q", got)
		}
	})

	t.Run("Should build run artifact paths under the workspace runs directory", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := filepath.Join(string(filepath.Separator), "tmp", "workspace")
		runArtifacts := model.NewRunArtifacts(workspaceRoot, "tasks-demo-20260405-120000-000000000")
		if got, want := runArtifacts.RunDir, filepath.Join(
			workspaceRoot,
			".compozy",
			"runs",
			"tasks-demo-20260405-120000-000000000",
		); got != want {
			t.Fatalf("unexpected run dir\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := runArtifacts.RunMetaPath, filepath.Join(runArtifacts.RunDir, "run.json"); got != want {
			t.Fatalf("unexpected run meta path\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := runArtifacts.JobsDir, filepath.Join(runArtifacts.RunDir, "jobs"); got != want {
			t.Fatalf("unexpected jobs dir\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := runArtifacts.ResultPath, filepath.Join(runArtifacts.RunDir, "result.json"); got != want {
			t.Fatalf("unexpected result path\nwant: %q\ngot:  %q", want, got)
		}

		jobArtifacts := runArtifacts.JobArtifacts("task_01-abc123")
		if got, want := jobArtifacts.PromptPath, filepath.Join(
			runArtifacts.JobsDir,
			"task_01-abc123.prompt.md",
		); got != want {
			t.Fatalf("unexpected prompt path\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := jobArtifacts.OutLogPath, filepath.Join(
			runArtifacts.JobsDir,
			"task_01-abc123.out.log",
		); got != want {
			t.Fatalf("unexpected stdout log path\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := jobArtifacts.ErrLogPath, filepath.Join(
			runArtifacts.JobsDir,
			"task_01-abc123.err.log",
		); got != want {
			t.Fatalf("unexpected stderr log path\nwant: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Should sanitize unsafe run identifiers", func(t *testing.T) {
		t.Parallel()

		runArtifacts := model.NewRunArtifacts("", " review/demo\\nested ")
		if got, want := runArtifacts.RunID, "review-demo-nested"; got != want {
			t.Fatalf("unexpected sanitized run id\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := runArtifacts.RunDir, filepath.Join(".compozy", "runs", "review-demo-nested"); got != want {
			t.Fatalf("unexpected sanitized run dir\nwant: %q\ngot:  %q", want, got)
		}
	})
}

func TestIsActiveWorkflowDirName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "Should return true for regular workflow names", input: "workflow-one", want: true},
		{name: "Should return false for an empty name", input: "", want: false},
		{name: "Should return false for hidden directories", input: ".hidden", want: false},
		{
			name:  "Should return false for archived workflow directories",
			input: model.ArchivedWorkflowDirName,
			want:  false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := model.IsActiveWorkflowDirName(tc.input); got != tc.want {
				t.Fatalf("unexpected active workflow result for %q: got %v want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestJobIssueCount(t *testing.T) {
	t.Parallel()

	t.Run("Should count issues across all groups", func(t *testing.T) {
		t.Parallel()

		job := model.Job{
			Groups: map[string][]model.IssueEntry{
				"group-a": {{Name: "issue-1"}, {Name: "issue-2"}},
				"group-b": {{Name: "issue-3"}},
			},
		}

		if got := job.IssueCount(); got != 3 {
			t.Fatalf("unexpected issue count: %d", got)
		}
	})
}

func TestUsageTotalUsesExplicitTotalWhenPresent(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer explicit total tokens when present", func(t *testing.T) {
		t.Parallel()

		usage := model.Usage{InputTokens: 2, OutputTokens: 3, TotalTokens: 99}
		if got := usage.Total(); got != 99 {
			t.Fatalf("unexpected usage total: %d", got)
		}
	})

	t.Run("Should derive total tokens when the explicit total is absent", func(t *testing.T) {
		t.Parallel()

		usage := model.Usage{InputTokens: 2, OutputTokens: 3}
		if got := usage.Total(); got != 5 {
			t.Fatalf("unexpected derived usage total: %d", got)
		}
	})
}

func TestRuntimeConfigApplyDefaultsPreservesExplicitValues(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve explicit runtime config values", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			Concurrent:             3,
			BatchSize:              2,
			IDE:                    model.IDEClaude,
			TailLines:              10,
			ReasoningEffort:        "high",
			AccessMode:             model.AccessModeDefault,
			Mode:                   model.ExecutionModePRDTasks,
			OutputFormat:           model.OutputFormatJSON,
			Timeout:                30 * time.Second,
			RetryBackoffMultiplier: 2,
		}
		cfg.ApplyDefaults()

		if cfg.Concurrent != 3 || cfg.BatchSize != 2 || cfg.IDE != model.IDEClaude ||
			cfg.AccessMode != model.AccessModeDefault ||
			cfg.Mode != model.ExecutionModePRDTasks ||
			cfg.OutputFormat != model.OutputFormatJSON {
			t.Fatalf("apply defaults should preserve explicit values: %#v", cfg)
		}
	})
}

func TestRuntimeConfigSurfaceOmitsSystemPromptWhileJobsRetainIt(t *testing.T) {
	t.Parallel()

	t.Run("Should keep runtime config free of unreachable system prompt fields", func(t *testing.T) {
		t.Parallel()

		runtimeType := reflect.TypeOf(model.RuntimeConfig{})
		if _, ok := runtimeType.FieldByName("SystemPrompt"); ok {
			t.Fatal("expected runtime config to omit SystemPrompt")
		}
	})

	t.Run("Should keep job system prompt support for prepared prompts", func(t *testing.T) {
		t.Parallel()

		jobType := reflect.TypeOf(model.Job{})
		if _, ok := jobType.FieldByName("SystemPrompt"); !ok {
			t.Fatal("expected prepared jobs to retain SystemPrompt")
		}
	})
}

func TestTaskMetadataUsesV2Fields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		typ       reflect.Type
		required  []string
		forbidden []string
	}{
		{
			name:      "task entry includes title and drops domain scope",
			typ:       reflect.TypeOf(model.TaskEntry{}),
			required:  []string{"Title", "Status", "TaskType", "Complexity", "Dependencies"},
			forbidden: []string{"Domain", "Scope"},
		},
		{
			name:      "task file meta includes title and drops domain scope",
			typ:       reflect.TypeOf(model.TaskFileMeta{}),
			required:  []string{"Title", "Status", "TaskType", "Complexity", "Dependencies"},
			forbidden: []string{"Domain", "Scope"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, fieldName := range tt.required {
				if _, ok := tt.typ.FieldByName(fieldName); !ok {
					t.Fatalf("expected %s to contain field %q", tt.typ.Name(), fieldName)
				}
			}
			for _, fieldName := range tt.forbidden {
				if _, ok := tt.typ.FieldByName(fieldName); ok {
					t.Fatalf("expected %s to omit field %q", tt.typ.Name(), fieldName)
				}
			}
		})
	}
}
