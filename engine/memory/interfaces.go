package memory

import (
	"context"
	"time"

	"github.com/CompoZy/llm-router/engine/llm" // Assuming llm.Message is here
)

// MemoryHealth provides diagnostic information about memory state.
type MemoryHealth struct {
	TokenCount    int        `json:"token_count"`
	MessageCount  int        `json:"message_count"`
	LastFlush     *time.Time `json:"last_flush,omitempty"`
	FlushStrategy string     `json:"flush_strategy"`
	// Potentially add lock status or other health indicators
}

// Memory defines the core interface for interaction with a memory instance.
// All operations are designed to be async-first.
type Memory interface {
	// Append adds a message to the memory.
	Append(ctx context.Context, msg llm.Message) error
	// Read retrieves all messages from the memory.
	// Implementations should handle ordering (e.g., chronological).
	Read(ctx context.Context) ([]llm.Message, error)
	// Len returns the number of messages in the memory.
	Len(ctx context.Context) (int, error)

	// GetTokenCount returns the current estimated token count of the messages in memory.
	GetTokenCount(ctx context.Context) (int, error)
	// GetMemoryHealth returns diagnostic information about the memory instance.
	GetMemoryHealth(ctx context.Context) (*MemoryHealth, error)

	// Clear removes all messages from the memory.
	// Useful for explicit cleanup or reset scenarios.
	Clear(ctx context.Context) error

	// GetID returns the unique identifier of this memory instance (usually the resolved key).
	GetID() string
}

// MemoryStore defines the interface for the underlying persistence layer for memory.
// Implementations will handle the actual storage and retrieval of messages.
type MemoryStore interface {
	// AppendMessage appends a single message to the store under the given key.
	AppendMessage(ctx context.Context, key string, msg llm.Message) error
	// AppendMessages appends multiple messages to the store under the given key.
	// This should be an atomic operation if possible for the backend.
	AppendMessages(ctx context.Context, key string, msgs []llm.Message) error
	// ReadMessages retrieves all messages associated with the given key.
	ReadMessages(ctx context.Context, key string) ([]llm.Message, error)
	// CountMessages returns the number of messages for a given key.
	CountMessages(ctx context.Context, key string) (int, error)
	// TrimMessages removes messages from the store to keep only `keepCount` newest messages.
	// For FIFO eviction, this would remove from the start of the list.
	TrimMessages(ctx context.Context, key string, keepCount int) error
	// ReplaceMessages replaces all messages for a key with a new set of messages.
	// Useful for operations like summarization where the history is rewritten.
	ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error
	// SetExpiration sets or updates the TTL for the given memory key.
	SetExpiration(ctx context.Context, key string, ttl time.Duration) error
	// DeleteMessages removes all messages associated with the given key.
	DeleteMessages(ctx context.Context, key string) error
	// GetKeyTTL returns the remaining time-to-live for a given key.
	// Returns a negative duration if the key does not exist or has no expiry.
	GetKeyTTL(ctx context.Context, key string) (time.Duration, error)
}

// TokenCounter defines an interface for counting tokens in a given text,
// potentially specific to an LLM model's tokenizer.
type TokenCounter interface {
	CountTokens(ctx context.Context, text string) (int, error)
	// GetEncoding returns the name of the encoding being used (e.g., "cl100k_base").
	GetEncoding() string
}
