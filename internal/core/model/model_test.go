package model_test

import (
	"os"
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
		if cfg.SoundOnCompleted != "" || cfg.SoundOnFailed != "" {
			t.Fatalf(
				"expected sound presets to stay empty when sound is disabled: got %q / %q",
				cfg.SoundOnCompleted,
				cfg.SoundOnFailed,
			)
		}
	})

	t.Run("Should fill sound presets only when sound is enabled", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{SoundEnabled: true}
		cfg.ApplyDefaults()
		if cfg.SoundOnCompleted != model.DefaultSoundOnCompleted {
			t.Fatalf("unexpected on_completed default: %q", cfg.SoundOnCompleted)
		}
		if cfg.SoundOnFailed != model.DefaultSoundOnFailed {
			t.Fatalf("unexpected on_failed default: %q", cfg.SoundOnFailed)
		}
	})

	t.Run("Should preserve explicit sound presets over defaults", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			SoundEnabled:     true,
			SoundOnCompleted: "/custom/done.aiff",
			SoundOnFailed:    "/custom/fail.aiff",
		}
		cfg.ApplyDefaults()
		if cfg.SoundOnCompleted != "/custom/done.aiff" {
			t.Fatalf("explicit on_completed was overwritten: %q", cfg.SoundOnCompleted)
		}
		if cfg.SoundOnFailed != "/custom/fail.aiff" {
			t.Fatalf("explicit on_failed was overwritten: %q", cfg.SoundOnFailed)
		}
	})

	t.Run("Should treat whitespace-only sound presets as unset and apply defaults", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			SoundEnabled:     true,
			SoundOnCompleted: "   ",
			SoundOnFailed:    "\t\n",
		}
		cfg.ApplyDefaults()
		if cfg.SoundOnCompleted != model.DefaultSoundOnCompleted {
			t.Fatalf("whitespace on_completed was not replaced with default: %q", cfg.SoundOnCompleted)
		}
		if cfg.SoundOnFailed != model.DefaultSoundOnFailed {
			t.Fatalf("whitespace on_failed was not replaced with default: %q", cfg.SoundOnFailed)
		}
	})

	t.Run("Should trim surrounding whitespace from explicit sound presets", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			SoundEnabled:     true,
			SoundOnCompleted: "  /custom/done.aiff  ",
			SoundOnFailed:    "\tbasso\n",
		}
		cfg.ApplyDefaults()
		if cfg.SoundOnCompleted != "/custom/done.aiff" {
			t.Fatalf("explicit on_completed was not trimmed: %q", cfg.SoundOnCompleted)
		}
		if cfg.SoundOnFailed != "basso" {
			t.Fatalf("explicit on_failed was not trimmed: %q", cfg.SoundOnFailed)
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

	t.Run("Should build the archived workflow name with timestamp millis and short id", func(t *testing.T) {
		t.Parallel()

		archivedAt := time.Date(2026, 4, 17, 18, 45, 12, 345000000, time.UTC)
		got := model.ArchivedWorkflowName("daemon", "wf-a1b2c3d4e5f60708", archivedAt)
		want := "1776451512345-a1b2c3d4-daemon"
		if got != want {
			t.Fatalf("ArchivedWorkflowName() = %q, want %q", got, want)
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
		if got, want := runArtifacts.RunDBPath, filepath.Join(runArtifacts.RunDir, "run.db"); got != want {
			t.Fatalf("unexpected run db path\nwant: %q\ngot:  %q", want, got)
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

	t.Run("Should sanitize unsafe job artifact names into the jobs namespace", func(t *testing.T) {
		t.Parallel()

		runArtifacts := model.NewRunArtifacts("", "demo-run")
		jobArtifacts := runArtifacts.JobArtifacts("../nested/task 01")
		if got, want := jobArtifacts.PromptPath, filepath.Join(
			runArtifacts.JobsDir,
			"nested-task-01.prompt.md",
		); got != want {
			t.Fatalf("unexpected sanitized prompt path\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := jobArtifacts.OutLogPath, filepath.Join(
			runArtifacts.JobsDir,
			"nested-task-01.out.log",
		); got != want {
			t.Fatalf("unexpected sanitized stdout path\nwant: %q\ngot:  %q", want, got)
		}
		if got, want := jobArtifacts.ErrLogPath, filepath.Join(
			runArtifacts.JobsDir,
			"nested-task-01.err.log",
		); got != want {
			t.Fatalf("unexpected sanitized stderr path\nwant: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Should reject dot-segment run identifiers", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			runID string
		}{
			{name: "current directory", runID: "."},
			{name: "parent directory", runID: ".."},
			{name: "punctuation only", runID: " !!! "},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				runArtifacts := model.NewRunArtifacts("", tc.runID)
				if got, want := runArtifacts.RunID, "run"; got != want {
					t.Fatalf("unexpected sanitized run id for %q\nwant: %q\ngot:  %q", tc.runID, want, got)
				}
				if got, want := runArtifacts.RunDir, filepath.Join(".compozy", "runs", "run"); got != want {
					t.Fatalf("unexpected sanitized run dir for %q\nwant: %q\ngot:  %q", tc.runID, want, got)
				}
			})
		}
	})
}

func TestResolveHomeRunArtifactsUsesHomeScopedRunDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	runArtifacts, err := model.ResolveHomeRunArtifacts("daemon-run-123")
	if err != nil {
		t.Fatalf("ResolveHomeRunArtifacts(): %v", err)
	}

	wantRunDir := filepath.Join(homeDir, ".compozy", "runs", "daemon-run-123")
	if got := runArtifacts.RunDir; got != wantRunDir {
		t.Fatalf("home run dir = %q, want %q", got, wantRunDir)
	}
	if got := runArtifacts.RunDBPath; got != filepath.Join(wantRunDir, "run.db") {
		t.Fatalf("home run db path = %q, want %q", got, filepath.Join(wantRunDir, "run.db"))
	}
}

func TestResolvePersistedRunArtifactsPrefersWorkspaceMetadata(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "exec-123")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(runArtifacts.RunMetaPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write run.json: %v", err)
	}

	resolved, err := model.ResolvePersistedRunArtifacts(workspaceRoot, "exec-123")
	if err != nil {
		t.Fatalf("ResolvePersistedRunArtifacts(): %v", err)
	}
	if got, want := resolved.RunDir, runArtifacts.RunDir; got != want {
		t.Fatalf("resolved run dir = %q, want %q", got, want)
	}
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

func TestRuntimeConfigRuntimeForTask(t *testing.T) {
	t.Parallel()

	t.Run("Should apply type rules before id rules", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			IDE:             model.IDECodex,
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
			TaskRuntimeRules: []model.TaskRuntimeRule{
				{
					Type:            testStringPointer("frontend"),
					IDE:             testStringPointer(model.IDEClaude),
					Model:           testStringPointer("sonnet"),
					ReasoningEffort: testStringPointer("high"),
				},
				{
					ID:              testStringPointer("task_02"),
					IDE:             testStringPointer(model.IDECursor),
					Model:           testStringPointer("cursor-model"),
					ReasoningEffort: testStringPointer("xhigh"),
				},
			},
		}

		frontendOnly := cfg.RuntimeForTask(model.TaskRuntimeTarget{ID: "task_01", Type: "frontend"})
		if frontendOnly.IDE != model.IDEClaude || frontendOnly.Model != "sonnet" ||
			frontendOnly.ReasoningEffort != "high" {
			t.Fatalf("unexpected type-resolved runtime: %#v", frontendOnly)
		}

		idOverride := cfg.RuntimeForTask(model.TaskRuntimeTarget{ID: "task_02", Type: "frontend"})
		if idOverride.IDE != model.IDECursor || idOverride.Model != "cursor-model" ||
			idOverride.ReasoningEffort != "xhigh" {
			t.Fatalf("unexpected id-resolved runtime: %#v", idOverride)
		}

		baseOnly := cfg.RuntimeForTask(model.TaskRuntimeTarget{ID: "task_03", Type: "backend"})
		if baseOnly.IDE != model.IDECodex || baseOnly.Model != "gpt-5.4" || baseOnly.ReasoningEffort != "medium" {
			t.Fatalf("unexpected base runtime: %#v", baseOnly)
		}
	})

	t.Run("Should clone runtime config without mutating the base", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			IDE:             model.IDECodex,
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
			TaskRuntimeRules: []model.TaskRuntimeRule{{
				ID:    testStringPointer("task_01"),
				Model: testStringPointer("override-model"),
			}},
		}

		resolved := cfg.RuntimeForTask(model.TaskRuntimeTarget{ID: "task_01"})
		resolved.Model = "mutated"

		if cfg.Model != "gpt-5.4" {
			t.Fatalf("base runtime was mutated: %#v", cfg)
		}
		if len(resolved.TaskRuntimeRules) != 0 {
			t.Fatalf("expected resolved runtime to clear task rules, got %#v", resolved.TaskRuntimeRules)
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

func testStringPointer(value string) *string {
	return &value
}
