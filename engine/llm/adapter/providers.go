package llmadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

type adapterWrapper func(*LangChainAdapter) LLMClient

type langChainProvider struct {
	name         core.ProviderName
	builder      ModelBuilder
	capabilities ProviderCapabilities
	wrap         adapterWrapper
}

func (p *langChainProvider) Name() core.ProviderName {
	return p.name
}

func (p *langChainProvider) Capabilities() ProviderCapabilities {
	return p.capabilities
}

func (p *langChainProvider) NewClient(ctx context.Context, cfg *core.ProviderConfig) (LLMClient, error) {
	adapter, err := NewLangChainAdapter(ctx, cfg, p.builder, p.capabilities)
	if err != nil {
		return nil, err
	}
	if p.wrap != nil {
		return p.wrap(adapter), nil
	}
	return adapter, nil
}

// BuiltinProviders enumerates the default providers supplied by the adapter
// package. A fresh slice with provider instances is returned on each call.
func BuiltinProviders() []Provider {
	return []Provider{
		newOpenAIProvider(),
		newAnthropicProvider(),
		newGroqProvider(),
		newMockProvider(),
		newOllamaProvider(),
		newGoogleProvider(),
		newDeepSeekProvider(),
		newXAIProvider(),
		newCerebrasProvider(),
		newOpenRouterProvider(),
	}
}

// RegisterProviders adds the provided providers to the registry, emitting
// structured logs for duplicate registrations.
func RegisterProviders(ctx context.Context, registry *Registry, providers ...Provider) error {
	if registry == nil {
		return fmt.Errorf("registry must not be nil")
	}
	log := logger.FromContext(ctx)
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if err := registry.Register(provider); err != nil {
			if errors.Is(err, ErrProviderAlreadyRegistered) {
				log.Warn("LLM provider already registered; skipping duplicate", "provider", provider.Name())
				continue
			}
			return fmt.Errorf("register provider %s: %w", provider.Name(), err)
		}
		log.Debug("LLM provider registered", "provider", provider.Name())
	}
	return nil
}

func newLangChainProvider(
	name core.ProviderName,
	capabilities ProviderCapabilities,
	builder ModelBuilder,
) Provider {
	return &langChainProvider{
		name:         name,
		builder:      builder,
		capabilities: capabilities,
	}
}

func newWrappedLangChainProvider(
	name core.ProviderName,
	capabilities ProviderCapabilities,
	builder ModelBuilder,
	wrap adapterWrapper,
) Provider {
	return &langChainProvider{
		name:         name,
		builder:      builder,
		capabilities: capabilities,
		wrap:         wrap,
	}
}

func newOpenAIProvider() Provider {
	return newLangChainProvider(
		core.ProviderOpenAI,
		ProviderCapabilities{
			StructuredOutput:    true,
			Streaming:           true,
			Vision:              true,
			ContextWindowTokens: 128000,
		},
		createOpenAILLM,
	)
}

func newAnthropicProvider() Provider {
	return newLangChainProvider(
		core.ProviderAnthropic,
		ProviderCapabilities{
			Streaming:           true,
			ContextWindowTokens: 200000,
		},
		createAnthropicLLM,
	)
}

func newGroqProvider() Provider {
	return newWrappedLangChainProvider(
		core.ProviderGroq,
		ProviderCapabilities{
			Streaming:           true,
			ContextWindowTokens: 32768,
		},
		createGroqLLM,
		func(adapter *LangChainAdapter) LLMClient {
			return &groqAdapter{LangChainAdapter: adapter}
		},
	)
}

func newMockProvider() Provider {
	return newLangChainProvider(
		core.ProviderMock,
		ProviderCapabilities{
			StructuredOutput:    true,
			ContextWindowTokens: 4096,
		},
		createMockLLM,
	)
}

func newOllamaProvider() Provider {
	return newWrappedLangChainProvider(
		core.ProviderOllama,
		ProviderCapabilities{
			Streaming:           true,
			ContextWindowTokens: 32768,
		},
		createOllamaLLM,
		func(adapter *LangChainAdapter) LLMClient {
			apiURL := "http://localhost:11434"
			if adapter.provider.APIURL != "" {
				apiURL = adapter.provider.APIURL
			}
			return newOllamaAdapter(adapter, apiURL)
		},
	)
}

func newGoogleProvider() Provider {
	return newLangChainProvider(
		core.ProviderGoogle,
		ProviderCapabilities{
			Streaming:           true,
			Vision:              true,
			ContextWindowTokens: 1000000,
		},
		createGoogleLLM,
	)
}

func newDeepSeekProvider() Provider {
	return newLangChainProvider(
		core.ProviderDeepSeek,
		ProviderCapabilities{
			StructuredOutput:    true,
			Streaming:           true,
			ContextWindowTokens: 128000,
		},
		createDeepSeekLLM,
	)
}

