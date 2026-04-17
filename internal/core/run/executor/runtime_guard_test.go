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
				payload.Config.Model = "gpt-5.4-mini"
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
		Model:                  "gpt-5.4",
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
		Model:                  "gpt-5.4",
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

func TestJobRunnerDispatchPreExecuteRejectsRuntimeMutation(t *testing.T) {
	t.Parallel()

	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"job.pre_execute": func(input any) (any, error) {
				payload := input.(jobPreExecutePayload)
				payload.Job.Model = "gpt-5.4-mini"
				return payload, nil
			},
		},
	}

	runner := &jobRunner{
		job: &job{
			SafeName:        "task_01",
			IDE:             model.IDECodex,
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
		},
		execCtx: &jobExecutionContext{
			cfg: &config{
				RuntimeManager: manager,
				RunArtifacts:   model.RunArtifacts{RunID: "run-1"},
			},
		},
	}

	err := runner.dispatchPreExecuteHook(context.Background())
	if err == nil {
		t.Fatal("dispatchPreExecuteHook error = nil, want runtime mutation failure")
	}
	if !strings.Contains(err.Error(), "job.pre_execute cannot mutate job runtime after planning completed") {
		t.Fatalf("dispatchPreExecuteHook error = %q, want runtime mutation guard", err.Error())
	}
}
