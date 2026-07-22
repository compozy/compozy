package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestPrepareExecutionConfigRunPreStartRejectsPreparedStateMutation(t *testing.T) {
	t.Parallel()

	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"run.pre_start": func(input any) (any, error) {
				payload := input.(runPreStartPayload)
				payload.Config.Model = "codex-fast"
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		WorkspaceRoot:          "/tmp/workspace",
		Name:                   "demo",
		TasksDir:               "/tmp/workspace/.compozy/tasks/demo",
		Mode:                   model.ExecutionModePRDTasks,
		IDE:                    model.IDECodex,
		Model:                  "gpt-5.5",
		ReasoningEffort:        "medium",
		AccessMode:             model.AccessModeFull,
		Timeout:                time.Minute,
		RetryBackoffMultiplier: 1.5,
	}

	_, err := prepareExecutionConfig(
		context.Background(),
		cfg,
		model.RunArtifacts{RunID: "run-1"},
		manager,
	)
	if err == nil {
		t.Fatal("prepareExecutionConfig error = nil, want prepared-state mutation failure")
	}
	if !strings.Contains(err.Error(), "run.pre_start cannot mutate model after workflow state preparation") {
		t.Fatalf("prepareExecutionConfig error = %q, want model mutation guard", err.Error())
	}
}

func TestPrepareExecutionConfigRunPreStartRejectsComplexityRuleMutation(t *testing.T) {
	t.Run("Should reject complexity rule mutation after workflow state preparation", func(t *testing.T) {
		t.Parallel()

		low := "low"
		high := "high"
		manager := &executionHookManager{
			mutators: map[string]func(any) (any, error){
				"run.pre_start": func(input any) (any, error) {
					payload := input.(runPreStartPayload)
					payload.Config.TaskRuntimeRules[0].Complexity = &high
					return payload, nil
				},
			},
		}

		cfg := &model.RuntimeConfig{
			WorkspaceRoot:          "/tmp/workspace",
			Name:                   "demo",
			TasksDir:               "/tmp/workspace/.compozy/tasks/demo",
			Mode:                   model.ExecutionModePRDTasks,
			IDE:                    model.IDECodex,
			Model:                  "gpt-5.5",
			ReasoningEffort:        "medium",
			AccessMode:             model.AccessModeFull,
			Timeout:                time.Minute,
			RetryBackoffMultiplier: 1.5,
			TaskRuntimeRules: []model.TaskRuntimeRule{{
				Complexity: &low,
			}},
		}

		_, err := prepareExecutionConfig(
			context.Background(),
			cfg,
			model.RunArtifacts{RunID: "run-1"},
			manager,
		)
		if err == nil {
			t.Fatal("prepareExecutionConfig error = nil, want task-runtime mutation failure")
		}
		if !strings.Contains(
			err.Error(),
			"run.pre_start cannot mutate task_runtime_rules after workflow state preparation",
		) {
			t.Fatalf("prepareExecutionConfig error = %q, want task-runtime mutation guard", err.Error())
		}
	})
}

func TestPrepareExecutionConfigRunPreStartAllowsLateMutableFields(t *testing.T) {
	t.Parallel()

	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"run.pre_start": func(input any) (any, error) {
				payload := input.(runPreStartPayload)
				payload.Config.Timeout = 2 * time.Minute
				payload.Config.MaxRetries = 4
				payload.Config.SoundEnabled = true
				payload.Config.SoundOnCompleted = "hero"
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		WorkspaceRoot:          "/tmp/workspace",
		Name:                   "demo",
		TasksDir:               "/tmp/workspace/.compozy/tasks/demo",
		Mode:                   model.ExecutionModePRDTasks,
		IDE:                    model.IDECodex,
		Model:                  "gpt-5.5",
		ReasoningEffort:        "medium",
		AccessMode:             model.AccessModeFull,
		Timeout:                time.Minute,
		MaxRetries:             1,
		RetryBackoffMultiplier: 1.5,
	}

	internalCfg, err := prepareExecutionConfig(
		context.Background(),
		cfg,
		model.RunArtifacts{RunID: "run-1"},
		manager,
	)
	if err != nil {
		t.Fatalf("prepareExecutionConfig: %v", err)
	}

	if got := internalCfg.Timeout; got != 2*time.Minute {
		t.Fatalf("internal timeout = %v, want %v", got, 2*time.Minute)
	}
	if got := internalCfg.MaxRetries; got != 4 {
		t.Fatalf("internal max retries = %d, want %d", got, 4)
	}
	if !internalCfg.SoundEnabled {
		t.Fatal("expected sound_enabled mutation to apply")
	}
	if got := internalCfg.SoundOnCompleted; got != "hero" {
		t.Fatalf("internal sound_on_completed = %q, want %q", got, "hero")
	}
}

