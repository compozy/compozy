package main

import (
	"context"
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

// RunModelRouting shows how per-task model overrides change execution strategy.
func RunModelRouting(ctx context.Context) error {
	agentCfg, workflowCfg, err := buildModelRoutingWorkflow(ctx)
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
			return fmt.Errorf("execute routing workflow: %w", err)
		}
		primary := stringOutput(resp.Output, "primary_model")
		secondary := stringOutput(resp.Output, "secondary_model")
		fmt.Printf("Primary task used: %s\nFallback task used: %s\n", primary, secondary)
		logger.FromContext(execCtx).Info("model routing completed", "primary", primary, "secondary", secondary)
		return nil
	})
}

func buildModelRoutingWorkflow(ctx context.Context) (*engineagent.Config, *engineworkflow.Config, error) {
	defaultModel := &engineagent.Model{
		Config: enginecore.ProviderConfig{Provider: enginecore.ProviderMock, Model: "mock-primary"},
	}
	agentCfg, err := newAgentWithModel(
		ctx,
		"router-agent",
		"Mention the supplied model name in your reply.",
		defaultModel,
		sdkagent.WithMaxIterations(1),
	)
	if err != nil {
		return nil, nil, err
	}
	primaryTask, err := sdktask.New(ctx, "primary-route",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt("You are using model mock-primary. Reply with 'Primary via mock-primary'."),
		sdktask.WithOutputs(inputPtr(map[string]any{"model_used": "{{ .task.output }}"})),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create primary task: %w", err)
	}
	secondaryModel := enginecore.ProviderConfig{Provider: enginecore.ProviderMock, Model: "mock-secondary"}
	secondaryTask, err := sdktask.New(ctx, "secondary-route",
		sdktask.WithAgent(agentCfg),
		sdktask.WithModelConfig(secondaryModel),
		sdktask.WithPrompt("You are using model mock-secondary. Reply with 'Fallback via mock-secondary'."),
		sdktask.WithOutputs(inputPtr(map[string]any{"model_used": "{{ .task.output }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create secondary task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(ctx, "model-routing",
		sdkworkflow.WithDescription("Demonstrates per-task model overrides"),
		sdkworkflow.WithTasks([]enginetask.Config{*primaryTask, *secondaryTask}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{
			"primary_model":   "{{ .tasks.primary-route.output.model_used }}",
			"secondary_model": "{{ .tasks.secondary-route.output.model_used }}",
		})),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create routing workflow: %w", err)
	}
	return agentCfg, workflowCfg, nil
}
