package main

import (
	"context"
	"fmt"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkagent "github.com/compozy/compozy/sdk/v2/agent"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	sdktask "github.com/compozy/compozy/sdk/v2/task"
	sdkworkflow "github.com/compozy/compozy/sdk/v2/workflow"
)

// RunScheduledWorkflow simulates a schedule tick and executes the registered workflow once.
func RunScheduledWorkflow(ctx context.Context) error {
	agentCfg, workflowCfg, err := buildScheduledWorkflow(ctx)
	if err != nil {
		return err
	}
	tickTime := time.Now().UTC().Format(time.RFC3339Nano)
	scheduleCfg := &projectschedule.Config{
		ID:          "demo-schedule",
		WorkflowID:  workflowCfg.ID,
		Cron:        "*/5 * * * *",
		Description: "Runs every five minutes; this example triggers it manually once.",
		Input:       map[string]any{"report": "morning", "tick_time": tickTime},
	}
	options := []compozy.Option{
		compozy.WithAgent(agentCfg),
		compozy.WithWorkflow(workflowCfg),
		compozy.WithSchedule(scheduleCfg),
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		log := logger.FromContext(execCtx)
		log.Info("simulating first schedule tick", "schedule", scheduleCfg.ID)
		resp, err := engine.ExecuteWorkflowSync(
			execCtx,
			workflowCfg.ID,
			&compozy.ExecuteSyncRequest{Input: scheduleCfg.Input},
		)
		if err != nil {
			return fmt.Errorf("execute scheduled workflow: %w", err)
		}
		report := stringOutput(resp.Output, "summary")
		fmt.Printf("Schedule tick at %s generated summary:\n%s\n", tickTime, report)
		log.Info("schedule run completed", "summary", report)
		return nil
	})
}

func buildScheduledWorkflow(ctx context.Context) (*engineagent.Config, *engineworkflow.Config, error) {
	agentModel := &engineagent.Model{
		Config: enginecore.ProviderConfig{Provider: enginecore.ProviderMock, Model: "mock-schedule"},
	}
	agentCfg, err := newAgentWithModel(ctx, "status-writer", "Produce short operational updates.", agentModel,
		sdkagent.WithMaxIterations(1),
	)
	if err != nil {
		return nil, nil, err
	}
	statusTask, err := sdktask.New(ctx, "generate-status",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt("Create a one-line status update for the {{ .workflow.input.report }} shift."),
		sdktask.WithOutputs(inputPtr(map[string]any{"note": "{{ .task.output }}"})),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create status task: %w", err)
	}
	summaryTask, err := sdktask.New(
		ctx,
		"wrap-up",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt("Compose a handoff summary referencing '{{ .tasks.generate-status.output.note }}'."),
		sdktask.WithOutputs(
			inputPtr(map[string]any{"summary": "{{ .task.output }}", "timestamp": "{{ .workflow.input.tick_time }}"}),
		),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create summary task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(ctx, "scheduled-workflow",
		sdkworkflow.WithDescription("Produces a shift summary for the schedule tick"),
		sdkworkflow.WithTasks([]enginetask.Config{*statusTask, *summaryTask}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{
			"summary":      "{{ .tasks.wrap-up.output.summary }}",
			"generated_at": "{{ .tasks.wrap-up.output.timestamp }}",
		})),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create schedule workflow: %w", err)
	}
	return agentCfg, workflowCfg, nil
}
