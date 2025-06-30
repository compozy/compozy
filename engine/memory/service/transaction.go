package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// MemoryTransaction provides transactional operations for memory modifications
type MemoryTransaction struct {
	mu      sync.RWMutex
	mem     memcore.Memory
	backup  []llm.Message
	cleared bool
}

// NewMemoryTransaction creates a new memory transaction
func NewMemoryTransaction(mem memcore.Memory) *MemoryTransaction {
	return &MemoryTransaction{
		mem: mem,
	}
}

// Begin starts the transaction by backing up current state
func (t *MemoryTransaction) Begin(ctx context.Context) error {
	// Backup current messages
	// NOTE: This creates a full copy of all messages which may cause memory
	// spikes for very large memory instances. This is a design trade-off for
	// ensuring reliable rollback capability. Future optimization could consider
	// alternative strategies for very large datasets.
	backup, err := t.mem.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to backup messages: %w", err)
	}
	t.mu.Lock()
	t.backup = backup
	t.mu.Unlock()
	return nil
}

// Clear clears the memory and marks it for potential rollback
func (t *MemoryTransaction) Clear(ctx context.Context) error {
	if err := t.mem.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear memory: %w", err)
	}
	t.mu.Lock()
	t.cleared = true
	t.mu.Unlock()
	return nil
}

// Commit finalizes the transaction (no-op for successful operations)
func (t *MemoryTransaction) Commit() error {
	// Reset state
	t.mu.Lock()
	t.backup = nil
	t.cleared = false
	t.mu.Unlock()
	return nil
}

// Rollback restores the original state
func (t *MemoryTransaction) Rollback(ctx context.Context) error {
	t.mu.RLock()
	backup := t.backup
	t.mu.RUnlock()

	if backup == nil {
		return nil // Nothing to rollback
	}

	// Clear any partial state
	if err := t.mem.Clear(ctx); err != nil {
		return fmt.Errorf("rollback clear failed: %w", err)
	}

	// Restore backup messages
	for i, msg := range backup {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("rollback canceled at message %d: %w", i, ctx.Err())
		default:
		}

		if err := t.mem.Append(ctx, msg); err != nil {
			return fmt.Errorf("rollback failed at message %d: %w", i, err)
		}
	}

	return nil
}

// ApplyMessages appends messages within the transaction
func (t *MemoryTransaction) ApplyMessages(ctx context.Context, messages []llm.Message) error {
	for i, msg := range messages {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation canceled at message %d: %w", i, ctx.Err())
		default:
		}

		if err := t.mem.Append(ctx, msg); err != nil {
			return fmt.Errorf("failed to append message %d: %w", i, err)
		}
	}
	return nil
}
