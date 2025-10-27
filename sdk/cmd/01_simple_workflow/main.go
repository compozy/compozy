package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/model"
	"github.com/compozy/compozy/sdk/project"
	"github.com/compozy/compozy/sdk/schema"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/workflow"
)

func main() {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		os.Exit(1)
	}
	if err := run(ctx); err != nil {
		logger.FromContext(ctx).Error("simple workflow example failed", "error", err)
		cleanup()
		os.Exit(1)
	}
	cleanup()
}

func initializeContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.WithoutCancel(context.Background()))
	log := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(baseCtx, log)
	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		cancel()
		_ = manager.Close(ctx)
		return nil, nil, fmt.Errorf("load configuration: %w", err)
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

func run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	env := "unknown"
	if cfg := config.FromContext(ctx); cfg != nil {
		env = cfg.Runtime.Environment
	}
	log.Info("running simple workflow example", "environment", env)
	modelCfg, err := buildModel(ctx)
	if err != nil {
		return handleBuildError(ctx, "model", err)
	}
	actionCfg, err := buildGreetAction(ctx)
	if err != nil {
		return handleBuildError(ctx, "action", err)
	}
	agentCfg, err := buildAgent(ctx, actionCfg)
	if err != nil {
		return handleBuildError(ctx, "agent", err)
	}
	taskCfg, err := buildTask(ctx, agentCfg)
	if err != nil {
		return handleBuildError(ctx, "task", err)
	}
	workflowCfg, err := buildWorkflow(ctx, agentCfg, taskCfg)
	if err != nil {
		return handleBuildError(ctx, "workflow", err)
	}
	projectCfg, err := buildProject(ctx, modelCfg, workflowCfg, agentCfg)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	fmt.Println("✅ Project built successfully")
	fmt.Printf("Project: %s\n", projectCfg.Name)
	fmt.Printf("Workflow: %s (tasks: %d)\n", workflowCfg.ID, len(workflowCfg.Tasks))
	fmt.Printf(
		"Agent: %s uses the project default model: %s %s\n",
		agentCfg.ID,
		strings.ToUpper(string(modelCfg.Provider)),
		modelCfg.Model,
	)
	fmt.Println("Use `go run ./sdk/examples/01_simple_workflow.go` after setting OPENAI_API_KEY to run this example.")
	return nil
}

func buildModel(ctx context.Context) (*core.ProviderConfig, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		logger.FromContext(ctx).Warn("OPENAI_API_KEY is not set; API calls will fail without it")
	}
	builder := model.New("openai", "gpt-4o-mini").
		WithAPIKey(apiKey).
		WithDefault(true).
		WithTemperature(0.2)
	return builder.Build(ctx)
}

func buildGreetAction(ctx context.Context) (*engineagent.ActionConfig, error) {
	greetingProp := schema.NewString().WithDescription("Rendered greeting message").WithMinLength(1)
	output, err := schema.NewObject().
		AddProperty("greeting", greetingProp).
		RequireProperty("greeting").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	return agent.NewAction("greet").
		WithPrompt("Generate a friendly greeting for {{ .input.name }}.").
		WithOutput(output).
		Build(ctx)
}

func buildAgent(ctx context.Context, actionCfg *engineagent.ActionConfig) (*engineagent.Config, error) {
	if actionCfg == nil {
		return nil, fmt.Errorf("action config is required")
	}
	return agent.New("assistant").
		WithInstructions("You are a concise assistant that greets users with enthusiasm.").
		AddAction(actionCfg).
		Build(ctx)
}

func buildTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	return task.NewBasic("greet-user").
		WithAgent(agentCfg.ID).
		WithAction("greet").
		WithFinal(true).
		WithInput(map[string]string{"name": "{{ .input.name }}"}).
		WithOutput("message = {{ .result.output.greeting }}").
		Build(ctx)
}

func buildWorkflow(
	ctx context.Context,
	agentCfg *engineagent.Config,
	taskCfg *enginetask.Config,
) (*engineworkflow.Config, error) {
	return workflow.New("greeting-workflow").
		WithDescription("Greets a user by name using a single agent action").
		AddAgent(agentCfg).
		AddTask(taskCfg).
		WithOutputs(map[string]string{"greeting": "{{ task \"greet-user\" \"message\" }}"}).
		Build(ctx)
}

func buildProject(
	ctx context.Context,
	modelCfg *core.ProviderConfig,
	workflowCfg *engineworkflow.Config,
	agentCfg *engineagent.Config,
) (*engineproject.Config, error) {
	return project.New("hello-world").
		WithVersion("1.0.0").
		WithDescription("Minimal Compozy SDK project that builds a greeting workflow").
		AddModel(modelCfg).
		AddWorkflow(workflowCfg).
		AddAgent(agentCfg).
		Build(ctx)
}

func handleBuildError(ctx context.Context, stage string, err error) error {
	var buildErr *sdkerrors.BuildError
	if errors.As(err, &buildErr) {
		log := logger.FromContext(ctx)
		for idx, cause := range buildErr.Errors {
			if cause == nil {
				continue
			}
			log.Error("builder validation failed", "stage", stage, "index", idx+1, "cause", cause.Error())
		}
	}
	return fmt.Errorf("%s build failed: %w", stage, err)
}
