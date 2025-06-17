package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/runtime"
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
	ToolRegistry   ToolRegistry
	PromptBuilder  PromptBuilder
	RuntimeManager *runtime.Manager
	LLMFactory     llmadapter.Factory
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
	log := logger.FromContext(ctx)
	// Validate input
	if err := o.validateInput(ctx, request); err != nil {
		return nil, NewValidationError(err, "request", request)
	}
	// Create LLM client using factory
	factory := o.config.LLMFactory
	if factory == nil {
		factory = llmadapter.NewDefaultFactory()
	}
	llmClient, err := factory.CreateClient(&request.Agent.Config)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMCreation, map[string]any{
			"provider": request.Agent.Config.Provider,
			"model":    request.Agent.Config.Model,
		})
	}
	defer func() {
		if closeErr := llmClient.Close(); closeErr != nil {
			log.Error("Failed to close LLM client", "error", closeErr)
		}
	}()
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
			ctx,
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
			{Role: "user", Content: enhancedPrompt},
		},
		Tools: toolDefs,
		Options: llmadapter.CallOptions{
			Temperature:      0.7, // TODO: make configurable via agent/action config
			UseJSONMode:      request.Action.JSONMode || (shouldUseStructured && len(toolDefs) == 0),
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
func (o *llmOrchestrator) validateInput(ctx context.Context, request Request) error {
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
		if err := request.Action.ValidateInput(ctx, request.Action.GetInput()); err != nil {
			return fmt.Errorf("input validation failed: %w", err)
		}
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
				ErrCodeToolNotFound,
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
	output, err := o.parseContent(ctx, response.Content, request.Action)
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
	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls to execute")
	}
	// Execute all tool calls sequentially
	var results []map[string]any
	for _, toolCall := range toolCalls {
		result, err := o.executeSingleToolCall(ctx, toolCall, request)
		if err != nil {
			return nil, err
		}
		results = append(results, map[string]any{
			"tool_call_id": toolCall.ID,
			"tool_name":    toolCall.Name,
			"result":       result,
		})
	}
	// If only one tool call, return its result directly
	if len(results) == 1 {
		if result, ok := results[0]["result"].(*core.Output); ok {
			return result, nil
		}
	}
	// Multiple tool calls - return combined results
	output := core.Output(map[string]any{
		"results": results,
	})
	return &output, nil
}

// executeSingleToolCall executes a single tool call
func (o *llmOrchestrator) executeSingleToolCall(
	ctx context.Context,
	toolCall llmadapter.ToolCall,
	request Request,
) (*core.Output, error) {
	// Find the tool
	tool, found := o.config.ToolRegistry.Find(ctx, toolCall.Name)
	if !found {
		return nil, NewToolError(
			fmt.Errorf("tool not found for execution"),
			ErrCodeToolNotFound,
			toolCall.Name,
			map[string]any{"call_id": toolCall.ID},
		)
	}
	// Execute the tool
	result, err := tool.Call(ctx, string(toolCall.Arguments))
	if err != nil {
		return nil, NewToolError(err, ErrCodeToolExecution, toolCall.Name, map[string]any{
			"call_id":   toolCall.ID,
			"arguments": toolCall.Arguments,
		})
	}
	// Check for tool execution errors using improved error detection
	if toolErr, isError := IsToolExecutionError(result); isError {
		return nil, NewToolError(
			fmt.Errorf("tool execution failed: %s", toolErr.Message),
			ErrCodeToolExecution,
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
	return o.parseContent(ctx, result, request.Action)
}

// parseContent parses content and validates against schema if provided
func (o *llmOrchestrator) parseContent(
	ctx context.Context,
	content string,
	action *agent.ActionConfig,
) (*core.Output, error) {
	// Try to parse as JSON first
	var data any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		// Successfully parsed as JSON - check if it's an object
		if obj, ok := data.(map[string]any); ok {
			output := core.Output(obj)

			// Validate against schema if provided
			if err := o.validateOutput(ctx, &output, action); err != nil {
				return nil, NewValidationError(err, "output", obj)
			}

			return &output, nil
		}

		// Valid JSON but not an object - return error since core.Output expects map
		return nil, NewLLMError(
			fmt.Errorf("expected JSON object, got %T", data),
			ErrCodeInvalidResponse,
			map[string]any{"content": data},
		)
	}

	// Not valid JSON, return as text response
	output := core.Output(map[string]any{
		"response": content,
	})
	return &output, nil
}

// validateOutput validates output against schema
func (o *llmOrchestrator) validateOutput(ctx context.Context, output *core.Output, action *agent.ActionConfig) error {
	if action.OutputSchema == nil {
		return nil
	}
	return action.ValidateOutput(ctx, output)
}

// Close cleans up resources
func (o *llmOrchestrator) Close() error {
	if o.config.ToolRegistry != nil {
		return o.config.ToolRegistry.Close()
	}
	return nil
}
