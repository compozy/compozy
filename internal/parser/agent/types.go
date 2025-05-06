package agent

type AgentID string
type APIKey string
type ProviderURL string
type ActionID string
type Instructions string
type MessageContent string
type ActionPrompt string
type Temperature float32
type MaxTokens uint32
type TopP float32
type FrequencyPenalty float32
type PresencePenalty float32

type ActionResponse struct {
	Response map[string]any `json:"response" yaml:"response"`
}
