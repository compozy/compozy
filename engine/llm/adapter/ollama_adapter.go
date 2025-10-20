package llmadapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/ollama/ollama/api"
)

// ollamaAdapter wraps LangChainAdapter to add tool calling support for Ollama by
// delegating to the official Ollama Go SDK.
type ollamaAdapter struct {
	*LangChainAdapter

	apiURL      string
	parsedBase  *url.URL
	httpClient  *http.Client
	httpTimeout time.Duration

	clientMu  sync.Mutex
	apiClient *api.Client
}

const (
	defaultOllamaTimeout = 5 * time.Minute
	maxRemoteImageBytes  = maxInlineImageBytes
)

// OllamaAdapterOption customizes the behavior of the Ollama adapter.
type OllamaAdapterOption func(*ollamaAdapter)

// WithOllamaHTTPClient overrides the HTTP client used for Ollama API requests and remote image fetches.
func WithOllamaHTTPClient(client *http.Client) OllamaAdapterOption {
	return func(adapter *ollamaAdapter) {
		adapter.httpClient = client
	}
}

// WithOllamaTimeout overrides the default HTTP timeout used when constructing an internal client.
func WithOllamaTimeout(timeout time.Duration) OllamaAdapterOption {
	return func(adapter *ollamaAdapter) {
		adapter.httpTimeout = timeout
	}
}

// newOllamaAdapter constructs an Ollama adapter on top of an existing LangChain adapter.
func newOllamaAdapter(base *LangChainAdapter, apiURL string, opts ...OllamaAdapterOption) *ollamaAdapter {
	adapter := &ollamaAdapter{
		LangChainAdapter: base,
		apiURL:           apiURL,
		httpTimeout:      defaultOllamaTimeout,
	}
	for _, opt := range opts {
		opt(adapter)
	}
	return adapter
}

// GenerateContent overrides the base implementation to add tool calling support for Ollama.
func (a *ollamaAdapter) GenerateContent(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("nil LLMRequest")
	}
	if err := ValidateConversation(req.Messages); err != nil {
		return nil, fmt.Errorf("invalid conversation: %w", err)
	}
	log := logger.FromContext(ctx)
	if len(req.Tools) == 0 {
		return a.LangChainAdapter.GenerateContent(ctx, req)
	}

	log.Debug("Using Ollama tool calling adapter",
		"model", a.provider.Model,
		"tools_count", len(req.Tools),
		"tool_choice", req.Options.ToolChoice,
	)

	chatReq, err := a.convertToOllamaRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	response, err := a.callOllamaAPI(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return a.convertFromOllamaResponse(response)
}

func (a *ollamaAdapter) convertToOllamaRequest(ctx context.Context, req *LLMRequest) (*api.ChatRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("nil LLMRequest")
	}

	chatReq := &api.ChatRequest{
		Model:   a.provider.Model,
		Options: a.buildOllamaOptions(ctx, req),
	}

	stream := false
	chatReq.Stream = &stream

	if req.Options.ForceJSON || req.Options.OutputFormat.IsJSONSchema() {
		chatReq.Format = json.RawMessage(`"json"`)
	}

	if len(req.Tools) > 0 {
		chatReq.Tools = make(api.Tools, len(req.Tools))
		for i, tool := range req.Tools {
			apiTool, err := convertToolDefinition(tool)
			if err != nil {
				return nil, err
			}
			chatReq.Tools[i] = apiTool
		}
	}

	if req.Options.ToolChoice != "" {
		logger.FromContext(ctx).Warn(
			"Ollama API does not currently honor tool_choice; continuing with default behavior",
			"tool_choice", req.Options.ToolChoice,
		)
	}

	chatReq.Messages = make([]api.Message, 0, len(req.Messages)+1)
	a.appendSystemMessage(chatReq, req)
	if err := a.appendConversationMessages(ctx, chatReq, req); err != nil {
		return nil, err
	}

	return chatReq, nil
}

