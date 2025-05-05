package agent

import (
	"encoding/json"
)

// ProviderName represents the name of a provider
type ProviderName string

const (
	ProviderOpenAI ProviderName = "openai"
	ProviderGroq   ProviderName = "groq"
)

// ModelName represents the name of a model
type ModelName string

const (
	// GPT-4 models
	ModelGPT4o               ModelName = "gpt-4o"
	ModelGPT4oMini           ModelName = "gpt-4o-mini"
	ModelO1Mini              ModelName = "o1-mini"
	ModelO3Mini              ModelName = "o3-mini"
	ModelLLama3370bVersatile ModelName = "llama-3.3-70b-versatile"
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
	Role    MessageRole    `json:"role" yaml:"role"`
	Content MessageContent `json:"content" yaml:"content"`
}

// ProviderConfig represents provider-specific configuration options
type ProviderConfig struct {
	Provider         ProviderName      `json:"provider" yaml:"provider"`
	Model            ModelName         `json:"model" yaml:"model"`
	APIKey           APIKey            `json:"api_key" yaml:"api_key"`
	Temperature      *Temperature      `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens        *MaxTokens        `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	TopP             *TopP             `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	FrequencyPenalty *FrequencyPenalty `json:"frequency_penalty,omitempty" yaml:"frequency_penalty,omitempty"`
	PresencePenalty  *PresencePenalty  `json:"presence_penalty,omitempty" yaml:"presence_penalty,omitempty"`
}

// AsJSON converts the provider configuration to a JSON value
func (p *ProviderConfig) AsJSON() (json.RawMessage, error) {
	return json.Marshal(p)
}
