package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	enginememory "github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/memory"
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
		logger.FromContext(ctx).Error("memory conversation example failed", "error", err)
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
	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
		log.Warn("REDIS_URL not set; using local default for demonstration")
	}
	memoryCfg, err := buildMemoryConfig(ctx)
	if err != nil {
		return handleBuildError(ctx, "memory_config", err)
	}
	memoryRef, err := buildMemoryReference(ctx, memoryCfg.ID)
	if err != nil {
		return handleBuildError(ctx, "memory_reference", err)
	}
	actionCfg, err := buildSupportAction(ctx)
	if err != nil {
		return handleBuildError(ctx, "action", err)
	}
	agentCfg, err := buildSupportAgent(ctx, actionCfg, memoryRef)
	if err != nil {
		return handleBuildError(ctx, "agent", err)
	}
	taskCfg, err := buildSupportTask(ctx, agentCfg)
	if err != nil {
		return handleBuildError(ctx, "task", err)
	}
	workflowCfg, err := buildSupportWorkflow(ctx, agentCfg, taskCfg)
	if err != nil {
		return handleBuildError(ctx, "workflow", err)
	}
	modelCfg, err := buildSupportModel(ctx)
	if err != nil {
		return handleBuildError(ctx, "model", err)
	}
	projectCfg, err := buildSupportProject(ctx, modelCfg, workflowCfg, agentCfg, memoryCfg)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	printSummary(projectCfg, memoryCfg, memoryRef, redisURL)
	return nil
}

func buildMemoryConfig(ctx context.Context) (*enginememory.Config, error) {
	builder := memory.New("customer-support")
	// Token counting keeps the rolling context budget aligned with the provider's pricing and limits.
	builder = builder.WithTokenCounter("openai", "gpt-4o-mini")
	builder = builder.WithMaxTokens(2000)
	// Summarization flush compresses older turns once 1k summary tokens are accumulated.
	builder = builder.WithSummarizationFlush("openai", "gpt-4", 1000)
	// Privacy scope isolates memory per user, preventing cross-tenant leakage.
	builder = builder.WithPrivacy(memory.PrivacyUserScope)
	// Expiration automatically clears memory after 24 hours to honor retention policies.
	builder = builder.WithExpiration(24 * time.Hour)
	// Redis persistence ensures the buffer survives process restarts and shares state across replicas.
	builder = builder.WithPersistence(memory.PersistenceRedis)
	// Distributed locking prevents concurrent agents from trampling each other's writes.
	builder = builder.WithDistributedLocking(true)
	return builder.Build(ctx)
}

func buildMemoryReference(ctx context.Context, memoryID string) (*memory.ReferenceConfig, error) {
	builder := memory.NewReference(memoryID)
	// Dynamic key template scopes reads and writes to conversation+user pairs at runtime.
	builder = builder.WithKey("conversation-{{.conversation.id}}-{{.user.id}}")
	return builder.Build(ctx)
}

func buildSupportAction(ctx context.Context) (*engineagent.ActionConfig, error) {
	responseProp := schema.NewString().
		WithDescription("Agent reply grounded by shared memory").
		WithMinLength(1)
	output, err := schema.NewObject().
		AddProperty("response", responseProp).
		RequireProperty("response").
		Build(ctx)
	if err != nil {
		return nil, err
	}
	builder := agent.NewAction("answer")
	builder = builder.WithPrompt(
		"You are a support agent continuing a long running conversation. " +
			"Summarize the shared memory state before answering {{ .input.user_question }}.",
	)
	builder = builder.WithOutput(output)
	return builder.Build(ctx)
}

func buildSupportAgent(
	ctx context.Context,
	actionCfg *engineagent.ActionConfig,
	memoryRef *memory.ReferenceConfig,
) (*engineagent.Config, error) {
	if actionCfg == nil {
		return nil, fmt.Errorf("action config is required")
	}
	builder := agent.New("support-agent")
	builder = builder.WithInstructions("Resolve user issues while maintaining privacy-scoped conversation history.")
	builder = builder.WithModel("openai", "gpt-4o-mini")
	builder = builder.AddAction(actionCfg)
	builder = builder.WithMemory(memoryRef)
	return builder.Build(ctx)
}

func buildSupportTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	builder := task.NewBasic("assist-user")
	builder = builder.WithAgent(agentCfg.ID)
	builder = builder.WithAction("answer")
	builder = builder.WithInput(map[string]string{"user_question": "{{ .input.question }}"})
	builder = builder.WithOutput("response = {{ .result.output.response }}")
	builder = builder.WithFinal(true)
	return builder.Build(ctx)
}

func buildSupportWorkflow(
	ctx context.Context,
	agentCfg *engineagent.Config,
	taskCfg *enginetask.Config,
) (*engineworkflow.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("agent config is required")
	}
	builder := workflow.New("support-workflow")
	builder = builder.WithDescription("Routes user questions through a memory-backed support agent")
	builder = builder.AddAgent(agentCfg)
	builder = builder.AddTask(taskCfg)
	builder = builder.WithOutputs(map[string]string{"answer": "{{ task \"assist-user\" \"response\" }}"})
	return builder.Build(ctx)
}

func buildSupportModel(ctx context.Context) (*core.ProviderConfig, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		logger.FromContext(ctx).Warn("OPENAI_API_KEY is not set; summarization requests will fail against the provider")
	}
	builder := model.New("openai", "gpt-4o-mini")
	builder = builder.WithAPIKey(apiKey)
	builder = builder.WithDefault(true)
	builder = builder.WithTemperature(0.2)
	return builder.Build(ctx)
}

func buildSupportProject(
	ctx context.Context,
	modelCfg *core.ProviderConfig,
	workflowCfg *engineworkflow.Config,
	agentCfg *engineagent.Config,
	memoryCfg *enginememory.Config,
) (*engineproject.Config, error) {
	if modelCfg == nil {
		return nil, fmt.Errorf("model config is required")
	}
	builder := project.New("memory-demo")
	builder = builder.WithVersion("1.0.0")
	builder = builder.WithDescription("Demonstrates Compozy memory with summarization, privacy, and persistence")
	builder = builder.AddModel(modelCfg)
	builder = builder.AddWorkflow(workflowCfg)
	builder = builder.AddAgent(agentCfg)
	builder = builder.AddMemory(memoryCfg)
	return builder.Build(ctx)
}

func printSummary(
	projectCfg *engineproject.Config,
	memoryCfg *enginememory.Config,
	memoryRef *memory.ReferenceConfig,
	redisURL string,
) {
	if projectCfg == nil || memoryCfg == nil || memoryRef == nil {
		return
	}
	fmt.Println("\nMemory Conversation Example Summary")
	fmt.Printf("Project: %s (%s)\n", projectCfg.Name, projectCfg.Version)
	fmt.Printf(
		"Memory: %s (scope=%s, expiration=%s, persistence=%s)\n",
		memoryCfg.ID,
		memoryCfg.PrivacyScope,
		memoryCfg.Expiration,
		memoryCfg.Persistence.Type,
	)
	fmt.Printf("Distributed locking enabled: %t\n", memoryCfg.Locking != nil)
	fmt.Printf("Dynamic key template: %s\n", memoryRef.Key)
	fmt.Printf("Redis URL (configure via REDIS_URL): %s\n", redisURL)
	fmt.Println(
		"Run `go run ./sdk/examples/04_memory_conversation.go` after setting OPENAI_API_KEY and REDIS_URL for full persistence.",
	)
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
