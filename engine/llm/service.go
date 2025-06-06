package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
	"github.com/tmc/langchaingo/llms"
)

type Service struct {
	runtime *runtime.Manager
	agent   *agent.Config
	action  *agent.ActionConfig
}

func NewService(runtime *runtime.Manager, agent *agent.Config, action *agent.ActionConfig) *Service {
	return &Service{
		runtime: runtime,
		agent:   agent,
		action:  action,
	}
}

func (s *Service) CreateLLM() (llms.Model, error) {
	return s.agent.Config.CreateLLM(nil)
}

func (s *Service) GenerateContent(ctx context.Context) (*core.Output, error) {
	model, err := s.CreateLLM()
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM: %w", err)
	}

	// Validate input parameters if schema is defined
	if err := s.validateInput(ctx); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Create message content using modern API
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, s.agent.Instructions),
		llms.TextParts(llms.ChatMessageTypeSystem, s.enhancePromptForStructuredOutput()),
	}

	// Configure call options based on available tools and schemas
	callOptions := s.buildCallOptions()
	result, err := model.GenerateContent(ctx, messages, callOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute agent: %w", err)
	}
	// Process and validate the result
	output, err := s.processLLMResult(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("failed to process LLM result: %w", err)
	}
	return output, nil
}

// validateInput validates input parameters against the action's input schema
func (s *Service) validateInput(ctx context.Context) error {
	if s.action.InputSchema == nil {
		return nil
	}
	return s.action.ValidateInput(ctx, s.action.GetInput())
}

// getTools returns properly configured tool definitions
func (s *Service) getTools() []llms.Tool {
	var tools []llms.Tool
	// Use actions tools if they are defined
	if len(s.action.Tools) > 0 {
		for _, toolConfig := range s.action.Tools {
			tools = append(tools, toolConfig.GetLLMDefinition())
		}
		return tools
	}
	// Otherwise agent tools are used
	for _, toolConfig := range s.agent.Tools {
		tools = append(tools, toolConfig.GetLLMDefinition())
	}
	return tools
}

// buildCallOptions constructs the appropriate call options based on tools and schemas
func (s *Service) buildCallOptions() []llms.CallOption {
	var options []llms.CallOption
	// Add tools if available
	tools := s.getTools()
	if len(tools) > 0 {
		options = append(options, llms.WithTools(tools), llms.WithToolChoice("auto"))
		return options
	}
	// Enable structured output if supported and schema is defined (only when no tools)
	if s.shouldUseStructuredOutput() {
		options = append(options, llms.WithJSONMode())
	}
	return options
}

// shouldUseStructuredOutput determines if structured output should be enabled
func (s *Service) shouldUseStructuredOutput() bool {
	if !s.supportsStructuredOutput() {
		return false
	}
	if s.action.ShouldUseJSONOutput() {
		return true
	}
	tools := make([]tool.Config, 0, len(s.agent.Tools)+len(s.action.Tools))
	tools = append(tools, s.agent.Tools...)
	tools = append(tools, s.action.Tools...)
	for _, t := range tools {
		if t.HasSchema() {
			return true
		}
	}
	return false
}

// processLLMResult processes the LLM response and validates output against schema
func (s *Service) processLLMResult(ctx context.Context, result *llms.ContentResponse) (*core.Output, error) {
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in LLM response")
	}
	choice := result.Choices[0]
	if len(choice.ToolCalls) > 0 {
		return s.executeToolCall(ctx, choice.ToolCalls[0])
	}
	output, err := s.parseContent(choice.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}
	if err := s.validateOutput(ctx, &output); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}
	return &output, nil
}

// executeToolCall executes a tool call and validates the result
func (s *Service) executeToolCall(ctx context.Context, toolCall llms.ToolCall) (*core.Output, error) {
	if toolCall.FunctionCall == nil {
		return nil, fmt.Errorf("tool call missing function call")
	}
	toolName := toolCall.FunctionCall.Name
	arguments := toolCall.FunctionCall.Arguments
	toolConfig, err := s.findTool(toolName)
	if err != nil {
		return nil, err
	}
	input, err := s.parseArgs(arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}
	tool := NewTool(toolConfig, s.agent.Env, s.runtime)
	output, err := tool.Call(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	return output, nil
}

func (s *Service) parseArgs(arguments string) (*core.Input, error) {
	var input core.Input
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return nil, fmt.Errorf("invalid tool arguments JSON: %w", err)
	}
	return &input, nil
}

// parseContent attempts to parse content as structured JSON or returns as text
func (s *Service) parseContent(content string) (core.Output, error) {
	// If action has output schema, expect structured content
	if s.action.ShouldUseJSONOutput() {
		var structuredOutput map[string]any
		if err := json.Unmarshal([]byte(content), &structuredOutput); err != nil {
			return nil, fmt.Errorf("expected structured output but received invalid JSON: %w", err)
		}
		return core.Output(structuredOutput), nil
	}
	// Try to parse as JSON, but fall back to text if it fails
	var jsonOutput map[string]any
	if err := json.Unmarshal([]byte(content), &jsonOutput); err == nil {
		return core.Output(jsonOutput), nil
	}
	// Return as text response
	return core.Output{"response": content}, nil
}

// validateOutput validates the output against the action's output schema
func (s *Service) validateOutput(ctx context.Context, output *core.Output) error {
	if s.action.OutputSchema == nil {
		return nil
	}
	return s.action.ValidateOutput(ctx, output)
}

// findTool locates the tool configuration by name
func (s *Service) findTool(toolName string) (*tool.Config, error) {
	tools := make([]tool.Config, 0, len(s.agent.Tools)+len(s.action.Tools))
	tools = append(tools, s.agent.Tools...)
	tools = append(tools, s.action.Tools...)
	for _, tc := range tools {
		if tc.ID == toolName {
			return &tc, nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", toolName)
}

// supportsStructuredOutput checks if the current provider supports structured outputs
func (s *Service) supportsStructuredOutput() bool {
	provider := s.agent.Config.Provider
	switch provider {
	case core.ProviderOpenAI, core.ProviderGroq, core.ProviderOllama:
		return true
	default:
		return false
	}
}

// enhancePromptForStructuredOutput adds JSON instruction if structured output is needed but not mentioned
func (s *Service) enhancePromptForStructuredOutput() string {
	prompt := s.action.Prompt
	// If structured output is needed but prompt doesn't mention JSON, enhance it
	if s.shouldUseStructuredOutput() {
		if s.action.OutputSchema != nil {
			return prompt + "\n\nIMPORTANT: You MUST respond with a valid JSON object only. " +
				"Do not include any HTML, markdown, or other formatting. " +
				"Return only valid JSON matching the following schema: " + s.action.OutputSchema.String()
		}
		return prompt + "\n\nIMPORTANT: You MUST respond in valid JSON format only. " +
			"No HTML, markdown, or other formatting." +
			"Just use a tool call if needed."
	}
	return prompt
}
