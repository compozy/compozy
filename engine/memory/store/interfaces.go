package store

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/llm"
)

// Store defines the interface for the underlying persistence layer for memory.
// This is a segregated interface focused on basic storage operations.
// Implementations will handle the actual storage and retrieval of messages.
type Store interface {
	MessageStore
	MetadataStore
	ExpirationStore
	FlushStateStore
}

// MessageStore handles core message storage operations
type MessageStore interface {
	// AppendMessage appends a single message to the store under the given key.
	AppendMessage(ctx context.Context, key string, msg llm.Message) error
	// AppendMessages appends multiple messages to the store under the given key.
	// This should be an atomic operation if possible for the backend.
	AppendMessages(ctx context.Context, key string, msgs []llm.Message) error
	// ReadMessages retrieves all messages associated with the given key.
	ReadMessages(ctx context.Context, key string) ([]llm.Message, error)
	// ReplaceMessages replaces all messages for a key with a new set of messages.
	// Useful for operations like summarization where the history is rewritten.
	ReplaceMessages(ctx context.Context, key string, messages []llm.Message) error
	// DeleteMessages removes all messages associated with the given key.
	DeleteMessages(ctx context.Context, key string) error
}

// MetadataStore handles metadata operations for performance optimization
type MetadataStore interface {
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
}

// ExpirationStore handles TTL and expiration operations
type ExpirationStore interface {
	// SetExpiration sets or updates the TTL for the given memory key.
	SetExpiration(ctx context.Context, key string, ttl time.Duration) error
	// GetKeyTTL returns the remaining time-to-live for a given key.
	// Returns a negative duration if the key does not exist or has no expiry.
	GetKeyTTL(ctx context.Context, key string) (time.Duration, error)
}

// FlushStateStore handles flush pending state management
type FlushStateStore interface {
	// IsFlushPending checks if a flush operation is currently pending for this key.
	IsFlushPending(ctx context.Context, key string) (bool, error)
	// MarkFlushPending sets or clears the flush pending flag for this key.
	// The implementation should ensure atomic operation and handle TTL appropriately.
	MarkFlushPending(ctx context.Context, key string, pending bool) error
}

// AtomicStore provides atomic operations that combine message and metadata updates
type AtomicStore interface {
	Store
	// AppendMessageWithTokenCount atomically appends a message and updates token count metadata.
	// This should be implemented as an atomic operation to prevent race conditions.
	AppendMessageWithTokenCount(ctx context.Context, key string, msg llm.Message, tokenCount int) error
	// TrimMessagesWithMetadata trims messages and updates metadata atomically.
	// This ensures token count and message count stay consistent after trimming.
	TrimMessagesWithMetadata(ctx context.Context, key string, keepCount int, newTokenCount int) error
	// ReplaceMessagesWithMetadata atomically replaces all messages and updates metadata.
	// This ensures token count and message count stay in sync with the actual messages.
	ReplaceMessagesWithMetadata(ctx context.Context, key string, messages []llm.Message, totalTokens int) error
	// CountMessages returns the number of messages for a given key.
	CountMessages(ctx context.Context, key string) (int, error)
}
