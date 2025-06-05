package llm

import (
	"encoding/json"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/core"
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
	Role    MessageRole `json:"role"    yaml:"role"`
	Content string      `json:"content" yaml:"content"`
}

// ProviderConfig represents provider-specific configuration options
type ProviderConfig struct {
	Provider         ProviderName `json:"provider"                    yaml:"provider"                    mapstructure:"provider"`
	Model            ModelName    `json:"model"                       yaml:"model"                       mapstructure:"model"`
	APIKey           string       `json:"api_key"                     yaml:"api_key"                     mapstructure:"api_key"`
	APIURL           string       `json:"api_url"                     yaml:"api_url"                     mapstructure:"api_url"`
	Temperature      float32      `json:"temperature,omitempty"       yaml:"temperature,omitempty"       mapstructure:"temperature,omitempty"`
	MaxTokens        int32        `json:"max_tokens,omitempty"        yaml:"max_tokens,omitempty"        mapstructure:"max_tokens,omitempty"`
	TopP             float32      `json:"top_p,omitempty"             yaml:"top_p,omitempty"             mapstructure:"top_p,omitempty"`
	FrequencyPenalty float32      `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty" mapstructure:"frequency_penalty,omitempty"`
	PresencePenalty  float32      `json:"presence_penalty,omitempty"  yaml:"presence_penalty,omitempty"  mapstructure:"presence_penalty,omitempty"`
}

// AsJSON converts the provider configuration to a JSON value
func (p *ProviderConfig) AsJSON() (json.RawMessage, error) {
	return json.Marshal(p)
}

// AsMap converts the provider configuration to a map for template normalization
func (p *ProviderConfig) AsMap() (map[string]any, error) {
	return core.AsMapDefault(p)
}

// FromMap updates the provider configuration from a normalized map
func (p *ProviderConfig) FromMap(data any) error {
	config, err := core.FromMapDefault[ProviderConfig](data)
	if err != nil {
		return err
	}
	return mergo.Merge(p, config, mergo.WithOverride)
}

// NewProviderConfig creates a new ProviderConfig with the API URL populated
func NewProviderConfig(provider ProviderName, model ModelName, apiKey string) *ProviderConfig {
	config := &ProviderConfig{
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
