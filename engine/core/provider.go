package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// Name represents the name of a provider
type ProviderName string

const (
	ProviderOpenAI    ProviderName = "openai"
	ProviderGroq      ProviderName = "groq"
	ProviderAnthropic ProviderName = "anthropic"
	ProviderGoogle    ProviderName = "google"
	ProviderOllama    ProviderName = "ollama"
	ProviderDeepSeek  ProviderName = "deepseek"
	ProviderXAI       ProviderName = "xai"
	ProviderMock      ProviderName = "mock" // Mock provider for testing
)

type PromptParams struct {
	MaxTokens         int32    `json:"max_tokens,omitempty"         yaml:"max_tokens,omitempty"         mapstructure:"max_tokens,omitempty"`
	Temperature       float64  `json:"temperature,omitempty"        yaml:"temperature,omitempty"        mapstructure:"temperature,omitempty"`
	StopWords         []string `json:"stop_words,omitempty"         yaml:"stop_words,omitempty"         mapstructure:"stop_words,omitempty"`
	TopK              int      `json:"top_k,omitempty"              yaml:"top_k,omitempty"              mapstructure:"top_k,omitempty"`
	TopP              float64  `json:"top_p,omitempty"              yaml:"top_p,omitempty"              mapstructure:"top_p,omitempty"`
	Seed              int      `json:"seed,omitempty"               yaml:"seed,omitempty"               mapstructure:"seed,omitempty"`
	MinLength         int      `json:"min_length,omitempty"         yaml:"min_length,omitempty"         mapstructure:"min_length,omitempty"`
	MaxLength         int      `json:"max_length,omitempty"         yaml:"max_length,omitempty"         mapstructure:"max_length,omitempty"`
	RepetitionPenalty float64  `json:"repetition_penalty,omitempty" yaml:"repetition_penalty,omitempty" mapstructure:"repetition_penalty,omitempty"`
}

// ProviderConfig represents provider-specific configuration options
type ProviderConfig struct {
	Provider     ProviderName `json:"provider"     yaml:"provider"     mapstructure:"provider"`
	Model        string       `json:"model"        yaml:"model"        mapstructure:"model"`
	APIKey       string       `json:"api_key"      yaml:"api_key"      mapstructure:"api_key"`
	APIURL       string       `json:"api_url"      yaml:"api_url"      mapstructure:"api_url"`
	Params       PromptParams `json:"params"       yaml:"params"       mapstructure:"params"`
	Organization string       `json:"organization" yaml:"organization" mapstructure:"organization"`
}

// NewProviderConfig creates a new ProviderConfig with the API URL populated
func NewProviderConfig(provider ProviderName, model string, apiKey string) *ProviderConfig {
	config := &ProviderConfig{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
	}
	return config
}

// AsJSON converts the provider configuration to a JSON value
func (p *ProviderConfig) AsJSON() (json.RawMessage, error) {
	return json.Marshal(p)
}

// AsMap converts the provider configuration to a map for template normalization
func (p *ProviderConfig) AsMap() (map[string]any, error) {
	return AsMapDefault(p)
}

// FromMap updates the provider configuration from a normalized map
func (p *ProviderConfig) FromMap(data any) error {
	config, err := FromMapDefault[ProviderConfig](data)
	if err != nil {
		return err
	}
	return mergo.Merge(p, config, mergo.WithOverride)
}

// CreateLLM creates an LLM instance based on the provider configuration
func (p *ProviderConfig) CreateLLM(responseFormat *openai.ResponseFormat) (llms.Model, error) {
	switch p.Provider {
	case ProviderOpenAI:
		return p.createOpenAILLM(responseFormat)
	case ProviderAnthropic:
		return p.createAnthropicLLM(responseFormat)
	case ProviderGroq:
		return p.createGroqLLM(responseFormat)
	case ProviderMock:
		return p.createMockLLM(responseFormat)
	case ProviderOllama:
		return p.createOllamaLLM(responseFormat)
	case ProviderGoogle:
		return p.createGoogleLLM(responseFormat)
	case ProviderDeepSeek:
		return p.createDeepSeekLLM(responseFormat)
	case ProviderXAI:
		return p.createXAILLM(responseFormat)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", p.Provider)
	}
}

// createOpenAILLM creates an OpenAI LLM instance
func (p *ProviderConfig) createOpenAILLM(responseFormat *openai.ResponseFormat) (llms.Model, error) {
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
func (p *ProviderConfig) createAnthropicLLM(responseFormat *openai.ResponseFormat) (llms.Model, error) {
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
func (p *ProviderConfig) createGroqLLM(responseFormat *openai.ResponseFormat) (llms.Model, error) {
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
func (p *ProviderConfig) createOllamaLLM(responseFormat *openai.ResponseFormat) (llms.Model, error) {
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
func (p *ProviderConfig) createMockLLM(_ *openai.ResponseFormat) (llms.Model, error) {
	return NewMockLLM(p.Model), nil
}

// createGoogleLLM creates a Google AI LLM instance
func (p *ProviderConfig) createGoogleLLM(_ *openai.ResponseFormat) (llms.Model, error) {
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
func (p *ProviderConfig) createDeepSeekLLM(responseFormat *openai.ResponseFormat) (llms.Model, error) {
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
func (p *ProviderConfig) createXAILLM(responseFormat *openai.ResponseFormat) (llms.Model, error) {
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

// -----------------------------------------------------------------------------
// Mock Provider
// -----------------------------------------------------------------------------

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
	// Extract the human message content to generate a response based on it
	var prompt string
	for _, message := range messages {
		if message.Role == llms.ChatMessageTypeHuman {
			for _, part := range message.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					prompt = textPart.Text
				}
			}
		}
	}

	// Simulate long execution for cancellation testing scenarios only
	if strings.Contains(prompt, "duration: 10s") ||
		strings.Contains(prompt, "Think deeply") ||
		strings.Contains(prompt, "cancellation-test") {
		// Simulate a long-running task for cancellation testing
		delay := 500 * time.Millisecond
		if strings.Contains(prompt, "duration: 10s") || strings.Contains(prompt, "Think deeply") {
			delay = 2 * time.Second // Longer delay for explicit long-running tasks
		}

		select {
		case <-time.After(delay):
			// Task completed normally
		case <-ctx.Done():
			// Task was canceled
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
