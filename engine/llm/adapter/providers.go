package llmadapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// CreateLLMFactory creates an LLM instance based on the provider configuration
func CreateLLMFactory(provider *core.ProviderConfig, responseFormat *openai.ResponseFormat) (llms.Model, error) {
	switch provider.Provider {
	case core.ProviderOpenAI:
		return createOpenAILLM(provider, responseFormat)
	case core.ProviderAnthropic:
		return createAnthropicLLM(provider, responseFormat)
	case core.ProviderGroq:
		return createGroqLLM(provider, responseFormat)
	case core.ProviderMock:
		return createMockLLM(provider, responseFormat)
	case core.ProviderOllama:
		return createOllamaLLM(provider, responseFormat)
	case core.ProviderGoogle:
		return createGoogleLLM(provider, responseFormat)
	case core.ProviderDeepSeek:
		return createDeepSeekLLM(provider, responseFormat)
	case core.ProviderXAI:
		return createXAILLM(provider, responseFormat)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider.Provider)
	}
}

// createOpenAILLM creates an OpenAI LLM instance
func createOpenAILLM(p *core.ProviderConfig, responseFormat *openai.ResponseFormat) (llms.Model, error) {
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

// createAnthropicLLM creates an Anthropic LLM instance
func createAnthropicLLM(p *core.ProviderConfig, responseFormat *openai.ResponseFormat) (llms.Model, error) {
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

// createGroqLLM creates a Groq LLM instance
func createGroqLLM(p *core.ProviderConfig, responseFormat *openai.ResponseFormat) (llms.Model, error) {
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

// createOllamaLLM creates an Ollama LLM instance
func createOllamaLLM(p *core.ProviderConfig, responseFormat *openai.ResponseFormat) (llms.Model, error) {
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
		opts = append(opts, ollama.WithFormat("json"))
	}
	return ollama.New(opts...)
}

// createMockLLM creates a mock LLM instance
func createMockLLM(p *core.ProviderConfig, _ *openai.ResponseFormat) (llms.Model, error) {
	return NewMockLLM(p.Model), nil
}

// createGoogleLLM creates a Google AI LLM instance
func createGoogleLLM(p *core.ProviderConfig, _ *openai.ResponseFormat) (llms.Model, error) {
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
	return googleai.New(context.Background(), opts...)
}

// createDeepSeekLLM creates a DeepSeek LLM instance
func createDeepSeekLLM(p *core.ProviderConfig, responseFormat *openai.ResponseFormat) (llms.Model, error) {
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

// createXAILLM creates an XAI (Grok) LLM instance
func createXAILLM(p *core.ProviderConfig, responseFormat *openai.ResponseFormat) (llms.Model, error) {
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
	// Extract all message content to generate a response based on it
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

	// Debug: log the actual prompt being processed
	fmt.Printf("MockLLM received prompt: %q\n", prompt)

	// For test environment, use minimal delay since cancellation propagation is limited
	// In production, the real LLM calls would be naturally long-running and cancellable
	if strings.Contains(prompt, "duration:") ||
		strings.Contains(prompt, "Think deeply") ||
		strings.Contains(prompt, "cancellation-test") ||
		strings.Contains(prompt, "slow") ||
		strings.Contains(prompt, "long-") {
		// Use a very short delay for testing - the test focuses on signal handling rather than activity cancellation
		totalDelay := 100 * time.Millisecond
		fmt.Printf("MockLLM simulating brief processing delay for cancellation testing\n")

		select {
		case <-time.After(totalDelay):
			fmt.Printf("MockLLM processing completed\n")
		case <-ctx.Done():
			fmt.Printf("MockLLM canceled: %v\n", ctx.Err())
			return nil, ctx.Err()
		}
	}

	// Generate a predictable response based on the prompt
	var responseText string
	if prompt != "" {
		responseText = fmt.Sprintf("Mock response for: %s", prompt)
	} else {
		responseText = "Mock agent response: task completed successfully"
	}

	response := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: responseText,
			},
		},
	}

	return response, nil
}

// Call implements the legacy Call interface
func (m *MockLLM) Call(_ context.Context, prompt string, _ ...llms.CallOption) (string, error) {
	response := fmt.Sprintf("Mock response for: %s", prompt)
	return response, nil
}
