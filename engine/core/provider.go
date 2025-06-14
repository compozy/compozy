package core

import (
	"encoding/json"

	"dario.cat/mergo"
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