func (a *ollamaAdapter) ensureClient() (*api.Client, error) {
	a.clientMu.Lock()
	defer a.clientMu.Unlock()

	if a.apiClient != nil {
		return a.apiClient, nil
	}

	if a.parsedBase == nil {
		parsed, err := url.Parse(a.apiURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Ollama API URL %q: %w", a.apiURL, err)
		}
		a.parsedBase = parsed
	}

	httpClient := a.ensureHTTPClient()

	a.apiClient = api.NewClient(a.parsedBase, httpClient)
	return a.apiClient, nil
}

func (a *ollamaAdapter) callOllamaAPI(ctx context.Context, req *api.ChatRequest) (*api.ChatResponse, error) {
	client, err := a.ensureClient()
	if err != nil {
		return nil, err
	}

	log := logger.FromContext(ctx)
	log.Debug("Sending Ollama API request",
		"url", a.apiURL,
		"model", req.Model,
		"tools_count", len(req.Tools),
	)

	var (
		finalResp  *api.ChatResponse
		toolCalls  []api.ToolCall
		contentBuf strings.Builder
	)

	err = client.Chat(ctx, req, func(resp api.ChatResponse) error {
		if resp.Message.Content != "" {
			contentBuf.WriteString(resp.Message.Content)
		}
		if len(resp.Message.ToolCalls) > 0 {
			toolCalls = append(toolCalls, resp.Message.ToolCalls...)
		}
		respCopy := resp
		finalResp = &respCopy
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ollama chat request failed: %w", err)
	}

	if finalResp == nil {
		return nil, fmt.Errorf("ollama chat returned no response")
	}

	finalResp.Message.Content = contentBuf.String()
	if len(toolCalls) > 0 {
		finalResp.Message.ToolCalls = toolCalls
	}

	return finalResp, nil
}

func (a *ollamaAdapter) ensureHTTPClient() *http.Client {
	if a.httpClient != nil {
		return a.httpClient
	}
	timeout := a.httpTimeout
	if timeout <= 0 {
		timeout = defaultOllamaTimeout
	}
	a.httpClient = &http.Client{Timeout: timeout}
	return a.httpClient
}

func (a *ollamaAdapter) fetchRemoteImage(ctx context.Context, imageURL string) ([]byte, error) {
	u, err := url.Parse(imageURL)
	if err != nil {
		return nil, fmt.Errorf("invalid image URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported image URL scheme: %s", u.Scheme)
	}
	host := u.Hostname()
	if isLocalOrPrivateHost(host) {
		return nil, fmt.Errorf("refusing to fetch image from private/loopback host: %s", host)
	}

	baseClient := a.ensureHTTPClient()
	client := *baseClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("stopped after 3 redirects")
		}
		if redirectHost := req.URL.Hostname(); isLocalOrPrivateHost(redirectHost) {
			return fmt.Errorf("redirect to private/loopback host blocked: %s", redirectHost)
		}
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	reader := io.LimitReader(resp.Body, int64(maxRemoteImageBytes)+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) > maxRemoteImageBytes {
		return nil, fmt.Errorf("image size %d exceeds limit %d", len(data), maxRemoteImageBytes)
	}
	return data, nil
}

func isLocalOrPrivateHost(host string) bool {
	if host == "" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
	}
	for _, cidr := range privateCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (a *ollamaAdapter) convertFromOllamaResponse(resp *api.ChatResponse) (*LLMResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil Ollama response")
	}

	llmResp := &LLMResponse{
		Content: resp.Message.Content,
		Usage:   buildUsageFromResponse(resp),
	}

	if len(resp.Message.ToolCalls) > 0 {
		llmResp.ToolCalls = make([]ToolCall, len(resp.Message.ToolCalls))
		for i, tc := range resp.Message.ToolCalls {
			argsBytes, err := json.Marshal(tc.Function.Arguments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool arguments: %w", err)
			}
			llmResp.ToolCalls[i] = ToolCall{
				ID:        fmt.Sprintf("call_%d", i),
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(argsBytes),
			}
		}
	}

	return llmResp, nil
}

