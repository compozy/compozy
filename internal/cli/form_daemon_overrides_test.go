package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/spf13/cobra"
)

func TestApplyInputMarksFormValuesAsExplicitOverrides(t *testing.T) {
	t.Parallel()

	t.Run("marks form-applied values as changed", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "reviews-fix"}
		cmd.Flags().Int("batch-size", 1, "batch size")

		gotBatchSize := 0
		applyInput(cmd, "batch-size", "10", parseIntInput, func(value int) {
			gotBatchSize = value
		})

		if gotBatchSize != 10 {
			t.Fatalf("form-applied batch size = %d, want 10", gotBatchSize)
		}
		if !cmd.Flags().Changed("batch-size") {
			t.Fatal("expected form-applied batch-size to be marked explicit")
		}
	})

	t.Run("preserves already-explicit CLI flags", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "reviews-fix"}
		cmd.Flags().Int("batch-size", 1, "batch size")
		if err := cmd.Flags().Set("batch-size", "7"); err != nil {
			t.Fatalf("set explicit batch-size: %v", err)
		}

		gotBatchSize := 7
		applyInput(cmd, "batch-size", "10", parseIntInput, func(value int) {
			gotBatchSize = value
		})

		if gotBatchSize != 7 {
			t.Fatalf("explicit batch-size should win, got %d", gotBatchSize)
		}
		if !cmd.Flags().Changed("batch-size") {
			t.Fatal("expected batch-size to remain marked explicit")
		}
	})

	t.Run("keeps blank optional inputs implicit", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{Use: "reviews-fix"}
		cmd.Flags().Int("batch-size", 1, "batch size")
		cmd.Flags().String("add-dir", "", "additional directories")

		gotBatchSize := 7
		applyInput(cmd, "batch-size", "", parseIntInput, func(value int) {
			gotBatchSize = value
		})
		if gotBatchSize != 7 {
			t.Fatalf("blank batch-size should preserve prior value, got %d", gotBatchSize)
		}
		if cmd.Flags().Changed("batch-size") {
			t.Fatal("expected blank batch-size to remain implicit")
		}

		gotAddDirs := []string{"../shared"}
		applyInput(cmd, "add-dir", "", parseStringSliceInput, func(value []string) {
			gotAddDirs = value
		})
		if len(gotAddDirs) != 1 || gotAddDirs[0] != "../shared" {
			t.Fatalf("blank add-dir should preserve prior value, got %#v", gotAddDirs)
		}
		if cmd.Flags().Changed("add-dir") {
			t.Fatal("expected blank add-dir to remain implicit")
		}
	})
}

