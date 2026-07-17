package model_test

import (
	"encoding/json"
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
		if cfg.TargetTaskNumber != nil {
			t.Fatalf("expected target task number to stay unset, got %d", *cfg.TargetTaskNumber)
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

func TestRuntimeConfigStallDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should apply on-by-default stall policy for an empty config", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{}
		cfg.ApplyDefaults()

		policy := cfg.StallPolicy()
		if !policy.Enabled {
			t.Fatalf("expected stall enabled by default, got %#v", policy)
		}
		if policy.IdleTimeout != model.DefaultStallIdleTimeout {
			t.Fatalf("idle timeout = %s, want %s", policy.IdleTimeout, model.DefaultStallIdleTimeout)
		}
		if policy.ChildTimeout != model.DefaultStallChildTimeout {
			t.Fatalf("child timeout = %s, want %s", policy.ChildTimeout, model.DefaultStallChildTimeout)
		}
		if policy.ChildTimeout <= policy.IdleTimeout {
			t.Fatalf("child timeout %s must exceed idle timeout %s", policy.ChildTimeout, policy.IdleTimeout)
		}
		if policy.TerminalCap != model.DefaultStallTerminalCap {
			t.Fatalf("terminal cap = %s, want %s", policy.TerminalCap, model.DefaultStallTerminalCap)
		}
		if policy.Retries != model.DefaultStallRetries {
			t.Fatalf("retries = %d, want %d", policy.Retries, model.DefaultStallRetries)
		}
		if cfg.SoundOnParked != model.DefaultSoundOnParked {
			t.Fatalf("parked sound = %q, want %q", cfg.SoundOnParked, model.DefaultSoundOnParked)
		}
	})

	t.Run("Should correct child timeout to exceed idle timeout", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			StallTimeout:      10 * time.Minute,
			ChildStallTimeout: 5 * time.Minute,
		}
		cfg.ApplyDefaults()

		policy := cfg.StallPolicy()
		if policy.ChildTimeout <= policy.IdleTimeout {
			t.Fatalf("invariant not enforced: child %s <= idle %s", policy.ChildTimeout, policy.IdleTimeout)
		}
		if policy.ChildTimeout != 20*time.Minute {
			t.Fatalf("corrected child timeout = %s, want 20m", policy.ChildTimeout)
		}
	})

	t.Run("Should resolve an explicit disable to a disabled policy", func(t *testing.T) {
		t.Parallel()

		disabled := false
		cfg := &model.RuntimeConfig{StallEnabled: &disabled}
		cfg.ApplyDefaults()

		if cfg.StallPolicy().Enabled {
			t.Fatal("expected explicit stall disable to be preserved through defaulting")
		}
	})

	t.Run("Should preserve an explicit zero-retry override", func(t *testing.T) {
		t.Parallel()

		zero := 0
		cfg := &model.RuntimeConfig{StallRetries: &zero}
		cfg.ApplyDefaults()

		if cfg.StallPolicy().Retries != 0 {
			t.Fatalf("retries = %d, want 0 preserved", cfg.StallPolicy().Retries)
		}
	})

	t.Run("Should preserve explicit stall durations and trim parked sound", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			StallTimeout:           90 * time.Second,
			ChildStallTimeout:      4 * time.Minute,
			TerminalCommandTimeout: 30 * time.Minute,
			SoundOnParked:          "  ping  ",
		}
		cfg.ApplyDefaults()

		policy := cfg.StallPolicy()
		if policy.IdleTimeout != 90*time.Second {
			t.Fatalf("idle timeout = %s, want 90s", policy.IdleTimeout)
		}
		if policy.ChildTimeout != 4*time.Minute {
			t.Fatalf("child timeout = %s, want 4m", policy.ChildTimeout)
		}
		if policy.TerminalCap != 30*time.Minute {
			t.Fatalf("terminal cap = %s, want 30m", policy.TerminalCap)
		}
		if cfg.SoundOnParked != "ping" {
			t.Fatalf("parked sound = %q, want trimmed ping", cfg.SoundOnParked)
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
		if got, want := runArtifacts.RecoveryDir, filepath.Join(runArtifacts.RunDir, "recovery"); got != want {
			t.Fatalf("unexpected recovery dir\nwant: %q\ngot:  %q", want, got)
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
		if got, want := jobArtifacts.WorktreeScopePath, filepath.Join(
			runArtifacts.JobsDir,
			"task_01-abc123.worktree_scope.json",
		); got != want {
			t.Fatalf("unexpected worktree scope path\nwant: %q\ngot:  %q", want, got)
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
		if got, want := jobArtifacts.WorktreeScopePath, filepath.Join(
			runArtifacts.JobsDir,
			"nested-task-01.worktree_scope.json",
		); got != want {
			t.Fatalf("unexpected sanitized worktree scope path\nwant: %q\ngot:  %q", want, got)
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
			Model:           "gpt-5.5",
			ReasoningEffort: "medium",
			TaskRuntimeRules: []model.TaskRuntimeRule{
				{
					Complexity:      testStringPointer("low"),
					IDE:             testStringPointer(model.IDEGemini),
					Model:           testStringPointer("flash"),
					ReasoningEffort: testStringPointer("low"),
				},
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

		frontendOnly := cfg.RuntimeForTask(model.TaskRuntimeTarget{
			ID: "task_01", Type: "frontend", Complexity: "low",
		})
		if frontendOnly.IDE != model.IDEClaude || frontendOnly.Model != "sonnet" ||
			frontendOnly.ReasoningEffort != "high" {
			t.Fatalf("unexpected type-resolved runtime: %#v", frontendOnly)
		}

		idOverride := cfg.RuntimeForTask(model.TaskRuntimeTarget{
			ID: "task_02", Type: "frontend", Complexity: "low",
		})
		if idOverride.IDE != model.IDECursor || idOverride.Model != "cursor-model" ||
			idOverride.ReasoningEffort != "xhigh" {
			t.Fatalf("unexpected id-resolved runtime: %#v", idOverride)
		}

		baseOnly := cfg.RuntimeForTask(model.TaskRuntimeTarget{
			ID: "task_03", Type: "backend", Complexity: "medium",
		})
		if baseOnly.IDE != model.IDECodex || baseOnly.Model != "gpt-5.5" || baseOnly.ReasoningEffort != "medium" {
			t.Fatalf("unexpected base runtime: %#v", baseOnly)
		}
	})

	t.Run("Should apply complexity defaults without overriding explicit runtime fields", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			IDE:             model.IDECodex,
			Model:           "explicit-model",
			ReasoningEffort: "medium",
			ExplicitRuntime: model.ExplicitRuntimeFlags{Model: true},
			TaskRuntimeRules: []model.TaskRuntimeRule{{
				Complexity:      testStringPointer("low"),
				IDE:             testStringPointer(model.IDEClaude),
				Model:           testStringPointer("haiku"),
				ReasoningEffort: testStringPointer("low"),
			}},
		}

		resolved := cfg.RuntimeForTask(model.TaskRuntimeTarget{Complexity: "low"})
		if resolved.IDE != model.IDEClaude || resolved.Model != "explicit-model" ||
			resolved.ReasoningEffort != "low" {
			t.Fatalf("unexpected complexity-resolved runtime: %#v", resolved)
		}
		if cfg.IDE != model.IDECodex || cfg.Model != "explicit-model" || cfg.ReasoningEffort != "medium" {
			t.Fatalf("complexity resolution mutated base runtime: %#v", cfg)
		}
	})

	t.Run("Should clone runtime config without mutating the base", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			IDE:             model.IDECodex,
			Model:           "gpt-5.5",
			ReasoningEffort: "medium",
			TaskRuntimeRules: []model.TaskRuntimeRule{{
				ID:    testStringPointer("task_01"),
				Model: testStringPointer("override-model"),
			}},
		}

		resolved := cfg.RuntimeForTask(model.TaskRuntimeTarget{ID: "task_01"})
		resolved.Model = "mutated"

		if cfg.Model != "gpt-5.5" {
			t.Fatalf("base runtime was mutated: %#v", cfg)
		}
		if len(resolved.TaskRuntimeRules) != 0 {
			t.Fatalf("expected resolved runtime to clear task rules, got %#v", resolved.TaskRuntimeRules)
		}
	})

	t.Run("Should scope matching rules to workflow when qualifier is set", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			IDE:   model.IDECodex,
			Model: "base",
			TaskRuntimeRules: []model.TaskRuntimeRule{
				{
					Workflow: testStringPointer("alpha"),
					ID:       testStringPointer("task_01"),
					Model:    testStringPointer("alpha-model"),
				},
				{
					ID:    testStringPointer("task_02"),
					Model: testStringPointer("global-model"),
				},
				{
					Workflow: testStringPointer("beta"),
					Type:     testStringPointer("frontend"),
					IDE:      testStringPointer(model.IDEClaude),
				},
			},
		}

		alpha := cfg.RuntimeForTask(model.TaskRuntimeTarget{Workflow: "alpha", ID: "task_01", Type: "frontend"})
		if alpha.Model != "alpha-model" || alpha.IDE != model.IDECodex {
			t.Fatalf("unexpected alpha runtime: %#v", alpha)
		}

		beta := cfg.RuntimeForTask(model.TaskRuntimeTarget{Workflow: "beta", ID: "task_02", Type: "frontend"})
		if beta.IDE != model.IDEClaude || beta.Model != "global-model" {
			t.Fatalf("unexpected beta runtime: %#v", beta)
		}

		gamma := cfg.RuntimeForTask(model.TaskRuntimeTarget{Workflow: "gamma", ID: "task_01", Type: "frontend"})
		if gamma.Model != "base" || gamma.IDE != model.IDECodex {
			t.Fatalf("unexpected gamma runtime: %#v", gamma)
		}
	})

	t.Run("Should treat blank workflow qualifiers as unscoped", func(t *testing.T) {
		t.Parallel()

		cfg := &model.RuntimeConfig{
			IDE:   model.IDECodex,
			Model: "base",
			TaskRuntimeRules: []model.TaskRuntimeRule{{
				Workflow: testStringPointer(" \t "),
				Type:     testStringPointer("backend"),
				Model:    testStringPointer("blank-workflow-model"),
			}},
		}

		resolved := cfg.RuntimeForTask(model.TaskRuntimeTarget{Workflow: "alpha", ID: "task_01", Type: "backend"})
		if resolved.Model != "blank-workflow-model" {
			t.Fatalf("blank workflow qualifier should match any workflow, got %#v", resolved)
		}
	})
}

