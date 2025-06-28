package service

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// MemoryTransaction provides transactional operations for memory modifications
type MemoryTransaction struct {
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
	t.backup = backup
	return nil
}

// Clear clears the memory and marks it for potential rollback
func (t *MemoryTransaction) Clear(ctx context.Context) error {
	if err := t.mem.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear memory: %w", err)
	}
	t.cleared = true
	return nil
}

// Commit finalizes the transaction (no-op for successful operations)
func (t *MemoryTransaction) Commit() error {
	// Reset state
	t.backup = nil
	t.cleared = false
	return nil
}

// Rollback restores the original state
func (t *MemoryTransaction) Rollback(ctx context.Context) error {
	if t.backup == nil {
		return nil // Nothing to rollback
	}

	// Clear any partial state
	if err := t.mem.Clear(ctx); err != nil {
		return fmt.Errorf("rollback clear failed: %w", err)
	}

	// Restore backup messages
	for i, msg := range t.backup {
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
