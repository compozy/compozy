//go:build examples

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	engineproject "github.com/compozy/compozy/engine/project"
	engineruntime "github.com/compozy/compozy/engine/runtime"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/project"
	sdkruntime "github.com/compozy/compozy/sdk/runtime"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/workflow"
)

const (
	projectID           = "runtime-native-tools"
	orchestratorAgentID = "runtime-orchestrator"
	orchestratorAction  = "plan-runtime"
	orchestratorTaskID  = "orchestrate-runtime"
	bunEntrypoint       = "./tools/index.ts"
	nodeEntrypoint      = "./tools/compat.mjs"
)

func main() {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	if err := run(ctx); err != nil {
		logger.FromContext(ctx).Error("runtime + native tools example failed", "error", err)
		os.Exit(1)
	}
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
	log.Info("running runtime + native tools example", "environment", currentEnvironment(ctx))
	actionCfg, err := buildOrchestratorAction(ctx)
	if err != nil {
		return handleBuildError(ctx, "action", err)
	}
	agentCfg, err := buildOrchestratorAgent(ctx, actionCfg)
	if err != nil {
		return handleBuildError(ctx, "agent", err)
	}
	taskCfg, err := buildRuntimeTask(ctx, agentCfg)
	if err != nil {
		return handleBuildError(ctx, "task", err)
	}
	workflowCfg, err := buildRuntimeWorkflow(ctx, agentCfg, taskCfg)
	if err != nil {
		return handleBuildError(ctx, "workflow", err)
	}
	bunRuntime, err := buildBunRuntime(ctx)
	if err != nil {
		return handleBuildError(ctx, "bun_runtime", err)
	}
	nodeRuntime := buildNodeRuntime()
	inheritedRuntime := buildInheritedRuntime(ctx)
	projectCfg, err := buildProject(ctx, workflowCfg, agentCfg, bunRuntime)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	summarizeRuntimes(ctx, bunRuntime, nodeRuntime, inheritedRuntime)
	log.Info("project configured", "project", projectCfg.Name, "workflow", workflowCfg.ID)
	return nil
}

func currentEnvironment(ctx context.Context) string {
	if cfg := config.FromContext(ctx); cfg != nil {
		return cfg.Runtime.Environment
	}
	return "unknown"
}

func buildOrchestratorAction(ctx context.Context) (*engineagent.ActionConfig, error) {
	return agent.NewAction(orchestratorAction).
		WithPrompt("Summarize the runtime profile and describe when to delegate to native tools.").
		Build(ctx)
}

func buildOrchestratorAgent(ctx context.Context, action *engineagent.ActionConfig) (*engineagent.Config, error) {
	return agent.New(orchestratorAgentID).
		WithInstructions("You evaluate runtime options and delegate to native tools when helpful.").
		WithModel("openai", "gpt-4o-mini").
		AddAction(action).
		Build(ctx)
}

func buildRuntimeTask(ctx context.Context, agentCfg *engineagent.Config) (*task.Config, error) {
	return task.NewBasic(orchestratorTaskID).
		WithAgent(agentCfg.ID).
		WithAction(orchestratorAction).
		WithFinal(true).
		Build(ctx)
}

func buildRuntimeWorkflow(
	ctx context.Context,
	agentCfg *engineagent.Config,
	taskCfg *task.Config,
) (*engineworkflow.Config, error) {
	return workflow.New("runtime-selection").
		WithDescription("Decide which runtime profile to execute for downstream tools").
		AddAgent(agentCfg).
		AddTask(taskCfg).
		Build(ctx)
}

func buildBunRuntime(ctx context.Context) (*engineruntime.Config, error) {
	nativeTools := sdkruntime.NewNativeTools().
		WithCallAgents().    // `cp__call_agents` fans out to other agents from Bun without custom glue.
		WithCallWorkflows(). // `cp__call_workflows` launches nested workflows natively inside the runtime.
		Build(ctx)
	// Permissions align with Bun security tiers: read for templates, env for secrets, net for API reach.
	return sdkruntime.NewBun().
		WithEntrypoint(bunEntrypoint).
		WithBunPermissions("--allow-read", "--allow-env", "--allow-net").
		WithNativeTools(nativeTools).
		WithToolTimeout(30 * time.Second).
		WithMaxMemoryMB(512). // Cap Bun memory to keep sandboxed workers within predictable limits.
		Build(ctx)
}