func TestRuntimeConfigClonePreservesTargetTaskNumber(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve and deep-copy the target task number", func(t *testing.T) {
		t.Parallel()

		targetTaskNumber := 3
		cfg := &model.RuntimeConfig{
			TargetTaskNumber: &targetTaskNumber,
			AddDirs:          []string{"/workspace/docs"},
		}

		cloned := cfg.Clone()
		if cloned == nil {
			t.Fatal("Clone() = nil, want runtime config")
		}
		if cloned.TargetTaskNumber == nil {
			t.Fatal("Clone().TargetTaskNumber = nil, want copied target")
		}
		if cloned.TargetTaskNumber == cfg.TargetTaskNumber {
			t.Fatal("Clone().TargetTaskNumber aliases base pointer")
		}
		if *cloned.TargetTaskNumber != targetTaskNumber {
			t.Fatalf("Clone().TargetTaskNumber = %d, want %d", *cloned.TargetTaskNumber, targetTaskNumber)
		}

		*cloned.TargetTaskNumber = 7
		cloned.AddDirs[0] = "/workspace/changed"
		if *cfg.TargetTaskNumber != targetTaskNumber {
			t.Fatalf("base TargetTaskNumber changed to %d, want %d", *cfg.TargetTaskNumber, targetTaskNumber)
		}
		if cfg.AddDirs[0] != "/workspace/docs" {
			t.Fatalf("base AddDirs changed to %#v", cfg.AddDirs)
		}
	})

	t.Run("Should deep-copy stall pointer fields", func(t *testing.T) {
		t.Parallel()

		enabled := false
		retries := 2
		cfg := &model.RuntimeConfig{StallEnabled: &enabled, StallRetries: &retries}

		cloned := cfg.Clone()
		if cloned.StallEnabled == cfg.StallEnabled || cloned.StallRetries == cfg.StallRetries {
			t.Fatal("Clone() aliases stall pointer fields")
		}
		*cloned.StallEnabled = true
		*cloned.StallRetries = 9
		if *cfg.StallEnabled != false || *cfg.StallRetries != 2 {
			t.Fatalf("mutating clone changed base stall fields: %#v / %#v", cfg.StallEnabled, cfg.StallRetries)
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

		targetTaskNumber := 5
		cfg := &model.RuntimeConfig{
			Concurrent:             3,
			BatchSize:              2,
			IDE:                    model.IDEClaude,
			TailLines:              10,
			ReasoningEffort:        "high",
			AccessMode:             model.AccessModeDefault,
			Mode:                   model.ExecutionModePRDTasks,
			OutputFormat:           model.OutputFormatJSON,
			TargetTaskNumber:       &targetTaskNumber,
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
		if cfg.TargetTaskNumber == nil || *cfg.TargetTaskNumber != targetTaskNumber {
			t.Fatalf("apply defaults changed TargetTaskNumber: %#v", cfg.TargetTaskNumber)
		}
	})
}

func TestRuntimeConfigSurfaceIncludesRecoveryRuntimeFields(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip SystemPrompt and RecoveryAttempt", func(t *testing.T) {
		t.Parallel()

		cfg := model.RuntimeConfig{
			SystemPrompt:    "fix root causes only",
			RecoveryAttempt: 1,
		}
		payload, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal runtime config: %v", err)
		}
		var got model.RuntimeConfig
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatalf("unmarshal runtime config: %v", err)
		}
		if got.SystemPrompt != cfg.SystemPrompt {
			t.Fatalf("SystemPrompt did not round-trip: %q", got.SystemPrompt)
		}
		if got.RecoveryAttempt != cfg.RecoveryAttempt {
			t.Fatalf("RecoveryAttempt did not round-trip: %d", got.RecoveryAttempt)
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
