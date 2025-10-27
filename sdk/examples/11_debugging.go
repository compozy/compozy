package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	"github.com/compozy/compozy/sdk/model"
	"github.com/compozy/compozy/sdk/project"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/workflow"
)

func main() {
	ctx, cleanup, err := initializeDebugContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare context: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	if err := runDebugExamples(ctx); err != nil {
		logger.FromContext(ctx).Error("debug example failed", "error", err)
		os.Exit(1)
	}
}

func initializeDebugContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.Background())
	log := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(baseCtx, log)
	manager := config.NewManager(ctx, config.NewService())
	_, loadErr := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider())
	if loadErr != nil {
		cancel()
		_ = manager.Close(ctx)
		return nil, nil, fmt.Errorf("load configuration: %w", loadErr)
	}
	ctx = config.ContextWithManager(ctx, manager)
	cleanup := func() {
		if err := manager.Close(ctx); err != nil {
			logger.FromContext(ctx).Warn("failed to close configuration manager", "error", err)
		}
		cancel()
	}
	return ctx, cleanup, nil
}

func runDebugExamples(ctx context.Context) error {
	if err := showErrorAccumulation(ctx); err != nil {
		return err
	}
	if err := showConfigInspection(ctx); err != nil {
		return err
	}
	if err := showManualValidation(ctx); err != nil {
		return err
	}
	if err := showPerformanceMonitoring(ctx); err != nil {
		return err
	}
	showLoggerPattern(ctx)
	return nil
}

func showErrorAccumulation(ctx context.Context) error {
	errorAgent, err := agent.New("").
		WithInstructions("").
		AddAction(nil).
		Build(ctx)
	if err == nil || errorAgent != nil {
		return errors.New("expected build to fail for debugging example")
	}
	var buildErr *sdkerrors.BuildError
	if errors.As(err, &buildErr) {
		fmt.Println("=== Error Accumulation ===")
		fmt.Printf("builder produced %d validation errors:\n", len(buildErr.Errors))
		for idx, cause := range buildErr.Errors {
			fmt.Printf("  %d. %v\n", idx+1, cause)
		}
	}
	return nil
}

func showConfigInspection(ctx context.Context) error {
	action, err := agent.NewAction("summarize").
		WithPrompt("Summarize {{ .input.topic }} in two sentences.").
		Build(ctx)
	if err != nil {
		return fmt.Errorf("build action: %w", err)
	}
	agentCfg, err := agent.New("debug-agent").
		WithInstructions("You provide concise summaries.").
		AddAction(action).
		Build(ctx)
	if err != nil {
		return fmt.Errorf("build agent: %w", err)
	}
	mapped, err := agentCfg.AsMap()
	if err != nil {
		return fmt.Errorf("convert agent to map: %w", err)
	}
	fmt.Println("=== Config Inspection (AsMap) ===")
	fmt.Printf("agent map keys: %v\n", sortedKeys(mapped))
	return nil
}

func showManualValidation(ctx context.Context) error {
	modelCfg, err := model.New("openai", "gpt-4o-mini").
		WithAPIKey("test-key").
		Build(ctx)
	if err != nil {
		return fmt.Errorf("build model: %w", err)
	}
	tsk, err := task.NewBasic("noop").
		WithTool("noop-tool").
		WithInput(map[string]string{"noop": "true"}).
		Build(ctx)
	if err != nil {
		return fmt.Errorf("build placeholder task: %w", err)
	}
	wf, err := workflow.New("debug-workflow").
		WithDescription("Minimal workflow used for validation").
		AddTask(tsk).
		Build(ctx)
	if err != nil {
		return fmt.Errorf("build workflow: %w", err)
	}
	projBuilder := project.New("manual-validate").
		WithVersion("0.1.0").
		AddModel(modelCfg).
		AddWorkflow(wf)
	fmt.Println("=== Manual Validation ===")
	fmt.Println("project configured, calling Build()...")
	projCfg, err := projBuilder.Build(ctx)
	if err != nil {
		return fmt.Errorf("project build failed: %w", err)
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		_ = projCfg.SetCWD(cwd)
	}
	if err := projCfg.Validate(ctx); err != nil {
		return fmt.Errorf("project validation failed: %w", err)
	}
	fmt.Println("project validation succeeded")
	return nil
}

func showPerformanceMonitoring(ctx context.Context) error {
	start := time.Now()
	waitTask, err := task.NewWait("micro-wait").
		WithDuration(10 * time.Millisecond).
		Build(ctx)
	if err != nil {
		return fmt.Errorf("build wait task: %w", err)
	}
	_, err = workflow.New("timed-workflow").
		WithDescription("Measures builder latency").
		AddTask(waitTask).
		Build(ctx)
	duration := time.Since(start)
	fmt.Println("=== Performance Monitoring ===")
	fmt.Printf("workflow build took %s\n", duration)
	if err != nil {
		return fmt.Errorf("timed workflow build failed: %w", err)
	}
	if duration > 50*time.Millisecond {
		fmt.Println("⚠️ build exceeded expected budget (50ms)")
	}
	return nil
}

func showLoggerPattern(ctx context.Context) {
	log := logger.FromContext(ctx)
	fmt.Println("=== Logger From Context ===")
	log.Debug("debug logging enabled", "component", "examples", "timestamp", time.Now().Format(time.RFC3339))
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
