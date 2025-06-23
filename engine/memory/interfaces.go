package memory

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
)

// Health provides diagnostic information about memory state.
type Health struct {
	TokenCount    int        `json:"token_count"`
	MessageCount  int        `json:"message_count"`
	LastFlush     *time.Time `json:"last_flush,omitempty"`
	FlushStrategy string     `json:"flush_strategy"`
	// Potentially add lock status or other health indicators
}

// ClearFlushPendingFlagInput defines the input for the cleanup activity.
type ClearFlushPendingFlagInput struct {
	MemoryInstanceKey string
	MemoryResourceID  string
	ProjectID         string
}

// Memory defines the core interface for interaction with a memory instance.
// All operations are designed to be async-first.
// Implementations should be thread-safe for concurrent access.
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
	GetMemoryHealth(ctx context.Context) (*Health, error)

	// Clear removes all messages from the memory.
	// Useful for explicit cleanup or reset scenarios.
	Clear(ctx context.Context) error

	// GetID returns the unique identifier of this memory instance (usually the resolved key).
	GetID() string

	// AppendWithPrivacy adds a message to memory with privacy controls applied.
	// This is a privacy-aware variant of Append that handles redaction and selective persistence.
	AppendWithPrivacy(ctx context.Context, msg llm.Message, metadata PrivacyMetadata) error
}

// FlushableMemory extends the Memory interface with flush capabilities.
// This interface is implemented by memory instances that support flushing.
type FlushableMemory interface {
	Memory
	// PerformFlush executes the complete memory flush operation.
	PerformFlush(ctx context.Context) (*FlushMemoryActivityOutput, error)
	// MarkFlushPending sets or clears the flush pending flag for this instance.
	MarkFlushPending(ctx context.Context, pending bool) error
}

// Store defines the interface for the underlying persistence layer for memory.
// Implementations will handle the actual storage and retrieval of messages.
type Store interface {
	// AppendMessage appends a single message to the store under the given key.
	AppendMessage(ctx context.Context, key string, msg llm.Message) error
	// AppendMessageWithTokenCount atomically appends a message and updates token count metadata.
	// This should be implemented as an atomic operation to prevent race conditions.
	AppendMessageWithTokenCount(ctx context.Context, key string, msg llm.Message, tokenCount int) error
	// AppendMessages appends multiple messages to the store under the given key.
	// This should be an atomic operation if possible for the backend.
	AppendMessages(ctx context.Context, key string, msgs []llm.Message) error
	// ReadMessages retrieves all messages associated with the given key.
	ReadMessages(ctx context.Context, key string) ([]llm.Message, error)
	// CountMessages returns the number of messages for a given key.
	CountMessages(ctx context.Context, key string) (int, error)
	// TrimMessagesWithMetadata trims messages and updates metadata atomically.
	// This ensures token count and message count stay consistent after trimming.
	TrimMessagesWithMetadata(ctx context.Context, key string, keepCount int, newTokenCount int) error
	// ReplaceMessages replaces all messages for a key with a new set of messages.
	// Useful for operations like summarization where the history is rewritten.
	ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error
	// ReplaceMessagesWithMetadata atomically replaces all messages and updates metadata.
	// This ensures token count and message count stay in sync with the actual messages.
	ReplaceMessagesWithMetadata(ctx context.Context, key string, messages []llm.Message, totalTokens int) error
	// SetExpiration sets or updates the TTL for the given memory key.
	SetExpiration(ctx context.Context, key string, ttl time.Duration) error
	// DeleteMessages removes all messages associated with the given key.
	DeleteMessages(ctx context.Context, key string) error
	// GetKeyTTL returns the remaining time-to-live for a given key.
	// Returns a negative duration if the key does not exist or has no expiry.
	GetKeyTTL(ctx context.Context, key string) (time.Duration, error)

	// Performance optimized metadata operations
	// GetTokenCount retrieves the cached token count from metadata (O(1) operation).
	// Returns 0 and no error if metadata doesn't exist (requires migration).
	GetTokenCount(ctx context.Context, key string) (int, error)
	// GetMessageCount retrieves the cached message count from metadata (O(1) operation).
	// Returns 0 and no error if metadata doesn't exist (requires migration).
	GetMessageCount(ctx context.Context, key string) (int, error)
	// IncrementTokenCount atomically increments the token count metadata.
	IncrementTokenCount(ctx context.Context, key string, delta int) error
	// SetTokenCount sets the token count metadata to a specific value.
	SetTokenCount(ctx context.Context, key string, count int) error

	// Flush pending state management
	// IsFlushPending checks if a flush operation is currently pending for this key.
	IsFlushPending(ctx context.Context, key string) (bool, error)
	// MarkFlushPending sets or clears the flush pending flag for this key.
	// The implementation should ensure atomic operation and handle TTL appropriately.
	MarkFlushPending(ctx context.Context, key string, pending bool) error
}

// TokenCounter defines an interface for counting tokens in a given text,
// potentially specific to an LLM model's tokenizer.
type TokenCounter interface {
	CountTokens(ctx context.Context, text string) (int, error)
	// GetEncoding returns the name of the encoding being used (e.g., "cl100k_base").
	GetEncoding() string
}

// ManagerInterface defines the interface for the Manager.
// This interface allows for mocking in tests and provides flexibility in implementation.
type ManagerInterface interface {
	// GetInstance retrieves or creates a MemoryInstance based on a MemoryReference and workflow context.
	GetInstance(ctx context.Context, memRef core.MemoryReference, workflowContext map[string]any) (Memory, error)
}
