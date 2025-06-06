package llm

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
