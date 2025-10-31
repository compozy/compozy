package main

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	compozy "github.com/compozy/compozy/sdk/compozy"
	"github.com/compozy/compozy/sdk/tool"
)

func buildHybridTools(ctx context.Context) (*enginetool.Config, *enginetool.Config, error) {
	nativeTool, err := tool.New(
		ctx,
		"timestamp-native",
		tool.WithName("Timestamp (Native)"),
		tool.WithDescription("Return the current UTC timestamp"),
		tool.WithNativeHandler(
			func(ctx context.Context, input map[string]any, cfg map[string]any) (map[string]any, error) {
				log := logger.FromContext(ctx)
				if log != nil {
					log.Debug("executing native handler", "tool", "timestamp-native")
				}
				now := time.Now().UTC().Format(time.RFC3339Nano)
				return map[string]any{"timestamp": now, "config": cfg}, nil
			},
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("build native tool: %w", err)
	}
	inlineTool, err := tool.New(
		ctx,
		"greet-inline",
		tool.WithName("Inline Greeting"),
		tool.WithDescription("Compose a greeting message inside Bun runtime"),
		tool.WithRuntime("bun"),
		tool.WithCode(
			"export default async function(input) {\n\t\tconst name = input?.name ?? \"friend\";\n\t\treturn { message: `Hello ${name}!` };\n\t}",
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("build inline tool: %w", err)
	}
	return nativeTool, inlineTool, nil
}

func buildHybridWorkflow() *engineworkflow.Config {
	next := "greeting-step"
	workflow := &engineworkflow.Config{ID: "hybrid-workflow"}
	workflow.Tasks = []enginetask.Config{
		{
			BaseConfig: enginetask.BaseConfig{
				ID:        "timestamp-step",
				Tool:      &enginetool.Config{ID: "timestamp-native"},
				OnSuccess: &core.SuccessTransition{Next: &next},
			},
		},
		{
			BaseConfig: enginetask.BaseConfig{
				ID:    "greeting-step",
				Tool:  &enginetool.Config{ID: "greet-inline"},
				Final: true,
			},
		},
	}
	return workflow
}

func runHybridTooling(ctx context.Context) error {
	nativeTool, inlineTool, err := buildHybridTools(ctx)
	if err != nil {
		return err
	}
	workflow := buildHybridWorkflow()
	engine, err := compozy.New(
		ctx,
		compozy.WithTool(nativeTool),
		compozy.WithTool(inlineTool),
		compozy.WithWorkflow(workflow),
	)
	if err != nil {
		return fmt.Errorf("compose engine: %w", err)
	}
	report, err := engine.ValidateReferences()
	if err != nil {
		return fmt.Errorf("validate references: %w", err)
	}
	if !report.Valid {
		return fmt.Errorf("hybrid workflow has validation issues: %v", report.MissingRefs)
	}
	fmt.Println("✅ Hybrid workflow ready:", workflow.ID)
	return nil
}
