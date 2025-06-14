package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

// Orchestrator coordinates LLM interactions, tool calls, and response processing
type Orchestrator interface {
	Execute(ctx context.Context, request Request) (*core.Output, error)
	Close() error
}

// Request represents a request to the orchestrator
type Request struct {
	Agent  *agent.Config
	Action *agent.ActionConfig
}

// OrchestratorConfig configures the LLM orchestrator
type OrchestratorConfig struct {
	LLMClientFactory llmadapter.Factory
	ToolRegistry     ToolRegistry
	PromptBuilder    PromptBuilder
	RuntimeManager   *runtime.Manager
}

// Implementation of LLMOrchestrator
type llmOrchestrator struct {
	config OrchestratorConfig
}

// NewOrchestrator creates a new LLM orchestrator
func NewOrchestrator(config OrchestratorConfig) Orchestrator {
	return &llmOrchestrator{
		config: config,
	}
}

// Execute processes an LLM request end-to-end
func (o *llmOrchestrator) Execute(ctx context.Context, request Request) (*core.Output, error) {
	// Validate input
	if err := o.validateInput(ctx, request); err != nil {
		return nil, NewValidationError(err, "request", request)
	}

	// Create LLM client
	llmClient, err := o.config.LLMClientFactory.CreateClient(&request.Agent.Config)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMCreation, map[string]any{
			"provider": request.Agent.Config.Provider,
			"model":    request.Agent.Config.Model,
		})
	}

	// Build prompt
	basePrompt, err := o.config.PromptBuilder.Build(ctx, request.Action)
	if err != nil {
		return nil, NewLLMError(err, "PROMPT_BUILD_ERROR", map[string]any{
			"action": request.Action.ID,
		})
	}

	// Determine if structured output should be used
	shouldUseStructured := o.config.PromptBuilder.ShouldUseStructuredOutput(
		string(request.Agent.Config.Provider),
		request.Action,
		request.Agent.Tools,
	)

	// Enhance prompt for structured output if needed
	enhancedPrompt := basePrompt
	if shouldUseStructured {
		enhancedPrompt = o.config.PromptBuilder.EnhanceForStructuredOutput(
			basePrompt,
			request.Action.OutputSchema,
			len(request.Agent.Tools) > 0,
		)
	}

	// Build tool definitions for LLM
	toolDefs, err := o.buildToolDefinitions(ctx, request.Agent.Tools)
	if err != nil {
		return nil, NewLLMError(err, "TOOL_DEFINITIONS_ERROR", map[string]any{
			"agent": request.Agent.ID,
		})
	}

	// Prepare LLM request
	llmReq := llmadapter.LLMRequest{
		SystemPrompt: request.Agent.Instructions,
		Messages: []llmadapter.Message{
			{
				Role:    "user",
				Content: enhancedPrompt,
			},
		},
		Tools: toolDefs,
		Options: llmadapter.CallOptions{
			Temperature:      0.7, // TODO: make configurable via agent/action config
			UseJSONMode:      shouldUseStructured && len(toolDefs) == 0,
			StructuredOutput: shouldUseStructured,
		},
	}

	// Generate content
	response, err := llmClient.GenerateContent(ctx, &llmReq)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMGeneration, map[string]any{
			"agent":  request.Agent.ID,
			"action": request.Action.ID,
		})
	}

	// Process response
	output, err := o.processResponse(ctx, response, request)
	if err != nil {
		return nil, fmt.Errorf("failed to process LLM response: %w", err)
	}

	return output, nil
}

// validateInput validates the input request
func (o *llmOrchestrator) validateInput(_ context.Context, request Request) error {
	if request.Agent == nil {
		return fmt.Errorf("agent config is required")
	}

	if request.Action == nil {
		return fmt.Errorf("action config is required")
	}

	if request.Agent.Instructions == "" {
		return fmt.Errorf("agent instructions are required")
	}

	if request.Action.Prompt == "" {
		return fmt.Errorf("action prompt is required")
	}

	// Validate input schema if defined
	if request.Action.InputSchema != nil {
		// TODO: Implement input validation against schema
		logger.Debug("input schema validation not yet implemented")
	}

	return nil
}

