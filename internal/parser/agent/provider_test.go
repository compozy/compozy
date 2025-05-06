package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderGetAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		want     string
	}{
		{
			name:     "OpenAI Provider",
			provider: &OpenAIProvider{},
			want:     "https://api.openai.com/v1",
		},
		{
			name:     "Groq Provider",
			provider: &GroqProvider{},
			want:     "https://api.groq.com/openai/v1",
		},
		{
			name:     "Anthropic Provider",
			provider: &AnthropicProvider{},
			want:     "https://api.anthropic.com/v1",
		},
		{
			name:     "Mistral Provider",
			provider: &MistralProvider{},
			want:     "https://api.mixtral.ai/v1",
		},
		{
			name:     "Cohere Provider",
			provider: &CohereProvider{},
			want:     "https://api.cohere.ai/v1",
		},
		{
			name:     "Perplexity Provider",
			provider: &PerplexityProvider{},
			want:     "https://api.perplexity.ai/v1",
		},
		{
			name:     "XAI Provider",
			provider: &XAIProvider{},
			want:     "https://api.x.ai/v1",
		},
		{
			name:     "Google Provider",
			provider: &GoogleProvider{},
			want:     "https://generativelanguage.googleapis.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.provider.GetAPIURL()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderName
		want     Provider
	}{
		{
			name:     "OpenAI Provider",
			provider: ProviderOpenAI,
			want:     &OpenAIProvider{},
		},
		{
			name:     "Groq Provider",
			provider: ProviderGroq,
			want:     &GroqProvider{},
		},
		{
			name:     "Anthropic Provider",
			provider: ProviderAnthropic,
			want:     &AnthropicProvider{},
		},
		{
			name:     "Mistral Provider",
			provider: ProviderMistral,
			want:     &MistralProvider{},
		},
		{
			name:     "Cohere Provider",
			provider: ProviderCohere,
			want:     &CohereProvider{},
		},
		{
			name:     "Perplexity Provider",
			provider: ProviderPerplexity,
			want:     &PerplexityProvider{},
		},
		{
			name:     "XAI Provider",
			provider: ProviderXAI,
			want:     &XAIProvider{},
		},
		{
			name:     "Google Provider",
			provider: ProviderGoogle,
			want:     &GoogleProvider{},
		},
		{
			name:     "Unknown Provider",
			provider: "unknown",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetProvider(tt.provider)
			assert.Equal(t, tt.want, got)
		})
	}
}