func newXAIProvider() Provider {
	return newLangChainProvider(
		core.ProviderXAI,
		ProviderCapabilities{
			StructuredOutput:    true,
			Streaming:           true,
			ContextWindowTokens: 131072,
		},
		createXAILLM,
	)
}

// createOpenAILLM creates an OpenAI LLM instance.
func createOpenAILLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	opts := []openai.Option{
		openai.WithModel(p.Model),
	}
	if p.APIKey != "" {
		opts = append(opts, openai.WithToken(p.APIKey))
	}
	if p.APIURL != "" {
		opts = append(opts, openai.WithBaseURL(p.APIURL))
	}
	if p.Organization != "" {
		opts = append(opts, openai.WithOrganization(p.Organization))
	}
	if responseFormat != nil {
		opts = append(opts, openai.WithResponseFormat(responseFormat))
	}
	return openai.New(opts...)
}

// createAnthropicLLM creates an Anthropic LLM instance.
func createAnthropicLLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	opts := []anthropic.Option{
		anthropic.WithModel(p.Model),
	}
	if p.APIKey != "" {
		opts = append(opts, anthropic.WithToken(p.APIKey))
	}
	if p.Organization != "" {
		return nil, fmt.Errorf("anthropic does not support organization")
	}
	if responseFormat != nil {
		return nil, fmt.Errorf("anthropic does not support response format")
	}
	return anthropic.New(opts...)
}

// createGroqLLM creates a Groq LLM instance.
func createGroqLLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	baseURL := "https://api.groq.com/openai/v1"
	if p.APIURL != "" {
		baseURL = p.APIURL
	}
	opts := []openai.Option{
		openai.WithModel(p.Model),
		openai.WithBaseURL(baseURL),
	}
	if p.APIKey != "" {
		opts = append(opts, openai.WithToken(p.APIKey))
	}
	if p.Organization != "" {
		opts = append(opts, openai.WithOrganization(p.Organization))
	}
	if responseFormat != nil {
		opts = append(opts, openai.WithResponseFormat(responseFormat))
	}
	return openai.New(opts...)
}

// createOllamaLLM creates an Ollama LLM instance.
func createOllamaLLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	opts := []ollama.Option{
		ollama.WithModel(p.Model),
	}
	if p.APIURL != "" {
		opts = append(opts, ollama.WithServerURL(p.APIURL))
	}
	if p.Organization != "" {
		return nil, fmt.Errorf("ollama does not support organization")
	}
	if responseFormat != nil {
		return nil, fmt.Errorf("ollama does not support structured output response formats")
	}
	return ollama.New(opts...)
}

// createMockLLM creates a mock LLM instance.
func createMockLLM(
	_ context.Context,
	p *core.ProviderConfig,
	_ *openai.ResponseFormat,
) (llms.Model, error) {
	return NewMockLLM(p.Model), nil
}

// createGoogleLLM creates a Google AI LLM instance.
func createGoogleLLM(
	ctx context.Context,
	p *core.ProviderConfig,
	_ *openai.ResponseFormat,
) (llms.Model, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context must not be nil")
	}
	opts := []googleai.Option{
		googleai.WithDefaultModel(p.Model),
	}
	if p.APIKey != "" {
		opts = append(opts, googleai.WithAPIKey(p.APIKey))
	}
	if p.APIURL != "" {
		return nil, fmt.Errorf("googleai does not support custom API URL")
	}
	if p.Organization != "" {
		return nil, fmt.Errorf("googleai does not support organization")
	}
	return googleai.New(ctx, opts...)
}

// createDeepSeekLLM creates a DeepSeek LLM instance.
func createDeepSeekLLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	baseURL := "https://api.deepseek.com/v1"
	if p.APIURL != "" {
		baseURL = p.APIURL
	}
	opts := []openai.Option{
		openai.WithModel(p.Model),
		openai.WithBaseURL(baseURL),
	}
	if p.APIKey != "" {
		opts = append(opts, openai.WithToken(p.APIKey))
	}
	if p.Organization != "" {
		opts = append(opts, openai.WithOrganization(p.Organization))
	}
	if responseFormat != nil {
		opts = append(opts, openai.WithResponseFormat(responseFormat))
	}
	return openai.New(opts...)
}

// createXAILLM creates an XAI (Grok) LLM instance.
func createXAILLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	baseURL := "https://api.x.ai/v1"
	if p.APIURL != "" {
		baseURL = p.APIURL
	}
	opts := []openai.Option{
		openai.WithModel(p.Model),
		openai.WithBaseURL(baseURL),
	}
	if p.APIKey != "" {
		opts = append(opts, openai.WithToken(p.APIKey))
	}
	if p.Organization != "" {
		opts = append(opts, openai.WithOrganization(p.Organization))
	}
	if responseFormat != nil {
		opts = append(opts, openai.WithResponseFormat(responseFormat))
	}
	return openai.New(opts...)
}