func TestHookRuntimeConfigRoundTripsStallPolicy(t *testing.T) {
	t.Parallel()

	want := model.StallPolicy{
		Enabled:      false,
		IdleTimeout:  4 * time.Minute,
		ChildTimeout: 9 * time.Minute,
		TerminalCap:  30 * time.Minute,
		Retries:      0,
	}
	hookConfig := hookRuntimeConfig(&config{Stall: want})

	if hookConfig.StallEnabled == nil || *hookConfig.StallEnabled {
		t.Fatalf("hook stall enabled = %#v, want explicit false", hookConfig.StallEnabled)
	}
	if got := hookConfig.StallTimeout; got != want.IdleTimeout {
		t.Fatalf("hook stall timeout = %v, want %v", got, want.IdleTimeout)
	}
	if got := hookConfig.ChildStallTimeout; got != want.ChildTimeout {
		t.Fatalf("hook child stall timeout = %v, want %v", got, want.ChildTimeout)
	}
	if got := hookConfig.TerminalCommandTimeout; got != want.TerminalCap {
		t.Fatalf("hook terminal command timeout = %v, want %v", got, want.TerminalCap)
	}
	if hookConfig.StallRetries == nil || *hookConfig.StallRetries != 0 {
		t.Fatalf("hook stall retries = %#v, want explicit 0", hookConfig.StallRetries)
	}

	destination := &config{}
	applyHookRuntimeConfig(destination, hookConfig)
	if got := destination.Stall; got != want {
		t.Fatalf("round-tripped stall policy = %#v, want %#v", got, want)
	}
}

func TestPrepareExecutionConfigRunPreStartMutatesStallPolicy(t *testing.T) {
	t.Parallel()

	enabled := true
	retries := 2
	var observed model.RuntimeConfig
	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"run.pre_start": func(input any) (any, error) {
				payload := input.(runPreStartPayload)
				observed = payload.Config
				disabled := false
				zeroRetries := 0
				payload.Config.StallEnabled = &disabled
				payload.Config.StallTimeout = 5 * time.Minute
				payload.Config.ChildStallTimeout = 12 * time.Minute
				payload.Config.TerminalCommandTimeout = 40 * time.Minute
				payload.Config.StallRetries = &zeroRetries
				return payload, nil
			},
		},
	}
	cfg := &model.RuntimeConfig{
		StallEnabled:           &enabled,
		StallTimeout:           3 * time.Minute,
		ChildStallTimeout:      8 * time.Minute,
		TerminalCommandTimeout: 45 * time.Minute,
		StallRetries:           &retries,
	}

	internalCfg, err := prepareExecutionConfig(
		context.Background(),
		cfg,
		model.RunArtifacts{RunID: "run-1"},
		manager,
	)
	if err != nil {
		t.Fatalf("prepareExecutionConfig: %v", err)
	}

	if observed.StallEnabled == nil || !*observed.StallEnabled {
		t.Fatalf("observed stall enabled = %#v, want explicit true", observed.StallEnabled)
	}
	if got := observed.StallTimeout; got != 3*time.Minute {
		t.Fatalf("observed stall timeout = %v, want %v", got, 3*time.Minute)
	}
	if got := observed.ChildStallTimeout; got != 8*time.Minute {
		t.Fatalf("observed child stall timeout = %v, want %v", got, 8*time.Minute)
	}
	if got := observed.TerminalCommandTimeout; got != 45*time.Minute {
		t.Fatalf("observed terminal command timeout = %v, want %v", got, 45*time.Minute)
	}
	if observed.StallRetries == nil || *observed.StallRetries != 2 {
		t.Fatalf("observed stall retries = %#v, want explicit 2", observed.StallRetries)
	}

	want := model.StallPolicy{
		Enabled:      false,
		IdleTimeout:  5 * time.Minute,
		ChildTimeout: 12 * time.Minute,
		TerminalCap:  40 * time.Minute,
		Retries:      0,
	}
	if got := internalCfg.Stall; got != want {
		t.Fatalf("mutated stall policy = %#v, want %#v", got, want)
	}
}

