package llmadapter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

// ModelBuilder constructs langchaingo models for a specific provider.
type ModelBuilder func(
	ctx context.Context,
	config *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error)

// LangChainAdapter adapts langchaingo to our LLMClient interface.
type LangChainAdapter struct {
	provider       core.ProviderConfig
	errorParser    *ErrorParser
	validationMode ValidationMode
	baseModel      llms.Model
	modelCache     map[string]llms.Model
	modelMu        sync.RWMutex
	buildModel     ModelBuilder
	capabilities   ProviderCapabilities
}

// NewLangChainAdapter creates a new LangChain adapter using the provided model builder.
func NewLangChainAdapter(
	ctx context.Context,
	config *core.ProviderConfig,
	builder ModelBuilder,
	capabilities ProviderCapabilities,
) (*LangChainAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("provider config is nil")
	}
	if builder == nil {
		return nil, fmt.Errorf("model builder is required")
	}
	model, err := builder(ctx, config, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM model: %w", err)
	}
	// Override context window if explicitly configured
	if config.ContextWindow > 0 {
		capabilities.ContextWindowTokens = config.ContextWindow
	}
	return &LangChainAdapter{
		provider:     *config,
		errorParser:  NewErrorParser(string(config.Provider)),
		baseModel:    model,
		modelCache:   map[string]llms.Model{"default": model},
		buildModel:   builder,
		capabilities: capabilities,
	}, nil
}

// ValidationMode controls how unsupported content is handled
type ValidationMode int

const (
	// ValidationModeWarn logs warnings for unsupported content but continues processing
	ValidationModeWarn ValidationMode = iota
	// ValidationModeError returns an error for unsupported content
	ValidationModeError
	// ValidationModeSilent ignores unsupported content without logging
	ValidationModeSilent
)

const (
	maxInlineImageBytes      = 2 * 1024 * 1024 // 2 MiB
	pdfOmittedSentinel       = "[PDF omitted: provider rejects PDF binaries; attach extracted text]"
	oversizedImageMsgPattern = "[Image omitted: size %d bytes exceeds inline limit of %d bytes]"
)

// SetValidationMode configures how unsupported content is handled
func (a *LangChainAdapter) SetValidationMode(mode ValidationMode) { a.validationMode = mode }

// Capabilities reports the provider capabilities associated with this adapter.
func (a *LangChainAdapter) Capabilities() ProviderCapabilities { return a.capabilities }

// ProviderMetadata returns the configured provider and model for downstream attribution.
// Callers can use this when agent metadata is unavailable, such as tool-only flows.
func (a *LangChainAdapter) ProviderMetadata() (core.ProviderName, string) {
	if a == nil {
		return "", ""
	}
	return a.provider.Provider, a.provider.Model
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
	if req.Options.ForceJSON && !a.capabilities.StructuredOutput {
		logger.FromContext(ctx).
			Warn(
				"Provider does not support native structured output; falling back to prompt-based extraction",
				"provider",
				string(a.provider.Provider),
			)
		req.Options.ForceJSON = false
	}
	// Convert our request to langchain format
	messages, err := a.convertMessages(ctx, req)
	if err != nil {
		return nil, err
	}
	options := a.buildCallOptions(req)
	model, err := a.ensureModel(ctx, req.Options.OutputFormat)
	if err != nil {
		return nil, err
	}
	log := logger.FromContext(ctx)
	log.Debug("Calling LLM GenerateContent",
		"provider", string(a.provider.Provider),
		"model", a.provider.Model,
		"messages_count", len(messages),
		"options_count", len(options),
		"tools_count", len(req.Tools),
		"tool_choice", req.Options.ToolChoice,
	)
	response, err := model.GenerateContent(ctx, messages, options...)
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
	return a.convertResponse(ctx, response)
}

// convertMessages converts our Message format to langchain MessageContent
func (a *LangChainAdapter) convertMessages(ctx context.Context, req *LLMRequest) ([]llms.MessageContent, error) {
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
		parts, err := a.buildContentParts(ctx, m)
		if err != nil {
			return nil, err
		}
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

	return messages, nil
}

