package instance

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
)

// Instance represents a memory instance with all its components
type Instance interface {
	core.Memory
	core.FlushableMemory

	// Additional instance-specific methods
	GetResource() *core.Resource
	GetStore() core.Store
	GetTokenCounter() core.TokenCounter
	GetMetrics() Metrics
	GetLockManager() LockManager

	// Close gracefully shuts down the instance, flushing any pending operations
	Close(ctx context.Context) error
}

// Metrics provides metrics and telemetry for memory operations
type Metrics interface {
	// RecordAppend records metrics for an append operation
	RecordAppend(ctx context.Context, duration time.Duration, tokenCount int, err error)
	// RecordRead records metrics for a read operation
	RecordRead(ctx context.Context, duration time.Duration, messageCount int, err error)
	// RecordFlush records metrics for a flush operation
	RecordFlush(ctx context.Context, duration time.Duration, messagesFlushed int, err error)
	// RecordTokenCount records the current token count
	RecordTokenCount(ctx context.Context, count int)
	// RecordMessageCount records the current message count
	RecordMessageCount(ctx context.Context, count int)
}

// LockManager handles distributed locking for memory operations
type LockManager interface {
	// AcquireAppendLock acquires a lock for append operations
	AcquireAppendLock(ctx context.Context, key string) (UnlockFunc, error)
	// AcquireClearLock acquires a lock for clear operations
	AcquireClearLock(ctx context.Context, key string) (UnlockFunc, error)
	// AcquireFlushLock acquires a lock for flush operations
	AcquireFlushLock(ctx context.Context, key string) (UnlockFunc, error)
}

// UnlockFunc is a function that releases a lock
type UnlockFunc func() error

// EvictionPolicy defines the interface for message eviction strategies
type EvictionPolicy interface {
	// SelectMessagesToEvict selects which messages should be evicted
	SelectMessagesToEvict(messages []llm.Message, targetCount int) []llm.Message
	// GetType returns the policy type
	GetType() string
}

// AsyncTokenCounter provides asynchronous token counting
type AsyncTokenCounter interface {
	// ProcessAsync queues a message for token counting without blocking
	ProcessAsync(ctx context.Context, memoryRef string, text string)
	// ProcessWithResult queues a message and waits for the result
	ProcessWithResult(ctx context.Context, memoryRef string, text string) (int, error)
	// Shutdown gracefully stops the async counter
	Shutdown()
}
