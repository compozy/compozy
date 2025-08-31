package core

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
)

// Memory defines the core interface for interaction with a memory instance.
// All operations are designed to be async-first.
// Implementations should be thread-safe for concurrent access.
type Memory interface {
	// Append adds a message to the memory.
	Append(ctx context.Context, msg llm.Message) error
	// AppendMany atomically adds multiple messages to the memory.
	// This ensures all messages are stored together or none are stored.
	AppendMany(ctx context.Context, msgs []llm.Message) error
	// Read retrieves all messages from the memory.
	// Implementations should handle ordering (e.g., chronological).
	Read(ctx context.Context) ([]llm.Message, error)
	// ReadPaginated retrieves messages from memory with pagination support.
	// Returns the requested slice of messages and the total count for pagination metadata.
	ReadPaginated(ctx context.Context, offset, limit int) ([]llm.Message, int, error)
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

// DynamicFlushableMemory extends FlushableMemory with dynamic strategy support.
// This interface allows memory instances to flush with a specific strategy per request,
// overriding the configured default strategy.
type DynamicFlushableMemory interface {
	FlushableMemory
	// PerformFlushWithStrategy executes flush with a specific strategy type.
	// If strategyType is empty, uses the configured default strategy.
	PerformFlushWithStrategy(ctx context.Context, strategyType FlushingStrategyType) (*FlushMemoryActivityOutput, error)
	// GetConfiguredStrategy returns the default configured strategy type.
	GetConfiguredStrategy() FlushingStrategyType
}

// Store defines the interface for the underlying persistence layer for memory.
// This is now a composite interface that includes all store operations.
// Implementations will handle the actual storage and retrieval of messages.
type Store interface {
	// Basic storage operations
	MessageStore
	MetadataStore
	ExpirationStore
	FlushStateStore

	// Atomic operations
	AtomicOperations
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
	// ReadMessagesPaginated retrieves messages associated with the given key with pagination support.
	// Returns the requested slice of messages and the total count for pagination metadata.
	ReadMessagesPaginated(ctx context.Context, key string, offset, limit int) ([]llm.Message, int, error)
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

// AtomicOperations provides atomic operations that combine message and metadata updates
type AtomicOperations interface {
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

// Health provides diagnostic information about memory state.
type Health struct {
	TokenCount     int        `json:"token_count"`
	MessageCount   int        `json:"message_count"`
	LastFlush      *time.Time `json:"last_flush,omitempty"`
	ActualStrategy string     `json:"actual_strategy"`
	// Potentially add lock status or other health indicators
}

// PrivacyMetadata contains privacy-related metadata for a message
type PrivacyMetadata struct {
	// DoNotPersist indicates this message should not be stored
	DoNotPersist bool `json:"do_not_persist"             yaml:"do_not_persist"`
	// SensitiveFields lists fields that contain sensitive data
	SensitiveFields []string `json:"sensitive_fields,omitempty" yaml:"sensitive_fields,omitempty"`
	// RedactionApplied indicates if redaction was already applied
	RedactionApplied bool `json:"redaction_applied"          yaml:"redaction_applied"`
	// PrivacyLevel indicates the sensitivity level (e.g., "public", "private", "confidential")
	PrivacyLevel string `json:"privacy_level,omitempty"    yaml:"privacy_level,omitempty"`
}

// FlushMemoryActivityInput contains the necessary information to perform a memory flush.
type FlushMemoryActivityInput struct {
	MemoryInstanceKey    string // The unique key for the memory instance (resolved key)
	MemoryResourceID     string // The ID of the MemoryResource config
	ProjectID            string // The project ID for namespacing
	ForceFlush           bool   // Whether to force flush regardless of conditions
	ReportProgressTaskID string // Optional: Task ID for progress reporting
}

// FlushMemoryActivityOutput contains the result of a memory flush operation.
type FlushMemoryActivityOutput struct {
	Success          bool   `json:"success"`
	SummaryGenerated bool   `json:"summary_generated"`
	MessageCount     int    `json:"message_count"`
	TokenCount       int    `json:"token_count"`
	Error            string `json:"error,omitempty"`
}

// ClearFlushPendingFlagInput defines the input for the cleanup activity.
type ClearFlushPendingFlagInput struct {
	MemoryInstanceKey string
	MemoryResourceID  string
	ProjectID         string
}

// FlushStrategy defines the interface for different flushing strategies
type FlushStrategy interface {
	// ShouldFlush determines if a flush should be triggered based on current state
	ShouldFlush(tokenCount, messageCount int, config *Resource) bool
	// PerformFlush executes the flush operation
	PerformFlush(
		ctx context.Context,
		messages []llm.Message,
		config *Resource,
	) (*FlushMemoryActivityOutput, error)
	// GetType returns the strategy type
	GetType() FlushingStrategyType
}
