package llm

import (
	"context"
)

// MemoryProvider defines the interface for providing memory instances to the LLM orchestrator.
// This interface is implemented by the task execution layer to avoid import cycles.
type MemoryProvider interface {
	// GetMemory retrieves a memory instance by ID and key template.
	// The provider is responsible for resolving the template.
	// Returns nil if no memory is configured or available.
	GetMemory(ctx context.Context, memoryID string, keyTemplate string) (Memory, error)
}

// Memory defines the core interface for interaction with a memory instance.
// This is a subset of the full memory interface to avoid import cycles.
// All operations are designed to be async-first.
type Memory interface {
	// Append adds a message to the memory.
	Append(ctx context.Context, msg Message) error
	// AppendMany appends multiple messages with well-defined semantics:
	// - Empty input: msgs == nil or len(msgs) == 0 is a no-op; returns nil.
	// - Ordering: preserves input order; messages are persisted in the same order
	//   as provided in msgs.
	// - Atomicity: all-or-none. Either all messages are persisted or none are. If
	//   the implementation cannot uphold atomicity (e.g., backend limitation or
	//   failure mid-batch), it must return an error instead of leaving partial
	//   writes. Compozy memory implementations return a memory-specific error
	//   (e.g., *memory/core.MemoryError with code MEMORY_APPEND_ERROR) to indicate
	//   this failure.
	// - Idempotency: retries are safe in the sense that each AppendMany call is
	//   applied atomically as a single unit. However, without idempotency keys or
	//   unique message IDs, repeated successful calls can append duplicate batches.
	//   Callers that require exactly-once semantics must implement deduplication
	//   at a higher level.
	AppendMany(ctx context.Context, msgs []Message) error
	// Read retrieves all messages from the memory.
	// Implementations should handle ordering (e.g., chronological).
	Read(ctx context.Context) ([]Message, error)
	// GetID returns the unique identifier of this memory instance (usually the resolved key).
	GetID() string
}
