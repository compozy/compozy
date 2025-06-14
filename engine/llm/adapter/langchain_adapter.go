package llmadapter

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/tmc/langchaingo/llms"
)

// LangChainAdapter adapts langchaingo to our LLMClient interface
type LangChainAdapter struct {
	model    llms.Model
	provider core.ProviderConfig
}

// NewLangChainAdapter creates a new LangChain adapter
func NewLangChainAdapter(config *core.ProviderConfig) (*LangChainAdapter, error) {
	model, err := config.CreateLLM(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM model: %w", err)
	}
	return &LangChainAdapter{
		model:    model,
		provider: *config,
	}, nil
}

// GenerateContent implements LLMClient interface
func (a *LangChainAdapter) GenerateContent(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// Convert our request to langchain format
	messages := a.convertMessages(req)
	options := a.buildCallOptions(req)

	// Call the underlying model
	response, err := a.model.GenerateContent(ctx, messages, options...)
	if err != nil {
		return nil, fmt.Errorf("langchain GenerateContent failed: %w", err)
	}

	// Convert response back to our format
	return a.convertResponse(response)
}

// convertMessages converts our Message format to langchain MessageContent
func (a *LangChainAdapter) convertMessages(req *LLMRequest) []llms.MessageContent {
	messages := make([]llms.MessageContent, 0, len(req.Messages)+1)

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, req.SystemPrompt))
	}

	// Convert each message
	for _, msg := range req.Messages {
		msgType := a.mapMessageRole(msg.Role)
		messages = append(messages, llms.TextParts(msgType, msg.Content))
	}

	return messages
}

// mapMessageRole maps our role to langchain ChatMessageType
func (a *LangChainAdapter) mapMessageRole(role string) llms.ChatMessageType {
	switch role {
	case "system":
		return llms.ChatMessageTypeSystem
	case "user":
		return llms.ChatMessageTypeHuman
	case "assistant":
		return llms.ChatMessageTypeAI
	case "tool":
		return llms.ChatMessageTypeTool
	default:
		return llms.ChatMessageTypeHuman
	}
}

// buildCallOptions builds langchain call options from our request
func (a *LangChainAdapter) buildCallOptions(req *LLMRequest) []llms.CallOption {
	var options []llms.CallOption

	// Add temperature if specified
	if req.Options.Temperature > 0 {
		options = append(options, llms.WithTemperature(req.Options.Temperature))
	}

	// Add max tokens if specified
	if req.Options.MaxTokens > 0 {
		options = append(options, llms.WithMaxTokens(int(req.Options.MaxTokens)))
	}

	// Add stop words if specified
	// TODO: Fix WithStopWords API compatibility
	// TODO: Implement stop words when API compatibility allows
	// if len(req.Options.StopWords) > 0 {
	//     options = append(options, llms.WithStopWords(req.Options.StopWords))
	// }

	// Add tools if specified
	if len(req.Tools) > 0 {
		tools := a.convertTools(req.Tools)
		options = append(options, llms.WithTools(tools))

		// Set tool choice if specified
		if req.Options.ToolChoice != "" {
			options = append(options, llms.WithToolChoice(req.Options.ToolChoice))
		}
	}

	// Enable JSON mode if requested and no tools
	if req.Options.UseJSONMode && len(req.Tools) == 0 {
		options = append(options, llms.WithJSONMode())
	}

	return options
}

// convertTools converts our tool definitions to langchain format
func (a *LangChainAdapter) convertTools(tools []ToolDefinition) []llms.Tool {
	llmTools := make([]llms.Tool, 0, len(tools))

	for _, tool := range tools {
		llmTool := llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
		llmTools = append(llmTools, llmTool)
	}

	return llmTools
}

// convertResponse converts langchain response to our format
func (a *LangChainAdapter) convertResponse(resp *llms.ContentResponse) (*LLMResponse, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}

	choice := resp.Choices[0]
	response := &LLMResponse{
		Content: choice.Content,
	}

	// Convert tool calls if present
	if len(choice.ToolCalls) > 0 {
		response.ToolCalls = make([]ToolCall, 0, len(choice.ToolCalls))
		for _, tc := range choice.ToolCalls {
			if tc.FunctionCall != nil {
				response.ToolCalls = append(response.ToolCalls, ToolCall{
					ID:        tc.ID,
					Name:      tc.FunctionCall.Name,
					Arguments: tc.FunctionCall.Arguments,
				})
			}
		}
	}

	// Note: langchaingo ContentResponse doesn't have Usage field
	// Usage tracking would need to be implemented at a different level

	return response, nil
}