// buildContentParts converts textual content and multimodal parts to langchaingo parts.
func (a *LangChainAdapter) buildContentParts(ctx context.Context, msg *Message) ([]llms.ContentPart, error) {
	if msg == nil {
		return nil, nil
	}
	var parts []llms.ContentPart
	if msg.Content != "" {
		parts = append(parts, llms.TextContent{Text: msg.Content})
	}
	if len(msg.Parts) == 0 {
		return parts, nil
	}
	for _, p := range msg.Parts {
		switch v := p.(type) {
		case TextPart:
			parts = append(parts, llms.TextContent{Text: v.Text})
		case ImageURLPart:
			parts = append(parts, llms.ImageURLContent{URL: v.URL, Detail: v.Detail})
		case BinaryPart:
			var err error
			parts, err = a.handleBinary(ctx, v, parts)
			if err != nil {
				return nil, err
			}
		default:
			logger.FromContext(ctx).
				Debug(
					"Skipping unsupported content part",
					"provider",
					string(a.provider.Provider),
					"part_type",
					fmt.Sprintf("%T", p),
				)
		}
	}
	return parts, nil
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
) ([]llms.ContentPart, error) {
	log := logger.FromContext(ctx)
	if a.validationMode == ValidationModeError {
		return nil, fmt.Errorf("provider %s does not accept binary content of type %s", a.provider.Provider, v.MIMEType)
	}
	if a.validationMode == ValidationModeWarn {
		log.Warn(
			"Provider does not accept generic binary content. Skipping.",
			"provider", string(a.provider.Provider),
			"mime", v.MIMEType,
			"size", len(v.Data),
		)
	}
	return parts, nil
}

func containsTextPart(parts []llms.ContentPart, text string) bool {
	for _, part := range parts {
		if tc, ok := part.(llms.TextContent); ok && tc.Text == text {
			return true
		}
	}
	return false
}

func (a *LangChainAdapter) handleAV(
	ctx context.Context,
	v BinaryPart,
	parts []llms.ContentPart,
) ([]llms.ContentPart, error) {
	if a.provider.Provider == core.ProviderGoogle {
		return append(parts, llms.BinaryContent{MIMEType: v.MIMEType, Data: v.Data}), nil
	}
	if a.provider.Provider == core.ProviderOpenAI && a.isAudio(v.MIMEType) {
		if a.validationMode == ValidationModeError {
			return nil, fmt.Errorf("OpenAI audio input not supported in langchaingo (requires input_audio)")
		}
		if a.validationMode == ValidationModeWarn {
			logger.FromContext(ctx).
				Warn("OpenAI audio not supported in this path. Skipping.", "mime", v.MIMEType, "size", len(v.Data))
		}
		return parts, nil
	}
	if a.provider.Provider == core.ProviderOpenAI && a.isVideo(v.MIMEType) {
		if a.validationMode == ValidationModeError {
			return nil, fmt.Errorf("OpenAI does not support video input")
		}
		if a.validationMode == ValidationModeWarn {
			logger.FromContext(ctx).
				Warn("OpenAI does not support video input. Skipping video content.", "mime", v.MIMEType, "size", len(v.Data))
		}
		return parts, nil
	}
	return parts, nil
}

