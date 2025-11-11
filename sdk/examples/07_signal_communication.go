package main

import (
	"context"
	"fmt"

	enginecore "github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	sdktask "github.com/compozy/compozy/sdk/v2/task"
	sdkworkflow "github.com/compozy/compozy/sdk/v2/workflow"
)

// RunSignalCommunication demonstrates signal emission and wait coordination within a workflow.
func RunSignalCommunication(ctx context.Context) error {
	workflowCfg, err := buildSignalWorkflow(ctx)
	if err != nil {
		return err
	}
	options := []compozy.Option{compozy.WithWorkflow(workflowCfg)}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		input := map[string]any{"request_id": "42", "message": "Data sync completed"}
		req := &compozy.ExecuteSyncRequest{Input: input}
		resp, err := engine.ExecuteWorkflowSync(execCtx, workflowCfg.ID, req)
		if err != nil {
			return fmt.Errorf("execute signal workflow: %w", err)
		}
		received := stringOutput(resp.Output, "acknowledged")
		fmt.Printf("Wait task received payload: %s\n", received)
		logger.FromContext(execCtx).Info("signal workflow completed", "payload", received)
		return nil
	})
}

func buildSignalWorkflow(ctx context.Context) (*engineworkflow.Config, error) {
	next := "await-signal"
	signalTask, err := sdktask.New(ctx, "publish-signal",
		sdktask.WithType(enginetask.TaskTypeSignal),
		sdktask.WithSignal(&enginetask.SignalConfig{
			ID: "request-{{ .workflow.input.request_id }}-done",
			Payload: map[string]any{
				"message":    "{{ .workflow.input.message }}",
				"request_id": "{{ .workflow.input.request_id }}",
			},
		}),
		sdktask.WithOnSuccess(&enginecore.SuccessTransition{Next: &next}),
	)
	if err != nil {
		return nil, fmt.Errorf("create signal task: %w", err)
	}
	waitTask, err := sdktask.New(ctx, next,
		sdktask.WithType(enginetask.TaskTypeWait),
		sdktask.WithWaitFor("request-{{ .workflow.input.request_id }}-done"),
		sdktask.WithTimeout("10s"),
		sdktask.WithOutputs(inputPtr(map[string]any{"acknowledged": "{{ signal.payload.message }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, fmt.Errorf("create wait task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(
		ctx,
		"signal-communication",
		sdkworkflow.WithDescription("Uses signal and wait tasks to hand off payload"),
		sdkworkflow.WithTasks([]enginetask.Config{*signalTask, *waitTask}),
		sdkworkflow.WithOutputs(
			outputPtr(map[string]any{"acknowledged": "{{ .tasks.await-signal.output.acknowledged }}"}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create signal workflow: %w", err)
	}
	return workflowCfg, nil
}