func newCerebrasProvider() Provider {
	return newLangChainProvider(
		core.ProviderCerebras,
		ProviderCapabilities{
			StructuredOutput:    true,
			Streaming:           true,
			ContextWindowTokens: 128000,
		},
		createCerebrasLLM,
	)
}

// createCerebrasLLM creates a Cerebras LLM instance.
func createCerebrasLLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	baseURL := "https://api.cerebras.ai/v1"
	if p.APIURL != "" {
		baseURL = p.APIURL
	}
	opts := []openai.Option{
		openai.WithModel(p.Model),
		openai.WithBaseURL(baseURL),
	}
	if p.APIKey != "" {
		opts = append(opts, openai.WithToken(p.APIKey))
	}
	if p.Organization != "" {
		opts = append(opts, openai.WithOrganization(p.Organization))
	}
	if responseFormat != nil {
		opts = append(opts, openai.WithResponseFormat(responseFormat))
	}
	return openai.New(opts...)
}

func newOpenRouterProvider() Provider {
	return newLangChainProvider(
		core.ProviderOpenRouter,
		ProviderCapabilities{
			StructuredOutput:    true,
			Streaming:           true,
			Vision:              true,
			ContextWindowTokens: 128000,
		},
		createOpenRouterLLM,
	)
}

// createOpenRouterLLM creates an OpenRouter LLM instance.
func createOpenRouterLLM(
	_ context.Context,
	p *core.ProviderConfig,
	responseFormat *openai.ResponseFormat,
) (llms.Model, error) {
	baseURL := "https://openrouter.ai/api/v1"
	if p.APIURL != "" {
		baseURL = p.APIURL
	}
	opts := []openai.Option{
		openai.WithModel(p.Model),
		openai.WithBaseURL(baseURL),
	}
	if p.APIKey != "" {
		opts = append(opts, openai.WithToken(p.APIKey))
	}
	if p.Organization != "" {
		opts = append(opts, openai.WithOrganization(p.Organization))
	}
	if responseFormat != nil {
		opts = append(opts, openai.WithResponseFormat(responseFormat))
	}
	return openai.New(opts...)
}

type groqAdapter struct {
	*LangChainAdapter
}

func (a *groqAdapter) GenerateContent(
	ctx context.Context,
	req *LLMRequest,
) (*LLMResponse, error) {
	resp, err := a.LangChainAdapter.GenerateContent(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.ToolCalls) == 1 && strings.EqualFold(resp.ToolCalls[0].Name, "json") {
		resp.Content = string(resp.ToolCalls[0].Arguments)
		resp.ToolCalls = nil
	}
	return resp, nil
}

// attachmentsEchoToken is used to trigger attachment echo mode in tests
const attachmentsEchoToken = "ATTACHMENTS_ECHO"

// MockLLM is a mock implementation of the LLM interface for testing
type MockLLM struct {
	model string
}

// NewMockLLM creates a new mock LLM
func NewMockLLM(model string) *MockLLM {
	return &MockLLM{
		model: model,
	}
}

// GenerateContent implements the LLM interface with predictable responses
func (m *MockLLM) GenerateContent(
	ctx context.Context,
	messages []llms.MessageContent,
	_ ...llms.CallOption,
) (*llms.ContentResponse, error) {
	// Extract prompt from messages
	prompt := m.extractPrompt(messages)
	// Attachments echo mode for tests: if the prompt contains attachmentsEchoToken,
	// summarize how many image URLs and binary parts were provided. This is only
	// used in tests that deliberately include that token in the user message.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.Contains(prompt, attachmentsEchoToken) {
		img, bin := m.countMediaParts(messages)
		response := map[string]map[string]int{
			"attachments": {
				"image_urls": img,
				"binaries":   bin,
			},
		}
		content, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal attachments response: %w", err)
		}
		return &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: string(content)}}}, nil
	}
	// Check for error conditions
	if err := m.checkErrorConditions(prompt); err != nil {
		return nil, err
	}
	// Handle delay simulation if needed
	if err := m.handleDelaySimulation(ctx, prompt); err != nil {
		return nil, err
	}
	// Generate response
	return m.generateResponse(prompt), nil
}

// extractPrompt extracts text content from messages
func (m *MockLLM) extractPrompt(messages []llms.MessageContent) string {
	var prompt string
	for _, message := range messages {
		// Check both system and human messages for trigger patterns
		if message.Role == llms.ChatMessageTypeHuman || message.Role == llms.ChatMessageTypeSystem {
			for _, part := range message.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					prompt += textPart.Text + " "
				}
			}
		}
	}
	return prompt
}

