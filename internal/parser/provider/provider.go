package provider

import (
	"encoding/json"
)

// MessageRole represents the role of a message
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// Message represents a message configuration
type Message struct {
	Role    MessageRole `json:"role" yaml:"role"`
	Content string      `json:"content" yaml:"content"`
}

// Config represents provider-specific configuration options
type Config struct {
	Provider         Name      `json:"provider" yaml:"provider"`
	Model            ModelName `json:"model" yaml:"model"`
	APIKey           string    `json:"api_key" yaml:"api_key"`
	APIURL           string    `json:"api_url" yaml:"api_url"`
	Temperature      float32   `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens        int32     `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	TopP             float32   `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	FrequencyPenalty float32   `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty  float32   `json:"presence_penalty,omitempty" yaml:"presence_penalty,omitempty"`
}

// AsJSON converts the provider configuration to a JSON value
func (p *Config) AsJSON() (json.RawMessage, error) {
	return json.Marshal(p)
}

// NewProviderConfig creates a new ProviderConfig with the API URL populated
func NewProviderConfig(provider Name, model ModelName, apiKey string) *Config {
	config := &Config{
		Provider: provider,
		Model:    model,
		APIKey:   apiKey,
	}
	// Populate APIURL using the Provider interface
	if p := GetProvider(provider); p != nil {
		config.APIURL = p.GetAPIURL()
	}
	return config
}
