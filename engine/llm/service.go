package llm

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

// Service provides LLM integration capabilities using clean architecture
type Service struct {
	orchestrator Orchestrator
	config       *Config
}

// NewService creates a new LLM service with clean architecture
func NewService(ctx context.Context, runtime runtime.Runtime, agent *agent.Config, opts ...Option) (*Service, error) {
	log := logger.FromContext(ctx)
	// Build configuration
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	// Additional sanity checks to fail fast on obviously bad values
	if config.Timeout <= 0 {
		return nil, fmt.Errorf("invalid configuration: Timeout must be > 0")
	}
	if config.MaxConcurrentTools < 1 {
		return nil, fmt.Errorf("invalid configuration: MaxConcurrentTools must be >= 1")
	}
	// Create MCP client if configured
	var mcpClient *mcp.Client
	if config.ProxyURL != "" {
		client, err := config.CreateMCPClient()
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP client: %w", err)
		}
		mcpClient = client
	}
	// Create tool registry
	toolRegistry := NewToolRegistry(ToolRegistryConfig{
		ProxyClient: mcpClient,
		CacheTTL:    config.CacheTTL,
	})
	// Register local tools
	if agent != nil {
		for i := range agent.Tools {
			localTool := NewLocalToolAdapter(&agent.Tools[i], &runtimeAdapter{runtime})
			if err := toolRegistry.Register(ctx, localTool); err != nil {
				log.Warn("Failed to register local tool", "tool", agent.Tools[i].ID, "error", err)
			}
		}
	}
	// Create components
	promptBuilder := NewPromptBuilder()
	// Create orchestrator
	orchestratorConfig := OrchestratorConfig{
		ToolRegistry:       toolRegistry,
		PromptBuilder:      promptBuilder,
		RuntimeManager:     runtime,
		LLMFactory:         config.LLMFactory,
		MemoryProvider:     config.MemoryProvider,
		Timeout:            config.Timeout,
		MaxConcurrentTools: config.MaxConcurrentTools,
		RetryAttempts:      config.RetryAttempts,
		RetryBackoffBase:   config.RetryBackoffBase,
		RetryBackoffMax:    config.RetryBackoffMax,
		RetryJitter:        config.RetryJitter,
	}
	llmOrchestrator := NewOrchestrator(&orchestratorConfig)
	return &Service{
		orchestrator: llmOrchestrator,
		config:       config,
	}, nil
}

// GenerateContent generates content using the orchestrator
func (s *Service) GenerateContent(
	ctx context.Context,
	agentConfig *agent.Config,
	taskWith *core.Input,
	actionID string,
) (*core.Output, error) {
	if agentConfig == nil {
		return nil, fmt.Errorf("agent config cannot be nil")
	}
	if actionID == "" {
		return nil, fmt.Errorf("actionID cannot be empty")
	}
	actionConfig, err := agent.FindActionConfig(agentConfig.Actions, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find action config: %w", err)
	}
	// Defensive copy to avoid shared-mutation/race on the agent's action config
	actionCopy, err := core.DeepCopy(actionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to clone action config: %w", err)
	}
	if taskWith != nil {
		inputCopy, err := core.DeepCopy(taskWith)
		if err != nil {
			return nil, fmt.Errorf("failed to clone task with: %w", err)
		}
		actionCopy.With = inputCopy
	}
	request := Request{
		Agent:  agentConfig,
		Action: actionCopy,
	}
	return s.orchestrator.Execute(ctx, request)
}

// InvalidateToolsCache invalidates the tools cache
func (s *Service) InvalidateToolsCache(ctx context.Context) {
	if orchestrator, ok := s.orchestrator.(*llmOrchestrator); ok {
		orchestrator.config.ToolRegistry.InvalidateCache(ctx)
	}
}

// Close cleans up resources
func (s *Service) Close() error {
	if s.orchestrator != nil {
		return s.orchestrator.Close()
	}
	return nil
}

// runtimeAdapter adapts runtime.Runtime to the registry.ToolRuntime interface
type runtimeAdapter struct {
	manager runtime.Runtime
}

func (r *runtimeAdapter) ExecuteTool(
	ctx context.Context,
	toolConfig *tool.Config,
	input map[string]any,
) (*core.Output, error) {
	// Convert input to core.Input
	coreInput := core.NewInput(input)
	// Create tool execution ID
	toolExecID, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate tool execution ID: %w", err)
	}
	// Get config from tool configuration
	config := toolConfig.GetConfig()
	// Execute the tool using the runtime manager
	return r.manager.ExecuteTool(ctx, toolConfig.ID, toolExecID, &coreInput, config, core.EnvMap{})
}