func (a *LangChainAdapter) handleBinary(
	ctx context.Context,
	v BinaryPart,
	parts []llms.ContentPart,
) ([]llms.ContentPart, error) {
	if a.isPDFAndNonGoogle(v.MIMEType) {
		logger.FromContext(ctx).
			Warn("Provider does not accept PDF binary. Omitting PDF content.", "provider", string(a.provider.Provider))
		if !containsTextPart(parts, pdfOmittedSentinel) {
			parts = append(parts, llms.TextContent{Text: pdfOmittedSentinel})
		}
		return parts, nil
	}
	if a.isImage(v.MIMEType) {
		if len(v.Data) > maxInlineImageBytes {
			logger.FromContext(ctx).
				Warn(
					"Inline image exceeds size limit; omitting",
					"mime",
					v.MIMEType,
					"size_bytes",
					len(v.Data),
					"limit_bytes",
					maxInlineImageBytes,
				)
			msg := fmt.Sprintf(oversizedImageMsgPattern, len(v.Data), maxInlineImageBytes)
			if !containsTextPart(parts, msg) {
				parts = append(parts, llms.TextContent{Text: msg})
			}
			return parts, nil
		}
		b64 := base64.StdEncoding.EncodeToString(v.Data)
		dataURL := fmt.Sprintf("data:%s;base64,%s", v.MIMEType, b64)
		return append(parts, llms.ImageURLContent{URL: dataURL}), nil
	}
	if a.isAudio(v.MIMEType) || a.isVideo(v.MIMEType) {
		return a.handleAV(ctx, v, parts)
	}
	if a.provider.Provider == core.ProviderGoogle {
		return append(parts, llms.BinaryContent{MIMEType: v.MIMEType, Data: v.Data}), nil
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
	options = append(options, buildBasicOptions(req)...)
	options = append(options, buildSamplingOptions(req)...)
	options = append(options, buildProviderSpecificOptions(req)...)
	options = append(options, buildToolOptions(a, req)...)
	options = append(options, buildOutputOptions(a, req)...)
	return options
}

func buildBasicOptions(req *LLMRequest) []llms.CallOption {
	var options []llms.CallOption
	if req.Options.TemperatureSet {
		options = append(options, llms.WithTemperature(req.Options.Temperature))
	} else if req.Options.Temperature > 0 {
		options = append(options, llms.WithTemperature(req.Options.Temperature))
	}
	if req.Options.MaxTokens > 0 {
		options = append(options, llms.WithMaxTokens(int(req.Options.MaxTokens)))
	}
	if len(req.Options.StopWords) > 0 {
		options = append(options, llms.WithStopWords(req.Options.StopWords))
	}
	return options
}

func buildSamplingOptions(req *LLMRequest) []llms.CallOption {
	var options []llms.CallOption
	if req.Options.TopP > 0 {
		options = append(options, llms.WithTopP(req.Options.TopP))
	}
	if req.Options.TopK > 0 {
		options = append(options, llms.WithTopK(req.Options.TopK))
	}
	if req.Options.FrequencyPenalty != 0 {
		options = append(options, llms.WithFrequencyPenalty(req.Options.FrequencyPenalty))
	}
	if req.Options.PresencePenalty != 0 {
		options = append(options, llms.WithPresencePenalty(req.Options.PresencePenalty))
	}
	if req.Options.Seed != 0 {
		options = append(options, llms.WithSeed(req.Options.Seed))
	}
	if req.Options.N > 0 {
		options = append(options, llms.WithN(req.Options.N))
	}
	return options
}

func buildProviderSpecificOptions(req *LLMRequest) []llms.CallOption {
	var options []llms.CallOption
	if req.Options.CandidateCount > 0 {
		options = append(options, llms.WithCandidateCount(req.Options.CandidateCount))
	}
	if req.Options.RepetitionPenalty > 0 {
		options = append(options, llms.WithRepetitionPenalty(req.Options.RepetitionPenalty))
	}
	if req.Options.MaxLength > 0 {
		options = append(options, llms.WithMaxLength(req.Options.MaxLength))
	}
	if req.Options.MinLength > 0 {
		options = append(options, llms.WithMinLength(req.Options.MinLength))
	}
	if len(req.Options.Metadata) > 0 {
		options = append(options, llms.WithMetadata(req.Options.Metadata))
	}
	return options
}

func buildToolOptions(a *LangChainAdapter, req *LLMRequest) []llms.CallOption {
	var options []llms.CallOption
	if len(req.Tools) > 0 {
		tools := a.convertTools(req.Tools)
		options = append(options, llms.WithTools(tools))
	}
	if req.Options.ToolChoice != "" {
		options = append(options, llms.WithToolChoice(req.Options.ToolChoice))
	}
	return options
}

func buildOutputOptions(a *LangChainAdapter, req *LLMRequest) []llms.CallOption {
	var options []llms.CallOption
	if a.shouldForceJSON(req) {
		options = append(options, llms.WithJSONMode())
	}
	if req.Options.ResponseMIME != "" {
		options = append(options, llms.WithResponseMIMEType(req.Options.ResponseMIME))
	}
	return options
}

const defaultModelCacheKey = "default"

func (a *LangChainAdapter) ensureModel(ctx context.Context, format OutputFormat) (llms.Model, error) {
	effectiveFormat := format
	if format.IsJSONSchema() && !a.supportsNativeStructuredOutput() {
		effectiveFormat = DefaultOutputFormat()
	}
	key, err := a.formatCacheKey(effectiveFormat)
	if err != nil {
		return nil, err
	}
	a.modelMu.Lock()
	defer a.modelMu.Unlock()
	if existing, ok := a.modelCache[key]; ok {
		return existing, nil
	}
	responseFormat, err := a.responseFormatFor(effectiveFormat)
	if err != nil {
		return nil, err
	}
	providerCfg := a.provider
	if a.buildModel == nil {
		return nil, fmt.Errorf("model builder is not configured")
	}
	model, err := a.buildModel(ctx, &providerCfg, responseFormat)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM model: %w", err)
	}
	a.modelCache[key] = model
	if key == defaultModelCacheKey {
		a.baseModel = model
	}
	return model, nil
}

