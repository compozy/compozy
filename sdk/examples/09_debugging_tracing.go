package main

import (
	"context"
	"encoding/json"
	"fmt"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkagent "github.com/compozy/compozy/sdk/v2/agent"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	sdktask "github.com/compozy/compozy/sdk/v2/task"
	sdkworkflow "github.com/compozy/compozy/sdk/v2/workflow"
)

// RunDebuggingAndTracing executes a workflow and prints execution metadata for debugging.
func RunDebuggingAndTracing(ctx context.Context) error {
	agentCfg, workflowCfg, err := buildDebugWorkflow(ctx)
	if err != nil {
		return err
	}
	options := []compozy.Option{
		compozy.WithAgent(agentCfg),
		compozy.WithWorkflow(workflowCfg),
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		resp, err := engine.ExecuteWorkflowSync(execCtx, workflowCfg.ID, &compozy.ExecuteSyncRequest{})
		if err != nil {
			return fmt.Errorf("execute workflow: %w", err)
		}
		telemetry := map[string]any{
			"exec_id":     resp.ExecID,
			"output":      resp.Output,
			"workflow_id": workflowCfg.ID,
		}
		blob, err := json.MarshalIndent(telemetry, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal telemetry: %w", err)
		}
		fmt.Printf("Telemetry snapshot:\n%s\n", string(blob))
		logger.FromContext(execCtx).Info("debug workflow completed", "exec_id", resp.ExecID)
		return nil
	})
}

func buildDebugWorkflow(ctx context.Context) (*engineagent.Config, *engineworkflow.Config, error) {
	agentModel := &engineagent.Model{
		Config: enginecore.ProviderConfig{Provider: enginecore.ProviderMock, Model: "mock-debug"},
	}
	agentCfg, err := newAgentWithModel(ctx, "debug-assistant", "Summarize diagnostic context succinctly.", agentModel,
		sdkagent.WithMaxIterations(1),
	)
	if err != nil {
		return nil, nil, err
	}
	inspectTask, err := sdktask.New(ctx, "collect-diagnostics",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt("Provide a sentence describing that debugging instrumentation is active."),
		sdktask.WithOutputs(inputPtr(map[string]any{"details": "{{ .task.output }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create debug task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(
		ctx,
		"debugging-and-tracing",
		sdkworkflow.WithDescription("Captures execution metadata for troubleshooting"),
		sdkworkflow.WithTasks([]enginetask.Config{*inspectTask}),
		sdkworkflow.WithOutputs(
			outputPtr(map[string]any{"diagnostics": "{{ .tasks.collect-diagnostics.output.details }}"}),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create debug workflow: %w", err)
	}
	return agentCfg, workflowCfg, nil
}
