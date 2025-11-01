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

// RunSimpleWorkflow executes a minimal workflow using a mock model-backed agent.
func RunSimpleWorkflow(ctx context.Context) error {
	agentCfg, workflowCfg, err := buildSimpleWorkflow(ctx)
	if err != nil {
		return err
	}
	options := []compozy.Option{
		compozy.WithAgent(agentCfg),
		compozy.WithWorkflow(workflowCfg),
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		request := &compozy.ExecuteSyncRequest{Input: map[string]any{"name": "Casey"}}
		response, err := engine.ExecuteWorkflowSync(execCtx, workflowCfg.ID, request)
		if err != nil {
			return fmt.Errorf("execute workflow: %w", err)
		}
		message := stringOutput(response.Output, "greeting")
		if message == "" {
			message = stringOutput(response.Output, enginecore.OutputRootKey)
		}
		fmt.Printf("Simple workflow greeting: %s\n", message)
		logger.FromContext(execCtx).Info("simple workflow completed", "greeting", message)
		return nil
	})
}

func buildSimpleWorkflow(ctx context.Context) (*engineagent.Config, *engineworkflow.Config, error) {
	model := &engineagent.Model{
		Config: enginecore.ProviderConfig{Provider: enginecore.ProviderMock, Model: "mock-greeting"},
	}
	agentCfg, err := newAgentWithModel(ctx, "friendly-assistant", "You create short, warm greetings.", model,
		sdkagent.WithMaxIterations(1),
	)
	if err != nil {
		return nil, nil, err
	}
	taskCfg, err := sdktask.New(ctx, "compose-greeting",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt("Greet {{ .workflow.input.name }} in a single cheerful sentence."),
		sdktask.WithOutputs(inputPtr(map[string]any{"message": "{{ .task.output }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(ctx, "simple-workflow",
		sdkworkflow.WithDescription("Greets a user with a friendly message"),
		sdkworkflow.WithTasks([]enginetask.Config{*taskCfg}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{"greeting": "{{ .tasks.compose-greeting.output.message }}"})),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create workflow: %w", err)
	}
	return agentCfg, workflowCfg, nil
}
