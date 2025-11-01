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

// RunParallelTasks demonstrates fan-out/fan-in execution with parallel tasks and aggregation.
func RunParallelTasks(ctx context.Context) error {
	agentCfg, workflowCfg, err := buildParallelWorkflow(ctx)
	if err != nil {
		return err
	}
	options := []compozy.Option{
		compozy.WithAgent(agentCfg),
		compozy.WithWorkflow(workflowCfg),
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		input := map[string]any{"project": "Helios"}
		resp, err := engine.ExecuteWorkflowSync(execCtx, workflowCfg.ID, &compozy.ExecuteSyncRequest{Input: input})
		if err != nil {
			return fmt.Errorf("execute workflow: %w", err)
		}
		summary := stringOutput(resp.Output, "summary")
		fmt.Printf("Parallel execution summary:\n%s\n", summary)
		logger.FromContext(execCtx).Info("parallel workflow completed", "summary", summary)
		return nil
	})
}

func buildParallelWorkflow(ctx context.Context) (*engineagent.Config, *engineworkflow.Config, error) {
	model := &engineagent.Model{
		Config: enginecore.ProviderConfig{Provider: enginecore.ProviderMock, Model: "mock-updates"},
	}
	agentCfg, err := newAgentWithModel(ctx, "status-reporter", "Summarize team progress in one sentence.", model,
		sdkagent.WithMaxIterations(1),
	)
	if err != nil {
		return nil, nil, err
	}
	branches, err := buildBranches(ctx, agentCfg)
	if err != nil {
		return nil, nil, err
	}
	next := "summarize-updates"
	parallelCfg, err := sdktask.NewParallel(ctx, "collect-updates", branches,
		sdktask.WithStrategy(enginetask.StrategyWaitAll),
		sdktask.WithOnSuccess(&enginecore.SuccessTransition{Next: &next}),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create parallel task: %w", err)
	}
	summaryCfg, err := sdktask.New(
		ctx,
		next,
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt(
			"Project {{ .workflow.input.project }} is underway. Draft a concise summary using these team updates:\n"+
				"- Research: {{ .tasks.research-update.output.status }}\n"+
				"- Product: {{ .tasks.product-update.output.status }}\n"+
				"- Support: {{ .tasks.support-update.output.status }}",
		),
		sdktask.WithOutputs(inputPtr(map[string]any{"summary": "{{ .task.output }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create summary task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(ctx, "parallel-tasks",
		sdkworkflow.WithDescription("Runs three updates in parallel and aggregates results"),
		sdkworkflow.WithTasks([]enginetask.Config{*parallelCfg, *summaryCfg}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{"summary": "{{ .tasks.summarize-updates.output.summary }}"})),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create workflow: %w", err)
	}
	return agentCfg, workflowCfg, nil
}

func buildBranches(ctx context.Context, agentCfg *engineagent.Config) ([]enginetask.Config, error) {
	definitions := []struct {
		id     string
		prompt string
	}{
		{"research-update", "Provide a research milestone update for project {{ .workflow.input.project }}."},
		{"product-update", "Summarize product development progress for project {{ .workflow.input.project }}."},
		{"support-update", "Report a customer support highlight for project {{ .workflow.input.project }}."},
	}
	branches := make([]enginetask.Config, 0, len(definitions))
	for _, spec := range definitions {
		taskCfg, err := sdktask.New(ctx, spec.id,
			sdktask.WithAgent(agentCfg),
			sdktask.WithPrompt(spec.prompt),
			sdktask.WithOutputs(inputPtr(map[string]any{"status": "{{ .task.output }}"})),
		)
		if err != nil {
			return nil, fmt.Errorf("create branch %s: %w", spec.id, err)
		}
		branches = append(branches, *taskCfg)
	}
	return branches, nil
}
