package agent

// AgentID represents an agent identifier
type AgentID string

// APIKey represents an API key
type APIKey string

// ProviderURL represents a provider URL
type ProviderURL string

// ActionID represents an action identifier
type ActionID string

// Instructions represents agent instructions
type Instructions string

// MessageContent represents message content
type MessageContent string

// ActionPrompt represents an action prompt
type ActionPrompt string

// Temperature represents a temperature value
type Temperature float32

// MaxTokens represents a max tokens value
type MaxTokens uint32

// TopP represents a top-p value
type TopP float32

// FrequencyPenalty represents a frequency penalty value
type FrequencyPenalty float32

// PresencePenalty represents a presence penalty value
type PresencePenalty float32

// ActionResponse represents a response from an agent
type ActionResponse struct {
	Response map[string]interface{} `json:"response" yaml:"response"`
}