func buildNodeRuntime() *engineruntime.Config {
	// Node runtime stays available for compatibility; it relies on V8 flags instead of Bun permission gates.
	cfg := engineruntime.DefaultConfig()
	cfg.RuntimeType = engineruntime.RuntimeTypeNode
	cfg.EntrypointPath = nodeEntrypoint
	cfg.BunPermissions = nil
	cfg.NodeOptions = []string{"--experimental-fetch", "--no-warnings"}
	cfg.ToolExecutionTimeout = 45 * time.Second
	cfg.MaxMemoryMB = 768
	return cfg
}

func buildInheritedRuntime(ctx context.Context) *engineruntime.Config {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return nil
	}
	inherited := engineruntime.DefaultConfig()
	inherited.RuntimeType = cfg.Runtime.RuntimeType
	inherited.EntrypointPath = cfg.Runtime.EntrypointPath
	inherited.ToolExecutionTimeout = cfg.Runtime.ToolExecutionTimeout
	inherited.BunPermissions = append([]string{}, cfg.Runtime.BunPermissions...)
	inherited.NativeTools = &engineruntime.NativeToolsConfig{
		CallAgents:    cfg.Runtime.NativeTools.CallAgents.Enabled,
		CallWorkflows: cfg.Runtime.NativeTools.CallWorkflows.Enabled,
	}
	return inherited
}

func buildProject(
	ctx context.Context,
	wf *engineworkflow.Config,
	agentCfg *engineagent.Config,
	runtimeCfg *engineruntime.Config,
) (*engineproject.Config, error) {
	builder := project.New(projectID).
		WithDescription("Runtime demo with native tools and multiple runtime profiles").
		AddAgent(agentCfg).
		AddWorkflow(wf)
	projectCfg, err := builder.Build(ctx)
	if err != nil {
		return nil, err
	}
	projectCfg.Runtime.Type = runtimeCfg.RuntimeType
	projectCfg.Runtime.Entrypoint = runtimeCfg.EntrypointPath
	projectCfg.Runtime.Permissions = append([]string{}, runtimeCfg.BunPermissions...)
	projectCfg.Runtime.ToolExecutionTimeout = runtimeCfg.ToolExecutionTimeout
	if validateErr := projectCfg.Validate(ctx); validateErr != nil {
		return nil, validateErr
	}
	return projectCfg, nil
}

func summarizeRuntimes(ctx context.Context, bunRuntime, nodeRuntime, inherited *engineruntime.Config) {
	log := logger.FromContext(ctx)
	log.Info("bun runtime ready",
		"entrypoint", bunRuntime.EntrypointPath,
		"permissions", bunRuntime.BunPermissions,
		"call_agents", bunRuntime.NativeTools != nil && bunRuntime.NativeTools.CallAgents,
		"call_workflows", bunRuntime.NativeTools != nil && bunRuntime.NativeTools.CallWorkflows,
		"max_memory_mb", bunRuntime.MaxMemoryMB,
	)
	log.Info("node runtime profile",
		"entrypoint", nodeRuntime.EntrypointPath,
		"options", nodeRuntime.NodeOptions,
		"max_memory_mb", nodeRuntime.MaxMemoryMB,
	)
	if inherited != nil {
		// Inherited runtime mirrors the global configuration attached to the context, demonstrating the third profile.
		log.Info("inherited runtime profile",
			"type", inherited.RuntimeType,
			"entrypoint", inherited.EntrypointPath,
			"permissions", inherited.BunPermissions,
			"call_agents", inherited.NativeTools != nil && inherited.NativeTools.CallAgents,
			"call_workflows", inherited.NativeTools != nil && inherited.NativeTools.CallWorkflows,
		)
	} else {
		log.Warn("global runtime config unavailable in context; skipping inherited profile")
	}
}

func handleBuildError(ctx context.Context, stage string, err error) error {
	var buildErr *sdkerrors.BuildError
	if errors.As(err, &buildErr) {
		for _, e := range buildErr.Errors {
			logger.FromContext(ctx).Error("builder validation failed", "stage", stage, "error", e)
		}
	}
	return fmt.Errorf("%s build failed: %w", stage, err)
}
