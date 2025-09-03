package llm

import (
	"context"
	"fmt"
	"strings"

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
	// Register tools - use resolved tools if provided, otherwise use agent tools
	var toolsToRegister []tool.Config
	if len(config.ResolvedTools) > 0 {
		// Use pre-resolved tools from hierarchical inheritance
		toolsToRegister = config.ResolvedTools
	} else if agent != nil && len(agent.Tools) > 0 {
		// Fall back to agent-specific tools (backward compatibility)
		toolsToRegister = agent.Tools
	}
	// Register the determined tools
	for i := range toolsToRegister {
		localTool := NewLocalToolAdapter(&toolsToRegister[i], &runtimeAdapter{runtime})
		if err := toolRegistry.Register(ctx, localTool); err != nil {
			log.Warn("Failed to register local tool", "tool", toolsToRegister[i].ID, "error", err)
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
	directPrompt string,
) (*core.Output, error) {
	if agentConfig == nil {
		return nil, fmt.Errorf("agent config cannot be nil")
	}
	dp := strings.TrimSpace(directPrompt)
	if actionID == "" && dp == "" {
		return nil, fmt.Errorf("either actionID or directPrompt must be provided")
	}

	actionConfig, err := s.buildActionConfig(agentConfig, actionID, dp)
	if err != nil {
		return nil, err
	}

	// Defensive copy to avoid shared mutation
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

	effectiveAgent, err := s.buildEffectiveAgent(agentConfig)
	if err != nil {
		return nil, err
	}

	request := Request{Agent: effectiveAgent, Action: actionCopy}
	return s.orchestrator.Execute(ctx, request)
}

// buildActionConfig resolves the action configuration from either an action ID
// or a direct prompt, augmenting the prompt when both are provided.
func (s *Service) buildActionConfig(
	agentConfig *agent.Config,
	actionID string,
	directPrompt string,
) (*agent.ActionConfig, error) {
	if actionID != "" {
		ac, err := agent.FindActionConfig(agentConfig.Actions, actionID)
		if err != nil {
			return nil, fmt.Errorf("failed to find action config: %w", err)
		}
		if directPrompt == "" {
			return ac, nil
		}
		// Create a copy so we don't mutate the original action
		acCopy := *ac
		if acCopy.Prompt != "" {
			acCopy.Prompt = fmt.Sprintf(
				"%s\n\nAdditional context:\n\"\"\"\n%s\n\"\"\"",
				acCopy.Prompt,
				directPrompt,
			)
		} else {
			acCopy.Prompt = directPrompt
		}
		return &acCopy, nil
	}
	// Direct prompt only flow
	return &agent.ActionConfig{ID: "direct-prompt", Prompt: directPrompt}, nil
}

// buildEffectiveAgent ensures the LLM is informed about available tools. If the
// agent doesn't declare tools but resolved tools exist (from project/workflow),
// clone the agent and attach those tool definitions for LLM advertisement.
func (s *Service) buildEffectiveAgent(agentConfig *agent.Config) (*agent.Config, error) {
	if agentConfig == nil {
		return nil, fmt.Errorf("agent config cannot be nil")
	}
	if len(agentConfig.Tools) > 0 || len(s.config.ResolvedTools) == 0 {
		return agentConfig, nil
	}
	if cloned, err := agentConfig.Clone(); err == nil && cloned != nil {
		cloned.Tools = s.config.ResolvedTools
		return cloned, nil
	}
	tmp := *agentConfig
	tmp.Tools = s.config.ResolvedTools
	return &tmp, nil
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
