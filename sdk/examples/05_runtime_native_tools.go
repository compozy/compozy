package main

import (
	"context"
	"fmt"
	"time"

	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	sdktask "github.com/compozy/compozy/sdk/v2/task"
	sdktool "github.com/compozy/compozy/sdk/v2/tool"
	sdkworkflow "github.com/compozy/compozy/sdk/v2/workflow"
)

// RunRuntimeNativeTools registers a native Go tool and an inline Bun script and executes them together.
func RunRuntimeNativeTools(ctx context.Context) error {
	nativeTool, inlineTool, err := buildTools(ctx)
	if err != nil {
		return err
	}
	workflowCfg, err := buildToolWorkflow(ctx)
	if err != nil {
		return err
	}
	options := []compozy.Option{
		compozy.WithTool(nativeTool),
		compozy.WithTool(inlineTool),
		compozy.WithWorkflow(workflowCfg),
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		input := map[string]any{"name": "Avery"}
		req := &compozy.ExecuteSyncRequest{Input: input}
		resp, err := engine.ExecuteWorkflowSync(execCtx, workflowCfg.ID, req)
		if err != nil {
			return fmt.Errorf("execute workflow: %w", err)
		}
		timestamp := stringOutput(resp.Output, "generated_at")
		message := stringOutput(resp.Output, "message")
		fmt.Printf("Hybrid tool run at %s produced message:\n%s\n", timestamp, message)
		logger.FromContext(execCtx).Info("tool workflow completed", "timestamp", timestamp)
		return nil
	})
}

func buildTools(ctx context.Context) (*enginetool.Config, *enginetool.Config, error) {
	native, err := sdktool.New(
		ctx,
		"timestamp-native",
		sdktool.WithName("Timestamp (Native)"),
		sdktool.WithDescription("Returns the current UTC timestamp"),
		sdktool.WithNativeHandler(
			func(_ context.Context, input map[string]any, cfg map[string]any) (map[string]any, error) {
				now := time.Now().UTC().Format(time.RFC3339Nano)
				return map[string]any{"timestamp": now, "config": cfg, "input": input}, nil
			},
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create native tool: %w", err)
	}
	inlineCode := "export default async function(input) {\n" +
		"  const name = input?.name ?? \"friend\";\n" +
		"  const timestamp = input?.timestamp ?? \"unknown time\";\n" +
		"  return { message: `Hello ${name}! This workflow ran at ${timestamp}.` };\n" +
		"}\n"
	inline, err := sdktool.New(
		ctx,
		"greet-inline",
		sdktool.WithName("Inline Greeter"),
		sdktool.WithDescription("Formats a friendly greeting inside the Bun runtime"),
		sdktool.WithRuntime("bun"),
		sdktool.WithCode(inlineCode),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create inline tool: %w", err)
	}
	return native, inline, nil
}

func buildToolWorkflow(ctx context.Context) (*engineworkflow.Config, error) {
	stampTask, err := sdktask.New(ctx, "capture-timestamp",
		sdktask.WithTool(&enginetool.Config{ID: "timestamp-native"}),
		sdktask.WithOutputs(inputPtr(map[string]any{"timestamp": "{{ .task.output.timestamp }}"})),
	)
	if err != nil {
		return nil, fmt.Errorf("create timestamp task: %w", err)
	}
	greetTask, err := sdktask.New(ctx, "compose-greeting",
		sdktask.WithTool(&enginetool.Config{ID: "greet-inline"}),
		sdktask.WithWith(inputPtr(map[string]any{
			"name":      "{{ .workflow.input.name }}",
			"timestamp": "{{ .tasks.capture-timestamp.output.timestamp }}",
		})),
		sdktask.WithOutputs(inputPtr(map[string]any{"message": "{{ .task.output.message }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, fmt.Errorf("create greeting task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(ctx, "runtime-native-tools",
		sdkworkflow.WithDescription("Combines native Go and Bun inline tools"),
		sdkworkflow.WithTasks([]enginetask.Config{*stampTask, *greetTask}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{
			"generated_at": "{{ .tasks.capture-timestamp.output.timestamp }}",
			"message":      "{{ .tasks.compose-greeting.output.message }}",
		})),
	)
	if err != nil {
		return nil, fmt.Errorf("create tool workflow: %w", err)
	}
	return workflowCfg, nil
}