// buildToolDefinitions converts agent tools to LLM adapter format
func (o *llmOrchestrator) buildToolDefinitions(
	ctx context.Context,
	tools []tool.Config,
) ([]llmadapter.ToolDefinition, error) {
	var definitions []llmadapter.ToolDefinition

	for _, toolConfig := range tools {
		// Find the tool in registry
		tool, found := o.config.ToolRegistry.Find(ctx, toolConfig.ID)
		if !found {
			return nil, NewToolError(
				fmt.Errorf("tool not found"),
				toolConfig.ID,
				map[string]any{"configured_tools": len(tools)},
			)
		}

		// Build tool definition
		def := llmadapter.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
		}

		// Add parameters schema if available
		if toolConfig.InputSchema != nil {
			def.Parameters = *toolConfig.InputSchema
		}

		definitions = append(definitions, def)
	}

	return definitions, nil
}

// processResponse processes the LLM response and executes any tool calls
func (o *llmOrchestrator) processResponse(
	ctx context.Context,
	response *llmadapter.LLMResponse,
	request Request,
) (*core.Output, error) {
	// If there are tool calls, execute them
	if len(response.ToolCalls) > 0 {
		return o.executeToolCalls(ctx, response.ToolCalls, request)
	}

	// Parse the content response
	output, err := o.parseContent(response.Content, request.Action.OutputSchema)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeInvalidResponse, map[string]any{
			"content": response.Content,
		})
	}

	return output, nil
}

// executeToolCalls executes tool calls and returns the result
func (o *llmOrchestrator) executeToolCalls(
	ctx context.Context,
	toolCalls []llmadapter.ToolCall,
	request Request,
) (*core.Output, error) {
	// For now, execute the first tool call
	// TODO: Support multiple tool calls and parallel execution
	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls to execute")
	}

	toolCall := toolCalls[0]

	// Find the tool
	tool, found := o.config.ToolRegistry.Find(ctx, toolCall.Name)
	if !found {
		return nil, NewToolError(
			fmt.Errorf("tool not found for execution"),
			toolCall.Name,
			map[string]any{"call_id": toolCall.ID},
		)
	}

	// Execute the tool
	result, err := tool.Call(ctx, toolCall.Arguments)
	if err != nil {
		return nil, NewToolError(err, toolCall.Name, map[string]any{
			"call_id":   toolCall.ID,
			"arguments": toolCall.Arguments,
		})
	}

	// Check for tool execution errors using improved error detection
	if toolErr, isError := IsToolExecutionError(result); isError {
		return nil, NewToolError(
			fmt.Errorf("tool execution failed: %s", toolErr.Message),
			toolCall.Name,
			map[string]any{
				"error_code":    toolErr.Code,
				"error_details": toolErr.Details,
			},
		)
	}

	// Parse the tool result with appropriate schema
	// Note: Tool output schema should come from the tool configuration, not action
	// For now, use action schema as fallback until tool schemas are properly wired
	schema := request.Action.OutputSchema
	return o.parseContent(result, schema)
}

// parseContent parses content and validates against schema if provided
func (o *llmOrchestrator) parseContent(content string, outputSchema *schema.Schema) (*core.Output, error) {
	// Try to parse as JSON first
	var data map[string]any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		// Successfully parsed as JSON
		output := core.Output(data)

		// Validate against schema if provided
		if outputSchema != nil {
			if err := o.validateOutput(&output, outputSchema); err != nil {
				return nil, NewValidationError(err, "output", data)
			}
		}

		return &output, nil
	}

	// Not valid JSON, return as text response
	output := core.Output(map[string]any{
		"response": content,
	})
	return &output, nil
}

// validateOutput validates output against schema
func (o *llmOrchestrator) validateOutput(_ *core.Output, _ *schema.Schema) error {
	// TODO: Implement proper schema validation
	logger.Debug("output schema validation not yet implemented")
	return nil
}

// Close cleans up resources
func (o *llmOrchestrator) Close() error {
	if o.config.ToolRegistry != nil {
		return o.config.ToolRegistry.Close()
	}
	return nil
}