// countMediaParts scans message parts for image URLs and binary payloads.
func (m *MockLLM) countMediaParts(messages []llms.MessageContent) (int, int) {
	var img, bin int
	for _, msg := range messages {
		for _, p := range msg.Parts {
			switch p.(type) {
			case llms.ImageURLContent:
				img++
			case llms.BinaryContent:
				bin++
			}
		}
	}
	return img, bin
}

// checkErrorConditions checks if the prompt should trigger an error
func (m *MockLLM) checkErrorConditions(prompt string) error {
	// For "Process with error for testing" action, always fail
	// This is used by test fixtures that expect failure
	if strings.Contains(prompt, "Process with error for testing") {
		return fmt.Errorf("mock agent error: simulated failure for testing")
	}
	// For other actions that may fail, check for should_fail parameter
	if strings.Contains(prompt, "Process item that may fail") ||
		strings.Contains(prompt, "handle_parallel_failure") {
		// In real scenarios, should_fail would be in the prompt
		// For now, these actions fail by default in tests
		return fmt.Errorf("mock agent error: simulated failure for testing")
	}
	return nil
}

// handleDelaySimulation handles delay simulation for testing
func (m *MockLLM) handleDelaySimulation(ctx context.Context, prompt string) error {
	if strings.Contains(prompt, "duration:") ||
		strings.Contains(prompt, "Think deeply") ||
		strings.Contains(prompt, "cancellation-test") ||
		strings.Contains(prompt, "slow") ||
		strings.Contains(prompt, "long-") {
		// Use a very short delay for testing
		totalDelay := 100 * time.Millisecond

		select {
		case <-time.After(totalDelay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// generateResponse generates a mock response based on the prompt
func (m *MockLLM) generateResponse(prompt string) *llms.ContentResponse {
	responseText := m.routeResponse(prompt)
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: responseText,
			},
		},
	}
}

// routeResponse reduces the cyclomatic complexity of generateResponse by delegating
// to smaller, purpose-specific helpers.
func (m *MockLLM) routeResponse(prompt string) string {
	switch {
	case helpers.ContainsAny(prompt, "Analyze a single activity", "analyze_activity"):
		return m.responseForAnalyzeActivity(prompt)
	case helpers.ContainsAny(prompt, "Process city data", "process_city"):
		return m.responseForProcessCity(prompt)
	case helpers.ContainsAny(prompt, "Read file content", "read_content"):
		return `{"content":"// Mock file content for testing\npackage main\n\nfunc main() {\n\t// Sample code\n}"}`
	case helpers.ContainsAny(prompt, "Analyze the following Go code file"):
		return `{
            "review": "Code looks good. No major issues found.",
            "suggestions": ["Consider adding error handling", "Add comments for complex logic"],
            "score": 8
        }`
	case helpers.ContainsAny(prompt, "Process a single collection item", "process_item"):
		return `{
            "result": "Item processed successfully",
            "processed_value": 100
        }`
	case prompt != "":
		return fmt.Sprintf("Mock response for: %s", prompt)
	default:
		return "Mock agent response: task completed successfully"
	}
}

func (m *MockLLM) responseForAnalyzeActivity(prompt string) string {
	switch {
	case helpers.Contains(prompt, "hiking"):
		return `{
            "analysis": "Excellent outdoor activity for cardiovascular health",
            "rating": 5
        }`
	case helpers.Contains(prompt, "swimming"):
		return `{
            "analysis": "Great full-body workout with low impact",
            "rating": 5
        }`
	case helpers.Contains(prompt, "cycling"):
		return `{
            "analysis": "Efficient cardio exercise that builds leg strength",
            "rating": 4
        }`
	default:
		return `{
            "analysis": "Good physical activity",
            "rating": 4
        }`
	}
}

func (m *MockLLM) responseForProcessCity(prompt string) string {
	switch {
	case helpers.Contains(prompt, "Seattle"):
		return `{
            "weather": "Rainy",
            "population": 750000
        }`
	case helpers.Contains(prompt, "Portland"):
		return `{
            "weather": "Cloudy",
            "population": 650000
        }`
	case helpers.Contains(prompt, "Vancouver"):
		return `{
            "weather": "Mild",
            "population": 700000
        }`
	default:
		return `{
            "weather": "Unknown",
            "population": 500000
        }`
	}
}

// Call implements the legacy Call interface
func (m *MockLLM) Call(_ context.Context, prompt string, _ ...llms.CallOption) (string, error) {
	response := fmt.Sprintf("Mock response for: %s", prompt)
	return response, nil
}
