package llm

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
)

type ModelConfig struct {
	Instructions string // System-level instructions for the LLM
}

type PromptConfig struct {
	Prompt string // Base prompt template
	Input  *core.Input
}

type Service interface {
	CreateLLM(config *ProviderConfig) (llms.Model, error)
	GenerateContent(
		ctx context.Context,
		model llms.Model,
		instructions string,
		prompt string,
	) (core.Output, error)
	BuildPrompt(
		config *PromptConfig,
	) string
}

type MultiProvider struct{}

func NewLLMService() Service {
	return &MultiProvider{}
}

func (p *MultiProvider) CreateLLM(config *ProviderConfig) (llms.Model, error) {
	switch config.Provider {
	case ProviderOpenAI:
		return p.createOpenAILLM(config)
	case ProviderAnthropic:
		return p.createAnthropicLLM(config)
	case ProviderGroq:
		return p.createGroqLLM(config)
	case ProviderMock:
		return p.createMockLLM(config)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}

func (p *MultiProvider) GenerateContent(
	ctx context.Context,
	model llms.Model,
	instructions string,
	prompt string,
) (core.Output, error) {
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, instructions),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}
	response, err := model.GenerateContent(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response choices generated")
	}
	responseText := response.Choices[0].Content
	result := core.Output{
		"response": responseText,
	}
	return result, nil
}

func (p *MultiProvider) BuildPrompt(
	config *PromptConfig,
) string {
	prompt := config.Prompt
	input := config.Input
	if input != nil {
		for key, value := range *input {
			prompt = fmt.Sprintf("%s\n\n%s: %v", prompt, key, value)
		}
	}
	return prompt
}

func (p *MultiProvider) createOpenAILLM(config *ProviderConfig) (llms.Model, error) {
	opts := []openai.Option{
		openai.WithModel(string(config.Model)),
	}
	if config.APIKey != "" {
		opts = append(opts, openai.WithToken(config.APIKey))
	}
	if config.APIURL != "" {
		opts = append(opts, openai.WithBaseURL(config.APIURL))
	}
	return openai.New(opts...)
}

func (p *MultiProvider) createAnthropicLLM(config *ProviderConfig) (llms.Model, error) {
	opts := []anthropic.Option{
		anthropic.WithModel(string(config.Model)),
	}
	if config.APIKey != "" {
		opts = append(opts, anthropic.WithToken(config.APIKey))
	}
	return anthropic.New(opts...)
}

func (p *MultiProvider) createGroqLLM(config *ProviderConfig) (llms.Model, error) {
	opts := []openai.Option{
		openai.WithModel(string(config.Model)),
		openai.WithBaseURL("https://api.groq.com/openai/v1"),
	}
	if config.APIKey != "" {
		opts = append(opts, openai.WithToken(config.APIKey))
	}
	return openai.New(opts...)
}

func (p *MultiProvider) createMockLLM(config *ProviderConfig) (llms.Model, error) {
	return NewMockLLM(string(config.Model)), nil
}