func TestReviewsFixInteractiveFormPropagatesDaemonBatchingOverrides(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := &reviewExecCaptureClient{
		stubDaemonCommandClient: &stubDaemonCommandClient{
			health: apicore.DaemonHealth{Ready: true},
			reviewRun: apicore.Run{
				RunID:            "run-review-form-batching-001",
				Mode:             "review",
				Status:           "running",
				PresentationMode: attachModeUI,
				StartedAt:        time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	var attachedRunID string
	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		attachedRunID = runID
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	defaults.collectForm = func(cmd *cobra.Command, state *commandState) error {
		inputs := &formInputs{
			name:            "demo",
			round:           "1",
			concurrent:      "3",
			batchSize:       "10",
			ide:             "codex",
			model:           "gpt-5.5",
			reasoningEffort: "high",
			autoCommit:      true,
		}
		inputs.apply(cmd, state)
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "reviews", "fix")
	if err != nil {
		t.Fatalf("execute interactive reviews fix: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if attachedRunID != "run-review-form-batching-001" {
		t.Fatalf("expected UI attach for run-review-form-batching-001, got %q", attachedRunID)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected quiet form flow before ui attach, got stdout=%q stderr=%q", stdout, stderr)
	}

	var batching struct {
		BatchSize  int `json:"batch_size"`
		Concurrent int `json:"concurrent"`
	}
	if err := json.Unmarshal(client.startReviewReq.Batching, &batching); err != nil {
		t.Fatalf("decode review batching overrides: %v", err)
	}
	if batching.BatchSize != 10 || batching.Concurrent != 3 {
		t.Fatalf("unexpected review batching overrides: %#v", batching)
	}

	var overrides daemonRuntimeOverrides
	if err := json.Unmarshal(client.startReviewReq.RuntimeOverrides, &overrides); err != nil {
		t.Fatalf("decode review runtime overrides: %v", err)
	}
	if overrides.AutoCommit == nil || !*overrides.AutoCommit {
		t.Fatalf("expected auto-commit form override, got %#v", overrides)
	}
	if overrides.IDE == nil || *overrides.IDE != "codex" {
		t.Fatalf("expected ide form override, got %#v", overrides)
	}
	if overrides.Model == nil || *overrides.Model != "gpt-5.5" {
		t.Fatalf("expected model form override, got %#v", overrides)
	}
	if overrides.ReasoningEffort == nil || *overrides.ReasoningEffort != "high" {
		t.Fatalf("expected reasoning-effort form override, got %#v", overrides)
	}
}

func TestTasksRunInteractiveFormPropagatesDaemonRuntimeOverrides(t *testing.T) {
	workspaceRoot, _ := makeValidateTasksWorkspace(t, "demo")
	withWorkingDir(t, workspaceRoot)

	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-form-overrides-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeUI,
			StartedAt:        time.Date(2026, 4, 20, 12, 5, 0, 0, time.UTC),
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	var attachedRunID string
	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		attachedRunID = runID
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	defaults.collectForm = func(cmd *cobra.Command, state *commandState) error {
		inputs := &formInputs{
			name:            "demo",
			ide:             "claude",
			model:           "gpt-5.5",
			reasoningEffort: "high",
			autoCommit:      true,
		}
		inputs.apply(cmd, state)
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "tasks", "run")
	if err != nil {
		t.Fatalf("execute interactive tasks run: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if attachedRunID != "run-task-form-overrides-001" {
		t.Fatalf("expected UI attach for run-task-form-overrides-001, got %q", attachedRunID)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout before ui attach, got %q", stdout)
	}

	var overrides daemonRuntimeOverrides
	if err := json.Unmarshal(client.startRequest.RuntimeOverrides, &overrides); err != nil {
		t.Fatalf("decode task runtime overrides: %v", err)
	}
	if overrides.AutoCommit == nil || !*overrides.AutoCommit {
		t.Fatalf("expected auto-commit form override, got %#v", overrides)
	}
	if overrides.IDE == nil || *overrides.IDE != "claude" {
		t.Fatalf("expected ide form override, got %#v", overrides)
	}
	if overrides.Model == nil || *overrides.Model != "gpt-5.5" {
		t.Fatalf("expected model form override, got %#v", overrides)
	}
	if overrides.ReasoningEffort == nil || *overrides.ReasoningEffort != "high" {
		t.Fatalf("expected reasoning-effort form override, got %#v", overrides)
	}
}

func TestTasksRunInteractiveFormPropagatesTaskRuntimeRulesWithoutExplicitFlagMutation(t *testing.T) {
	workspaceRoot, _ := makeValidateTasksWorkspace(t, "demo")
	withWorkingDir(t, workspaceRoot)

	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-form-task-runtime-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeUI,
			StartedAt:        time.Date(2026, 4, 20, 12, 6, 0, 0, time.UTC),
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		if runID != "run-task-form-task-runtime-001" {
			t.Fatalf("unexpected attached run id: %q", runID)
		}
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	defaults.collectForm = func(cmd *cobra.Command, state *commandState) error {
		inputs := &formInputs{
			name:            "demo",
			ide:             "codex",
			model:           "gpt-5.5",
			reasoningEffort: "medium",
		}
		inputs.apply(cmd, state)
		if cmd.Flags().Changed("task-runtime") {
			t.Fatal("expected task-runtime flag to remain implicit in the simulated interactive flow")
		}
		state.replaceConfiguredTaskRunRules = true
		state.executionTaskRuntimeRules = []model.TaskRuntimeRule{
			{
				Type:            stringPointer("backend"),
				IDE:             stringPointer("claude"),
				Model:           stringPointer("sonnet-4.5"),
				ReasoningEffort: stringPointer("high"),
			},
			{
				ID:    stringPointer("task_02"),
				IDE:   stringPointer("codex"),
				Model: stringPointer("codex-fast"),
			},
		}
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "tasks", "run")
	if err != nil {
		t.Fatalf(
			"execute interactive tasks run with task runtime rules: %v\nstdout:\n%s\nstderr:\n%s",
			err,
			stdout,
			stderr,
		)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout before ui attach, got %q", stdout)
	}

	var overrides daemonRuntimeOverrides
	if err := json.Unmarshal(client.startRequest.RuntimeOverrides, &overrides); err != nil {
		t.Fatalf("decode task runtime overrides: %v", err)
	}
	if overrides.TaskRuntimeRules == nil || len(*overrides.TaskRuntimeRules) != 2 {
		t.Fatalf("expected two task runtime rules from interactive form state, got %#v", overrides.TaskRuntimeRules)
	}

	typeRule := (*overrides.TaskRuntimeRules)[0]
	if typeRule.Type == nil || *typeRule.Type != "backend" {
		t.Fatalf("expected first task runtime rule to target backend, got %#v", typeRule)
	}
	if typeRule.IDE == nil || *typeRule.IDE != "claude" || typeRule.Model == nil || *typeRule.Model != "sonnet-4.5" {
		t.Fatalf("expected first task runtime rule to preserve interactive overrides, got %#v", typeRule)
	}

	taskRule := (*overrides.TaskRuntimeRules)[1]
	if taskRule.ID == nil || *taskRule.ID != "task_02" {
		t.Fatalf("expected second task runtime rule to target task_02, got %#v", taskRule)
	}
	if taskRule.Model == nil || *taskRule.Model != "codex-fast" {
		t.Fatalf("expected second task runtime rule to preserve task-specific model override, got %#v", taskRule)
	}
}

func TestTasksRunInteractiveFormClearsConfiguredTaskRuntimeRulesExplicitly(t *testing.T) {
	workspaceRoot, _ := makeValidateTasksWorkspace(t, "demo")
	writeCLIWorkspaceConfig(t, workspaceRoot, `
[tasks.run]
  [[tasks.run.task_runtime_rules]]
  type = "backend"
  ide = "claude"
  model = "sonnet-4.5"
`)
	withWorkingDir(t, workspaceRoot)

	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-form-task-runtime-clear-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeUI,
			StartedAt:        time.Date(2026, 4, 20, 12, 7, 0, 0, time.UTC),
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		if runID != "run-task-form-task-runtime-clear-001" {
			t.Fatalf("unexpected attached run id: %q", runID)
		}
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	defaults.collectForm = func(cmd *cobra.Command, state *commandState) error {
		inputs := &formInputs{name: "demo"}
		inputs.apply(cmd, state)
		if cmd.Flags().Changed("task-runtime") {
			t.Fatal("expected task-runtime flag to remain implicit in the simulated interactive clear flow")
		}
		state.replaceConfiguredTaskRunRules = true
		state.executionTaskRuntimeRules = nil
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "tasks", "run")
	if err != nil {
		t.Fatalf(
			"execute interactive tasks run with cleared task runtime rules: %v\nstdout:\n%s\nstderr:\n%s",
			err,
			stdout,
			stderr,
		)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout before ui attach, got %q", stdout)
	}
	if !strings.Contains(string(client.startRequest.RuntimeOverrides), `"task_runtime_rules":[]`) {
		t.Fatalf(
			"expected interactive clear to send an explicit empty task_runtime_rules override, got %s",
			client.startRequest.RuntimeOverrides,
		)
	}

	var overrides daemonRuntimeOverrides
	if err := json.Unmarshal(client.startRequest.RuntimeOverrides, &overrides); err != nil {
		t.Fatalf("decode task runtime overrides: %v", err)
	}
	if overrides.TaskRuntimeRules == nil {
		t.Fatalf("expected cleared interactive task runtime selection to remain explicit, got %#v", overrides)
	}
	if len(*overrides.TaskRuntimeRules) != 0 {
		t.Fatalf(
			"expected cleared interactive task runtime selection to send zero rules, got %#v",
			*overrides.TaskRuntimeRules,
		)
	}
}

func TestReviewsFixInteractiveFormBlankBatchingPreservesWorkspaceDefaults(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, workspaceRoot, `
[fix_reviews]
concurrent = 4
batch_size = 7
`)
	withWorkingDir(t, workspaceRoot)

	client := &reviewExecCaptureClient{
		stubDaemonCommandClient: &stubDaemonCommandClient{
			health: apicore.DaemonHealth{Ready: true},
			reviewRun: apicore.Run{
				RunID:            "run-review-form-blank-batching-001",
				Mode:             "review",
				Status:           "running",
				PresentationMode: attachModeUI,
				StartedAt:        time.Date(2026, 4, 20, 12, 10, 0, 0, time.UTC),
			},
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		if runID != "run-review-form-blank-batching-001" {
			t.Fatalf("unexpected attached run id: %q", runID)
		}
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	defaults.collectForm = func(cmd *cobra.Command, state *commandState) error {
		inputs := newFormInputsFromState(state)
		inputs.name = "demo"
		inputs.round = "1"
		inputs.concurrent = ""
		inputs.batchSize = ""
		inputs.apply(cmd, state)
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "reviews", "fix")
	if err != nil {
		t.Fatalf("execute interactive reviews fix blank batching: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected quiet form flow before ui attach, got stdout=%q stderr=%q", stdout, stderr)
	}
	var batching map[string]any
	if err := json.Unmarshal(client.startReviewReq.Batching, &batching); err != nil {
		t.Fatalf("decode review batching payload: %v", err)
	}
	if _, ok := batching["batch_size"]; ok {
		t.Fatalf("expected blank batch-size to stay implicit, got %#v", batching)
	}
	if _, ok := batching["concurrent"]; ok {
		t.Fatalf("expected blank concurrent to stay implicit, got %#v", batching)
	}
}

func TestTasksRunInteractiveFormBlankAddDirsPreservesWorkspaceDefaults(t *testing.T) {
	workspaceRoot, _ := makeValidateTasksWorkspace(t, "demo")
	writeCLIWorkspaceConfig(t, workspaceRoot, `
[defaults]
add_dirs = ["../shared", "../docs"]
`)
	withWorkingDir(t, workspaceRoot)

	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		startRun: apicore.Run{
			RunID:            "run-task-form-blank-adddir-001",
			Mode:             string(core.ModePRDTasks),
			Status:           "running",
			PresentationMode: attachModeUI,
			StartedAt:        time.Date(2026, 4, 20, 12, 15, 0, 0, time.UTC),
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	installTestCLIRunObservers(t, func(_ context.Context, _ daemonCommandClient, runID string) error {
		if runID != "run-task-form-blank-adddir-001" {
			t.Fatalf("unexpected attached run id: %q", runID)
		}
		return nil
	}, nil)

	defaults := allowBundledSkillsForExecutionTests()
	defaults.isInteractive = func() bool { return true }
	defaults.collectForm = func(cmd *cobra.Command, state *commandState) error {
		inputs := newFormInputsFromState(state)
		inputs.name = "demo"
		inputs.addDirs = ""
		inputs.apply(cmd, state)
		return nil
	}

	cmd := newRootCommandWithDefaults(newLazyRootDispatcher(), defaults)
	stdout, stderr, err := executeCommandCapturingProcessIO(t, cmd, nil, "tasks", "run")
	if err != nil {
		t.Fatalf("execute interactive tasks run blank add-dir: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout before ui attach, got %q", stdout)
	}
	if !strings.Contains(stderr, "preflight=ok") {
		t.Fatalf("expected preflight success log on stderr, got %q", stderr)
	}

	var overrides daemonRuntimeOverrides
	if err := json.Unmarshal(client.startRequest.RuntimeOverrides, &overrides); err != nil {
		t.Fatalf("decode task runtime overrides: %v", err)
	}
	if overrides.AddDirs != nil {
		t.Fatalf("expected blank add-dir to stay implicit, got %#v", overrides)
	}
}
