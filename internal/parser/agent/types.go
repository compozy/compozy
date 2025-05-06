package agent

type AgentID string
type ActionID string
type ActionPrompt string
type Instructions string

type ActionResponse struct {
	Response map[string]any `json:"response" yaml:"response"`
}
