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
	// Read retrieves all messages from the memory.
	// Implementations should handle ordering (e.g., chronological).
	Read(ctx context.Context) ([]Message, error)
	// GetID returns the unique identifier of this memory instance (usually the resolved key).
	GetID() string
}
