package llmadapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/tmc/langchaingo/llms"
)

// LangChainAdapter adapts langchaingo to our LLMClient interface
type LangChainAdapter struct {
	model          llms.Model
	provider       core.ProviderConfig
	errorParser    *ErrorParser
	validationMode ValidationMode
}

// NewLangChainAdapter creates a new LangChain adapter
func NewLangChainAdapter(ctx context.Context, config *core.ProviderConfig) (*LangChainAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("provider config is nil")
	}
	model, err := CreateLLMFactory(ctx, config, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM model: %w", err)
	}
	return &LangChainAdapter{
		model:       model,
		provider:    *config,
		errorParser: NewErrorParser(string(config.Provider)),
	}, nil
}

// ValidationMode controls how unsupported content is handled
type ValidationMode int

const (
	ValidationModeWarn ValidationMode = iota
	ValidationModeError
	ValidationModeSilent
)

// SetValidationMode configures how unsupported content is handled
func (a *LangChainAdapter) SetValidationMode(mode ValidationMode) { a.validationMode = mode }

// convertAudioToDataURL converts audio binary to a base64 data URL
func (a *LangChainAdapter) convertAudioToDataURL(mimeType string, data []byte) string {
	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, b64)
}

// GenerateContent implements LLMClient interface
func (a *LangChainAdapter) GenerateContent(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	// Guard against nil request
	if req == nil {
		return nil, fmt.Errorf("nil LLMRequest")
	}
	// Validate role-specific constraints to catch wiring mistakes early
	if err := ValidateConversation(req.Messages); err != nil {
		return nil, fmt.Errorf("invalid conversation: %w", err)
	}
	// Convert our request to langchain format
	messages := a.convertMessages(ctx, req)
	options := a.buildCallOptions(req)
	// Call the underlying model
	response, err := a.model.GenerateContent(ctx, messages, options...)
	if err != nil {
		// Try to extract structured error information before wrapping
		// Lazy-init parser if nil to protect against zero-value construction
		if a.errorParser == nil {
			a.errorParser = NewErrorParser(string(a.provider.Provider))
		}
		if structuredErr := a.errorParser.ParseError(err); structuredErr != nil {
			return nil, structuredErr
		}
		// Fallback to wrapping unknown errors with provider/model context
		return nil, fmt.Errorf(
			"langchain GenerateContent failed (provider=%s, model=%s): %w",
			string(a.provider.Provider), a.provider.Model, err,
		)
	}
	// Convert response back to our format
	return a.convertResponse(response)
}

// convertMessages converts our Message format to langchain MessageContent
func (a *LangChainAdapter) convertMessages(ctx context.Context, req *LLMRequest) []llms.MessageContent {
	messages := make([]llms.MessageContent, 0, len(req.Messages)+1)

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, req.SystemPrompt))
	}

	// Convert each message
	for i := range req.Messages {
		m := &req.Messages[i]
		msgType := a.mapMessageRole(m.Role)
		// Build parts supporting text, tool calls, and tool results
		parts := a.buildContentParts(ctx, m)
		// Assistant tool calls
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				parts = append(parts, llms.ToolCall{
					ID:   tc.ID,
					Type: "function",
					FunctionCall: &llms.FunctionCall{
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					},
				})
			}
		}
		// Tool results - only append for appropriate roles
		if len(m.ToolResults) > 0 && m.Role == RoleTool {
			for _, tr := range m.ToolResults {
				content := tr.Content
				if len(tr.JSONContent) > 0 {
					content = string(tr.JSONContent)
				}
				parts = append(parts, llms.ToolCallResponse{
					ToolCallID: tr.ID,
					Name:       tr.Name,
					Content:    content,
				})
			}
		}
		// Only append message if it has content parts to reduce token noise
		if len(parts) > 0 {
			messages = append(messages, llms.MessageContent{Role: msgType, Parts: parts})
		}
	}

	return messages
}

// buildContentParts converts textual content and multimodal parts to langchaingo parts.
func (a *LangChainAdapter) buildContentParts(ctx context.Context, msg *Message) []llms.ContentPart {
	var parts []llms.ContentPart
	if msg != nil && msg.Content != "" {
		parts = append(parts, llms.TextContent{Text: msg.Content})
	}
	if msg == nil || len(msg.Parts) == 0 {
		return parts
	}
	for _, p := range msg.Parts {
		switch v := p.(type) {
		case TextPart:
			parts = append(parts, llms.TextContent{Text: v.Text})
		case ImageURLPart:
			parts = append(parts, llms.ImageURLContent{URL: v.URL, Detail: v.Detail})
		case BinaryPart:
			parts = a.handleBinary(ctx, v, parts)
		default:
			// ignore unknown types
		}
	}
	return parts
}