func buildUsageFromResponse(resp *api.ChatResponse) *Usage {
	if resp == nil {
		return nil
	}
	if resp.PromptEvalCount < 0 || resp.EvalCount < 0 {
		return nil
	}
	if resp.PromptEvalCount == 0 && resp.EvalCount == 0 {
		return nil
	}

	total := resp.PromptEvalCount + resp.EvalCount
	return &Usage{
		PromptTokens:     resp.PromptEvalCount,
		CompletionTokens: resp.EvalCount,
		TotalTokens:      total,
	}
}

func (a *ollamaAdapter) convertMessage(ctx context.Context, msg *Message) (api.Message, error) {
	if msg == nil {
		return api.Message{}, fmt.Errorf("nil message")
	}

	content, images := a.buildMessageContent(ctx, msg)

	toolCalls, err := convertToolCallsToAPI(msg.ToolCalls)
	if err != nil {
		return api.Message{}, err
	}

	result := api.Message{
		Role:      strings.ToLower(msg.Role),
		Content:   content,
		Images:    images,
		ToolCalls: toolCalls,
	}

	if msg.Role == RoleTool && len(msg.ToolResults) > 0 {
		if msg.ToolResults[0].Name != "" {
			result.ToolName = msg.ToolResults[0].Name
		}
	}

	return result, nil
}

func (a *ollamaAdapter) buildMessageContent(ctx context.Context, msg *Message) (string, []api.ImageData) {
	sections := make([]string, 0, 1+len(msg.Parts)+len(msg.ToolResults))
	if msg.Content != "" {
		sections = append(sections, msg.Content)
	}

	partSections, partImages := a.collectPartContent(ctx, msg.Parts)
	if len(partSections) > 0 {
		sections = append(sections, partSections...)
	}

	images := append([]api.ImageData(nil), partImages...)
	appendToolResultSections(&sections, msg.ToolResults)

	return joinNonEmptySections(sections), images
}

func (a *ollamaAdapter) collectPartContent(
	ctx context.Context,
	parts []ContentPart,
) ([]string, []api.ImageData) {
	if len(parts) == 0 {
		return nil, nil
	}

	sections := make([]string, 0, len(parts))
	images := make([]api.ImageData, 0, len(parts))

	for _, part := range parts {
		switch v := part.(type) {
		case TextPart:
			if v.Text != "" {
				sections = append(sections, v.Text)
			}
		case ImageURLPart:
			a.handleImageURLPart(ctx, v, &sections, &images)
		case BinaryPart:
			a.handleBinaryPart(ctx, v, &sections, &images)
		default:
			logger.FromContext(ctx).Debug(
				"Skipping unsupported content part for Ollama",
				"provider", string(a.provider.Provider),
				"part_type", fmt.Sprintf("%T", part),
			)
		}
	}

	return sections, images
}

func (a *ollamaAdapter) handleImageURLPart(
	ctx context.Context,
	part ImageURLPart,
	sections *[]string,
	images *[]api.ImageData,
) {
	if part.URL == "" {
		return
	}
	log := logger.FromContext(ctx)
	if strings.HasPrefix(part.URL, "data:") {
		data, err := decodeDataURL(part.URL)
		if err != nil {
			log.Warn(
				"Ollama adapter failed to decode image data URI",
				"provider", string(a.provider.Provider),
				"error", err,
			)
			*sections = append(*sections, imageFailureNotice("decode data URI"))
			return
		}
		if len(data) > maxInlineImageBytes {
			log.Warn(
				"Ollama adapter skipping oversized image",
				"mime", "data-uri",
				"size_bytes", len(data),
				"limit_bytes", maxInlineImageBytes,
			)
			*sections = append(*sections, oversizedImageNotice(len(data)))
			return
		}
		*images = append(*images, api.ImageData(data))
		return
	}
	data, err := a.fetchRemoteImage(ctx, part.URL)
	if err != nil {
		log.Warn(
			"Ollama adapter failed to fetch remote image",
			"provider", string(a.provider.Provider),
			"url_prefix", truncateForLog(part.URL, 48),
			"error", err,
		)
		*sections = append(*sections, imageFailureNotice("fetch remote image"))
		return
	}
	*images = append(*images, api.ImageData(data))
}