func (a *LangChainAdapter) formatCacheKey(format OutputFormat) (string, error) {
	if !format.IsJSONSchema() {
		return defaultModelCacheKey, nil
	}
	if format.Schema == nil {
		return "", fmt.Errorf("structured output schema is required")
	}
	raw, err := json.Marshal(format.Schema)
	if err != nil {
		return "", fmt.Errorf("marshal structured output schema: %w", err)
	}
	sum := sha256.Sum256(raw)
	name := format.Name
	if name == "" {
		name = "action_output"
	}
	strict := "0"
	if format.Strict {
		strict = "1"
	}
	return fmt.Sprintf("schema:%s:%s:%s", name, strict, hex.EncodeToString(sum[:])), nil
}

func (a *LangChainAdapter) responseFormatFor(format OutputFormat) (*openai.ResponseFormat, error) {
	if !format.IsJSONSchema() {
		return nil, nil
	}
	if !a.supportsNativeStructuredOutput() {
		return nil, nil
	}
	return createOpenAIResponseFormat(format)
}

func (a *LangChainAdapter) supportsNativeStructuredOutput() bool {
	return a.capabilities.StructuredOutput
}

func createOpenAIResponseFormat(format OutputFormat) (*openai.ResponseFormat, error) {
	if !format.IsJSONSchema() {
		return nil, nil
	}
	if format.Schema == nil {
		return nil, fmt.Errorf("structured output schema is required")
	}
	raw, err := json.Marshal(format.Schema)
	if err != nil {
		return nil, fmt.Errorf("marshal structured output schema: %w", err)
	}
	var property openai.ResponseFormatJSONSchemaProperty
	if err := json.Unmarshal(raw, &property); err != nil {
		return nil, fmt.Errorf("convert structured output schema: %w", err)
	}
	name := format.Name
	if name == "" {
		name = "action_output"
	}
	return &openai.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &openai.ResponseFormatJSONSchema{
			Name:   name,
			Strict: format.Strict,
			Schema: &property,
		},
	}, nil
}

