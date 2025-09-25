package contracts

import "context"

// MemoryProvider defines the interface for providing memory instances to the orchestrator.
type MemoryProvider interface {
	GetMemory(ctx context.Context, memoryID string, keyTemplate string) (Memory, error)
}

// Memory defines the interaction contract with a memory instance.
type Memory interface {
	Append(ctx context.Context, msg Message) error
	AppendMany(ctx context.Context, msgs []Message) error
	Read(ctx context.Context) ([]Message, error)
	GetID() string
}

// MessageRole represents the role of a message stored in memory.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// Message represents a memory message payload.
type Message struct {
	Role    MessageRole `json:"role"    yaml:"role"`
	Content string      `json:"content" yaml:"content"`
}