func (a *ollamaAdapter) handleBinaryPart(
	ctx context.Context,
	part BinaryPart,
	sections *[]string,
	images *[]api.ImageData,
) {
	log := logger.FromContext(ctx)
	if !strings.HasPrefix(part.MIMEType, "image/") {
		*sections = append(*sections, binaryContentNotice(part.MIMEType, len(part.Data)))
		return
	}
	if len(part.Data) > maxInlineImageBytes {
		log.Warn(
			"Ollama adapter skipping oversized image",
			"mime", part.MIMEType,
			"size_bytes", len(part.Data),
			"limit_bytes", maxInlineImageBytes,
		)
		*sections = append(*sections, oversizedImageNotice(len(part.Data)))
		return
	}
	*images = append(*images, api.ImageData(part.Data))
}

func (a *ollamaAdapter) appendSystemMessage(chatReq *api.ChatRequest, req *LLMRequest) {
	systemContent := strings.TrimSpace(req.SystemPrompt)
	if directive := buildToolChoiceDirective(req.Options.ToolChoice, req.Tools); directive != "" {
		if systemContent != "" {
			systemContent = fmt.Sprintf("%s\n\n%s", systemContent, directive)
		} else {
			systemContent = directive
		}
	}
	if systemContent == "" {
		return
	}
	chatReq.Messages = append(chatReq.Messages, api.Message{
		Role:    "system",
		Content: systemContent,
	})
}

func (a *ollamaAdapter) appendConversationMessages(
	ctx context.Context,
	chatReq *api.ChatRequest,
	req *LLMRequest,
) error {
	for i := range req.Messages {
		message := &req.Messages[i]
		if message.Role == RoleTool && len(message.ToolResults) > 1 {
			for _, result := range message.ToolResults {
				subMessage := *message
				subMessage.ToolResults = []ToolResult{result}
				msg, err := a.convertMessage(ctx, &subMessage)
				if err != nil {
					return err
				}
				chatReq.Messages = append(chatReq.Messages, msg)
			}
			continue
		}
		msg, err := a.convertMessage(ctx, message)
		if err != nil {
			return err
		}
		chatReq.Messages = append(chatReq.Messages, msg)
	}
	return nil
}

func appendToolResultSections(sections *[]string, results []ToolResult) {
	if len(results) == 0 {
		return
	}
	for _, result := range results {
		switch {
		case len(result.JSONContent) > 0:
			*sections = append(*sections, string(result.JSONContent))
		case result.Content != "":
			*sections = append(*sections, result.Content)
		}
	}
}

func joinNonEmptySections(sections []string) string {
	if len(sections) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(section)
	}
	return builder.String()
}

func convertToolCallsToAPI(calls []ToolCall) ([]api.ToolCall, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	result := make([]api.ToolCall, len(calls))
	for i, call := range calls {
		args, err := decodeToolCallArguments(call.Arguments)
		if err != nil {
			return nil, err
		}
		result[i] = api.ToolCall{
			Function: api.ToolCallFunction{
				Index:     i,
				Name:      call.Name,
				Arguments: args,
			},
		}
	}
	return result, nil
}

func decodeToolCallArguments(raw json.RawMessage) (api.ToolCallFunctionArguments, error) {
	if len(raw) == 0 {
		return api.ToolCallFunctionArguments{}, nil
	}
	var args api.ToolCallFunctionArguments
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}
	return args, nil
}