func (a *LangChainAdapter) isPDFAndNonGoogle(mime string) bool {
	return mime == "application/pdf" && a.provider.Provider != core.ProviderGoogle
}

func (a *LangChainAdapter) isImage(mime string) bool { return strings.HasPrefix(mime, "image/") }
func (a *LangChainAdapter) isAudio(mime string) bool { return strings.HasPrefix(mime, "audio/") }
func (a *LangChainAdapter) isVideo(mime string) bool { return strings.HasPrefix(mime, "video/") }

func (a *LangChainAdapter) skipOrWarnBinary(
	ctx context.Context,
	v BinaryPart,
	parts []llms.ContentPart,
) []llms.ContentPart {
	log := logger.FromContext(ctx)
	if a.validationMode == ValidationModeError {
		return parts
	}
	if a.validationMode == ValidationModeWarn {
		log.Warn(
			"Provider does not accept generic binary content. Skipping.",
			"provider", string(a.provider.Provider),
			"mime", v.MIMEType,
			"size", len(v.Data),
		)
	}
	return parts
}

func (a *LangChainAdapter) handleAV(ctx context.Context, v BinaryPart, parts []llms.ContentPart) []llms.ContentPart {
	if a.provider.Provider == core.ProviderGoogle {
		return append(parts, llms.BinaryContent{MIMEType: v.MIMEType, Data: v.Data})
	}
	if a.provider.Provider == core.ProviderOpenAI && a.isAudio(v.MIMEType) {
		const maxAudioBytes = 25 * 1024 * 1024
		if len(v.Data) > maxAudioBytes {
			if a.validationMode == ValidationModeWarn {
				logger.FromContext(ctx).Warn("Audio exceeds provider max size. Skipping.", "size", len(v.Data))
			}
			return parts
		}
		dataURL := a.convertAudioToDataURL(v.MIMEType, v.Data)
		logger.FromContext(ctx).Debug("Converted audio to data URL for OpenAI", "mime", v.MIMEType, "size", len(v.Data))
		return append(parts, llms.ImageURLContent{URL: dataURL})
	}
	if a.provider.Provider == core.ProviderOpenAI && a.isVideo(v.MIMEType) {
		if a.validationMode == ValidationModeWarn {
			logger.FromContext(ctx).
				Warn("OpenAI does not support video input. Skipping video content.", "mime", v.MIMEType, "size", len(v.Data))
		}
		return parts
	}
	return parts
}

func (a *LangChainAdapter) handleBinary(
	ctx context.Context,
	v BinaryPart,
	parts []llms.ContentPart,
) []llms.ContentPart {
	if a.isPDFAndNonGoogle(v.MIMEType) {
		logger.FromContext(ctx).
			Warn("Provider does not accept PDF binary. Omitting PDF content.", "provider", string(a.provider.Provider))
		return append(
			parts,
			llms.TextContent{Text: "[PDF omitted: provider rejects PDF binaries; attach extracted text]"},
		)
	}
	if a.isImage(v.MIMEType) {
		b64 := base64.StdEncoding.EncodeToString(v.Data)
		dataURL := fmt.Sprintf("data:%s;base64,%s", v.MIMEType, b64)
		return append(parts, llms.ImageURLContent{URL: dataURL})
	}
	if a.isAudio(v.MIMEType) || a.isVideo(v.MIMEType) {
		return a.handleAV(ctx, v, parts)
	}
	if a.provider.Provider == core.ProviderGoogle {
		return append(parts, llms.BinaryContent{MIMEType: v.MIMEType, Data: v.Data})
	}
	return a.skipOrWarnBinary(ctx, v, parts)
}

// mapAudioMIMEToFormat maps MIME types to OpenAI input_audio formats.
// Note: OpenAI Chat Completions (via langchaingo v0.1.13) does not expose
// an input_audio content part type. Audio/video BinaryParts are therefore
// omitted for OpenAI to avoid 400 errors. Other providers (e.g., GoogleAI)
// receive BinaryContent directly and handle audio/video blobs.

// mapMessageRole maps our role to langchain ChatMessageType
func (a *LangChainAdapter) mapMessageRole(role string) llms.ChatMessageType {
	switch role {
	case RoleSystem:
		return llms.ChatMessageTypeSystem
	case RoleUser:
		return llms.ChatMessageTypeHuman
	case RoleAssistant:
		return llms.ChatMessageTypeAI
	case RoleTool:
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
					Arguments: json.RawMessage(tc.FunctionCall.Arguments),
				})
			}
		}
	}

	// Note: langchaingo ContentResponse doesn't have Usage field
	// Usage tracking would need to be implemented at a different level

	return response, nil
}

// Close implements LLMClient interface - langchain models don't require explicit cleanup
func (a *LangChainAdapter) Close() error {
	// LangChain models don't expose cleanup methods directly
	// HTTP clients and connections are managed by the underlying providers
	return nil
}
