package uc

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

// SafeMCPConfig contains only non-sensitive MCP configuration data for task execution
type SafeMCPConfig struct {
	ID           string                 `json:"id"`
	URL          string                 `json:"url"`
	Command      string                 `json:"command,omitempty"`
	Proto        string                 `json:"proto,omitempty"`
	Transport    mcpproxy.TransportType `json:"transport,omitempty"`
	StartTimeout time.Duration          `json:"start_timeout,omitempty"`
	MaxSessions  int                    `json:"max_sessions,omitempty"`
	// Env field is intentionally omitted to avoid serializing sensitive data
}

type ExecuteTaskInput struct {
	TaskConfig   *task.Config    `json:"task_config"`
	WorkflowMCPs []SafeMCPConfig `json:"workflow_mcps,omitempty"`
}

type ExecuteTask struct {
	runtime *runtime.Manager
}

func NewExecuteTask(runtime *runtime.Manager) *ExecuteTask {
	return &ExecuteTask{runtime: runtime}
}

func (uc *ExecuteTask) Execute(ctx context.Context, input *ExecuteTaskInput) (*core.Output, error) {
	agentConfig := input.TaskConfig.Agent
	toolConfig := input.TaskConfig.Tool
	var result *core.Output
	var err error
	switch {
	case agentConfig != nil:
		actionID := input.TaskConfig.Action
		// TODO: remove this when do automatically selection for action
		if actionID == "" {
			return nil, fmt.Errorf("action ID is required for agent")
		}
		result, err = uc.executeAgent(ctx, agentConfig, actionID, input.WorkflowMCPs)
		if err != nil {
			return nil, fmt.Errorf("failed to execute agent: %w", err)
		}
		return result, nil
	case toolConfig != nil:
		result, err = uc.executeTool(ctx, input.TaskConfig, toolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("no component specified for execution")
	}
}

func (uc *ExecuteTask) executeAgent(
	ctx context.Context,
	agentConfig *agent.Config,
	actionID string,
	workflowMCPs []SafeMCPConfig,
) (*core.Output, error) {
	actionConfig, err := agent.FindActionConfig(agentConfig.Actions, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find action config: %w", err)
	}

	// Convert safe MCP configs back to full configs for LLM service
	fullMCPConfigs := uc.restoreMCPConfigs(workflowMCPs)
	llmService, err := llm.NewService(uc.runtime, agentConfig, actionConfig, fullMCPConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM service: %w", err)
	}
	// Ensure MCP connections are properly closed when agent execution completes
	defer func() {
		if closeErr := llmService.Close(); closeErr != nil {
			// Log error but don't fail the task
			logger.Warn("failed to close LLM service", "error", closeErr)
		}
	}()

	result, err := llmService.GenerateContent(ctx, agentConfig, actionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	return result, nil
}

func (uc *ExecuteTask) executeTool(
	ctx context.Context,
	taskConfig *task.Config,
	toolConfig *tool.Config,
) (*core.Output, error) {
	tool := llm.NewTool(toolConfig, toolConfig.Env, uc.runtime)
	output, err := tool.Call(ctx, taskConfig.With)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	return output, nil
}

// restoreMCPConfigs converts SafeMCPConfig back to full mcp.Config with environment variables
// Environment variables should be loaded from a secure source rather than serialized in activities
func (uc *ExecuteTask) restoreMCPConfigs(safeMCPs []SafeMCPConfig) []mcp.Config {
	fullConfigs := make([]mcp.Config, len(safeMCPs))
	for i, safe := range safeMCPs {
		fullConfigs[i] = mcp.Config{
			ID:           safe.ID,
			URL:          safe.URL,
			Command:      safe.Command,
			Proto:        safe.Proto,
			Transport:    safe.Transport,
			StartTimeout: safe.StartTimeout,
			MaxSessions:  safe.MaxSessions,
			// TODO: Load environment variables from secure storage based on MCP ID
			// For now, this assumes environment variables are available at runtime
			Env: make(map[string]string),
		}
	}
	return fullConfigs
}

// ProjectMCPConfigs converts full mcp.Config slice to SafeMCPConfig slice, excluding sensitive data
func ProjectMCPConfigs(fullMCPs []mcp.Config) []SafeMCPConfig {
	safeConfigs := make([]SafeMCPConfig, len(fullMCPs))
	for i, full := range fullMCPs {
		safeConfigs[i] = SafeMCPConfig{
			ID:           full.ID,
			URL:          full.URL,
			Command:      full.Command,
			Proto:        full.Proto,
			Transport:    full.Transport,
			StartTimeout: full.StartTimeout,
			MaxSessions:  full.MaxSessions,
			// Env field is intentionally omitted
		}
	}
	return safeConfigs
}