func (a *ollamaAdapter) buildOllamaOptions(ctx context.Context, req *LLMRequest) map[string]any {
	if req == nil {
		return nil
	}
	options := make(map[string]any)
	applyOllamaSamplingOptions(options, &req.Options)
	if len(req.Options.Metadata) > 0 {
		a.mergeMetadataOptions(ctx, options, req.Options.Metadata)
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func applyOllamaSamplingOptions(options map[string]any, opts *CallOptions) {
	if opts == nil {
		return
	}
	if opts.TemperatureSet {
		options["temperature"] = opts.Temperature
	} else if opts.Temperature > 0 {
		options["temperature"] = opts.Temperature
	}
	if opts.TopP > 0 {
		options["top_p"] = opts.TopP
	}
	if opts.TopK > 0 {
		options["top_k"] = opts.TopK
	}
	if opts.Seed != 0 {
		options["seed"] = opts.Seed
	}
	if opts.MaxTokens > 0 {
		options["num_predict"] = opts.MaxTokens
	}
	if opts.RepetitionPenalty > 0 {
		options["repeat_penalty"] = opts.RepetitionPenalty
	}
	if len(opts.StopWords) > 0 {
		options["stop"] = opts.StopWords
	}
}

func (a *ollamaAdapter) mergeMetadataOptions(ctx context.Context, options map[string]any, metadata map[string]any) {
	cloned := core.CloneMap(metadata)
	for key, value := range cloned {
		if _, exists := options[key]; exists {
			continue
		}
		if !isSupportedOllamaOptionValue(value) {
			logger.FromContext(ctx).Debug(
				"Skipping unsupported Ollama option metadata value",
				"provider", string(a.provider.Provider),
				"key", key,
				"type", fmt.Sprintf("%T", value),
			)
			continue
		}
		options[key] = normalizeOllamaOptionValue(value)
	}
}

func buildToolChoiceDirective(toolChoice string, tools []ToolDefinition) string {
	switch toolChoice {
	case "", "auto":
		return ""
	case "none":
		return "Do not invoke any tool functions for this request." +
			" Respond using reasoning based solely on the provided context."
	default:
		for _, tool := range tools {
			if tool.Name == toolChoice {
				return fmt.Sprintf(
					"When you need to call a tool, you must invoke the function %q. "+
						"Do not call any other tools unless explicitly instructed.",
					toolChoice,
				)
			}
		}
	}
	return ""
}

func convertToolDefinition(tool ToolDefinition) (api.Tool, error) {
	apiTool := api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Properties: map[string]api.ToolProperty{},
			},
		},
	}

	if len(tool.Parameters) == 0 {
		return apiTool, nil
	}

	raw, err := json.Marshal(tool.Parameters)
	if err != nil {
		return api.Tool{}, fmt.Errorf("failed to encode parameters for tool %q: %w", tool.Name, err)
	}

	var params api.ToolFunctionParameters
	if err := json.Unmarshal(raw, &params); err != nil {
		return api.Tool{}, fmt.Errorf("invalid parameters for tool %q: %w", tool.Name, err)
	}

	if params.Type == "" {
		params.Type = "object"
	}
	if params.Properties == nil {
		params.Properties = map[string]api.ToolProperty{}
	}

	apiTool.Function.Parameters = params
	return apiTool, nil
}

func decodeDataURL(dataURL string) ([]byte, error) {
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return nil, fmt.Errorf("invalid data URL")
	}
	meta := dataURL[:comma]
	payload := dataURL[comma+1:]
	if !strings.HasSuffix(meta, ";base64") {
		return nil, fmt.Errorf("non-base64 data URLs are not supported")
	}
	return base64.StdEncoding.DecodeString(payload)
}

func imageFailureNotice(reason string) string {
	return fmt.Sprintf("[Image unavailable: %s]", reason)
}

func oversizedImageNotice(size int) string {
	return fmt.Sprintf(oversizedImageMsgPattern, size, maxInlineImageBytes)
}

func binaryContentNotice(mime string, size int) string {
	return fmt.Sprintf("[Binary content omitted: mime=%s, size=%d bytes]", mime, size)
}

func truncateForLog(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func isSupportedOllamaOptionValue(value any) bool {
	switch v := value.(type) {
	case string, bool, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case []string:
		return true
	case []any:
		for _, item := range v {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func normalizeOllamaOptionValue(value any) any {
	switch v := value.(type) {
	case []any:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				continue
			}
			result[i] = str
		}
		return result
	default:
		return value
	}
}