func TestPrepareExecutionConfigRunPreStartCorrectsNestedStallBudget(t *testing.T) {
	t.Parallel()

	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"run.pre_start": func(input any) (any, error) {
				payload := input.(runPreStartPayload)
				payload.Config.StallTimeout = 10 * time.Minute
				payload.Config.ChildStallTimeout = 5 * time.Minute
				return payload, nil
			},
		},
	}
	cfg := &model.RuntimeConfig{
		StallTimeout:      3 * time.Minute,
		ChildStallTimeout: 8 * time.Minute,
	}

	internalCfg, err := prepareExecutionConfig(
		context.Background(),
		cfg,
		model.RunArtifacts{RunID: "run-1"},
		manager,
	)
	if err != nil {
		t.Fatalf("prepareExecutionConfig: %v", err)
	}

	if got := internalCfg.Stall.IdleTimeout; got != 10*time.Minute {
		t.Fatalf("mutated stall timeout = %v, want %v", got, 10*time.Minute)
	}
	if got := internalCfg.Stall.ChildTimeout; got != 20*time.Minute {
		t.Fatalf("corrected child stall timeout = %v, want %v", got, 20*time.Minute)
	}
}

func TestJobRunnerDispatchPreExecuteRejectsRuntimeMutation(t *testing.T) {
	t.Parallel()

	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"job.pre_execute": func(input any) (any, error) {
				payload := input.(jobPreExecutePayload)
				payload.Job.Model = "codex-fast"
				return payload, nil
			},
		},
	}

	execCfg := &config{
		RuntimeManager: manager,
		RunArtifacts:   model.RunArtifacts{RunID: "run-1"},
	}
	runner := &jobRunner{
		job: &job{
			SafeName:        "task_01",
			IDE:             model.IDECodex,
			Model:           "gpt-5.5",
			ReasoningEffort: "medium",
		},
		cfg:     execCfg,
		execCtx: &jobExecutionContext{cfg: execCfg},
	}

	err := runner.dispatchPreExecuteHook(context.Background())
	if err == nil {
		t.Fatal("dispatchPreExecuteHook error = nil, want runtime mutation failure")
	}
	if !strings.Contains(err.Error(), "job.pre_execute cannot mutate job runtime after planning completed") {
		t.Fatalf("dispatchPreExecuteHook error = %q, want runtime mutation guard", err.Error())
	}
}

func TestJobRunnerDispatchPreExecuteRejectsWhitespaceOnlyRuntimeMutation(t *testing.T) {
	t.Parallel()

	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"job.pre_execute": func(input any) (any, error) {
				payload := input.(jobPreExecutePayload)
				payload.Job.IDE += " "
				return payload, nil
			},
		},
	}

	execCfg := &config{
		RuntimeManager: manager,
		RunArtifacts:   model.RunArtifacts{RunID: "run-1"},
	}
	runner := &jobRunner{
		job: &job{
			SafeName:        "task_01",
			IDE:             model.IDECodex,
			Model:           "gpt-5.5",
			ReasoningEffort: "medium",
		},
		cfg:     execCfg,
		execCtx: &jobExecutionContext{cfg: execCfg},
	}

	err := runner.dispatchPreExecuteHook(context.Background())
	if err == nil {
		t.Fatal("dispatchPreExecuteHook error = nil, want whitespace mutation failure")
	}
	if !strings.Contains(err.Error(), "job.pre_execute cannot mutate job runtime after planning completed") {
		t.Fatalf("dispatchPreExecuteHook error = %q, want runtime mutation guard", err.Error())
	}
}