// convertTools converts our tool definitions to langchain format
func (a *LangChainAdapter) convertTools(tools []ToolDefinition) []llms.Tool {
	effective := a.ensureProviderTools(tools)
	llmTools := make([]llms.Tool, 0, len(effective))

	for _, tool := range effective {
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

func (a *LangChainAdapter) ensureProviderTools(tools []ToolDefinition) []ToolDefinition {
	if !strings.EqualFold(string(a.provider.Provider), string(core.ProviderGroq)) {
		return tools
	}
	for _, tool := range tools {
		if strings.EqualFold(tool.Name, "json") {
			return tools
		}
	}
	jsonParams := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	jsonTool := ToolDefinition{
		Name:        "json",
		Description: "Internal JSON structured output tool for Groq compatibility.",
		Parameters:  jsonParams,
	}
	extended := make([]ToolDefinition, len(tools)+1)
	copy(extended, tools)
	extended[len(tools)] = jsonTool
	return extended
}

func (a *LangChainAdapter) shouldForceJSON(req *LLMRequest) bool {
	if req == nil || !req.Options.ForceJSON {
		return false
	}
	return a.capabilities.StructuredOutput
}

// convertResponse converts langchain response to our format and attaches usage metadata.
func (a *LangChainAdapter) convertResponse(ctx context.Context, resp *llms.ContentResponse) (*LLMResponse, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from LLM")
	}
	if resp.Choices[0] == nil {
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

	if usage, ok := a.buildUsage(choice.GenerationInfo); ok {
		response.Usage = usage
	} else {
		a.logMissingUsage(ctx, choice.GenerationInfo)
	}
	return response, nil
}

// Close implements LLMClient interface - langchain models don't require explicit cleanup
func (a *LangChainAdapter) Close() error {
	// LangChain models don't expose cleanup methods directly
	// HTTP clients and connections are managed by the underlying providers
	return nil
}

func (a *LangChainAdapter) buildUsage(info map[string]any) (*Usage, bool) {
	counts := collectUsageCounts(info)
	if !counts.hasAny() {
		return nil, false
	}
	usage := &Usage{
		PromptTokens:       nonNeg(counts.prompt),
		CompletionTokens:   nonNeg(counts.completion),
		TotalTokens:        nonNeg(counts.total()),
		ReasoningTokens:    clampPtrNonNeg(counts.reasoning),
		CachedPromptTokens: clampPtrNonNeg(counts.cachedPrompt),
		InputAudioTokens:   clampPtrNonNeg(counts.inputAudio),
		OutputAudioTokens:  clampPtrNonNeg(counts.outputAudio),
	}
	return usage, true
}

type usageCounts struct {
	prompt        int
	completion    int
	totalValue    *int
	reasoning     *int
	cachedPrompt  *int
	inputAudio    *int
	outputAudio   *int
	hasPrompt     bool
	hasCompletion bool
}

func collectUsageCounts(info map[string]any) usageCounts {
	var counts usageCounts
	if len(info) == 0 {
		return counts
	}
	counts.prompt, counts.completion, counts.totalValue, counts.hasPrompt, counts.hasCompletion =
		collectBaseTokenCounts(info)
	counts.reasoning, counts.cachedPrompt, counts.inputAudio, counts.outputAudio =
		collectExtendedTokenCounts(info)
	return counts
}

func (c usageCounts) hasAny() bool {
	return c.hasPrompt ||
		c.hasCompletion ||
		c.totalValue != nil ||
		c.reasoning != nil ||
		c.cachedPrompt != nil ||
		c.inputAudio != nil ||
		c.outputAudio != nil
}

func (c usageCounts) total() int {
	if c.totalValue != nil {
		return *c.totalValue
	}
	if c.hasPrompt && c.hasCompletion {
		return c.prompt + c.completion
	}
	return 0
}

// collectBaseTokenCounts extracts prompt, completion, and total token usage.
// It returns counts alongside presence flags so callers can detect missing fields.
func collectBaseTokenCounts(
	info map[string]any,
) (prompt, completion int, totalValue *int, hasPrompt, hasCompletion bool) {
	prompt, hasPrompt = readUsageInt(
		info,
		"prompt_tokens",
		"PromptTokens",
		"promptTokens",
		"input_tokens",
		"inputTokens",
		"promptTokenCount",
	)
	completion, hasCompletion = readUsageInt(
		info,
		"completion_tokens",
		"CompletionTokens",
		"completionTokens",
		"output_tokens",
		"outputTokens",
		"candidatesTokenCount",
		"candidateTokenCount",
	)
	if total, ok := readUsageInt(
		info,
		"total_tokens",
		"TotalTokens",
		"totalTokens",
		"totalTokenCount",
	); ok {
		totalValue = intPtr(total)
	}
	return prompt, completion, totalValue, hasPrompt, hasCompletion
}

// collectExtendedTokenCounts extracts optional usage categories reported by providers.
// Optional fields remain nil when providers omit the corresponding usage counters.
func collectExtendedTokenCounts(
	info map[string]any,
) (reasoning, cachedPrompt, inputAudio, outputAudio *int) {
	if r, ok := readUsageInt(info, "reasoning_tokens", "ReasoningTokens"); ok {
		reasoning = intPtr(r)
	}
	if cached, ok := readUsageInt(
		info,
		"cached_prompt_tokens",
		"CachedPromptTokens",
		"cached_tokens",
		"cachedTokens",
	); ok {
		cachedPrompt = intPtr(cached)
	}
	if input, ok := readUsageInt(info, "input_audio_tokens", "InputAudioTokens"); ok {
		inputAudio = intPtr(input)
	}
	if output, ok := readUsageInt(info, "output_audio_tokens", "OutputAudioTokens"); ok {
		outputAudio = intPtr(output)
	}
	return
}

func (a *LangChainAdapter) logMissingUsage(ctx context.Context, info map[string]any) {
	log := logger.FromContext(ctx)
	if log == nil {
		return
	}
	if len(info) == 0 {
		log.Debug(
			"Provider omitted usage metadata",
			"provider",
			string(a.provider.Provider),
			"model",
			a.provider.Model,
		)
		return
	}
	keys := make([]string, 0, len(info))
	for k := range info {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	log.Debug(
		"Failed to parse usage metadata",
		"provider",
		string(a.provider.Provider),
		"model",
		a.provider.Model,
		"metadata_keys",
		keys,
	)
}

func readUsageInt(info map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := info[key]; ok {
			if parsed, ok := coerceToInt(value); ok {
				return parsed, true
			}
		}
	}
	return 0, false
}

func coerceToInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
		if f, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return int(f), true
		}
	case string:
		if v == "" {
			return 0, false
		}
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return int(f), true
		}
	}
	return 0, false
}

func nonNeg(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func clampPtrNonNeg(p *int) *int {
	if p == nil {
		return nil
	}
	value := max(*p, 0)
	return &value
}

func intPtr(v int) *int {
	value := v
	return &value
}
