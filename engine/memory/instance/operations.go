package instance

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
)

// Operations handles memory operations like append, read, clear
type Operations struct {
	store        core.Store
	lockManager  LockManager
	tokenCounter core.TokenCounter
	metrics      Metrics
	instanceID   string
	logger       logger.Logger
}

// NewOperations creates a new operations handler
func NewOperations(
	instanceID string,
	store core.Store,
	lockManager LockManager,
	tokenCounter core.TokenCounter,
	metrics Metrics,
	logger logger.Logger,
) *Operations {
	return &Operations{
		instanceID:   instanceID,
		store:        store,
		lockManager:  lockManager,
		tokenCounter: tokenCounter,
		metrics:      metrics,
		logger:       logger,
	}
}

// AppendMessage appends a message with proper locking and metrics
func (o *Operations) AppendMessage(ctx context.Context, msg llm.Message) error {
	start := time.Now()

	// Acquire append lock
	unlock, err := o.lockManager.AcquireAppendLock(ctx, o.instanceID)
	if err != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), 0, err)
		return err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			o.logger.Error("Failed to release append lock", "error", unlockErr, "instance_id", o.instanceID)
		}
	}()

	// Calculate token count
	tokenCount := o.calculateTokenCount(ctx, msg)

	// Append to store
	if err := o.store.AppendMessage(ctx, o.instanceID, msg); err != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
		return err
	}

	// Update token count metadata
	if err := o.store.IncrementTokenCount(ctx, o.instanceID, tokenCount); err != nil {
		// Log but don't fail the operation
		o.recordMetadataError(ctx, "increment_token_count", err)
	}

	o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, nil)
	return nil
}

// AppendMessageWithTokenCount appends a message and updates token count atomically
func (o *Operations) AppendMessageWithTokenCount(ctx context.Context, msg llm.Message, tokenCount int) error {
	start := time.Now()

	// Acquire append lock
	unlock, err := o.lockManager.AcquireAppendLock(ctx, o.instanceID)
	if err != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), 0, err)
		return err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			o.logger.Error("Failed to release append lock", "error", unlockErr, "instance_id", o.instanceID)
		}
	}()

	// Use atomic operation if available
	if atomicStore, ok := o.store.(core.AtomicOperations); ok {
		err = atomicStore.AppendMessageWithTokenCount(ctx, o.instanceID, msg, tokenCount)
	} else {
		// Fallback to separate operations
		if err = o.store.AppendMessage(ctx, o.instanceID, msg); err == nil {
			err = o.store.IncrementTokenCount(ctx, o.instanceID, tokenCount)
		}
	}

	o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
	return err
}

// ReadMessages retrieves all messages
func (o *Operations) ReadMessages(ctx context.Context) ([]llm.Message, error) {
	start := time.Now()

	messages, err := o.store.ReadMessages(ctx, o.instanceID)
	messageCount := len(messages)

	// Calculate total tokens for metrics
	totalTokens := 0
	if err == nil {
		for _, msg := range messages {
			totalTokens += o.calculateTokenCount(ctx, msg)
		}
	}

	o.metrics.RecordRead(ctx, time.Since(start), messageCount, err)
	return messages, err
}

// ClearMessages removes all messages with proper locking
func (o *Operations) ClearMessages(ctx context.Context) error {
	// Acquire clear lock
	unlock, err := o.lockManager.AcquireClearLock(ctx, o.instanceID)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			o.logger.Error("Failed to release clear lock", "error", unlockErr, "instance_id", o.instanceID)
		}
	}()

	// Clear messages
	err = o.store.DeleteMessages(ctx, o.instanceID)

	// Reset token count
	if err == nil {
		if setErr := o.store.SetTokenCount(ctx, o.instanceID, 0); setErr != nil {
			o.recordMetadataError(ctx, "reset_token_count", setErr)
		}
	}

	return err
}

// GetMessageCount returns the number of messages
func (o *Operations) GetMessageCount(ctx context.Context) (int, error) {
	return o.store.GetMessageCount(ctx, o.instanceID)
}

// GetTokenCount returns the current token count
func (o *Operations) GetTokenCount(ctx context.Context) (int, error) {
	tokenCount, err := o.store.GetTokenCount(ctx, o.instanceID)
	if err != nil {
		return 0, err
	}

	// If token count is 0, it might need migration
	if tokenCount == 0 {
		// Check if there are messages
		messageCount, err := o.store.GetMessageCount(ctx, o.instanceID)
		if err != nil {
			return 0, err
		}

		if messageCount > 0 {
			// Need to calculate token count from messages
			return o.calculateTokensFromMessages(ctx)
		}
	}

	return tokenCount, nil
}

// calculateTokenCount calculates tokens for a single message
func (o *Operations) calculateTokenCount(ctx context.Context, msg llm.Message) int {
	if o.tokenCounter == nil {
		return len(msg.Content) / 4 // Rough estimate
	}

	count, err := o.tokenCounter.CountTokens(ctx, msg.Content)
	if err != nil {
		// Fallback to rough estimate
		return len(msg.Content) / 4
	}

	return count
}

// calculateTokensFromMessages calculates total tokens from all messages
func (o *Operations) calculateTokensFromMessages(ctx context.Context) (int, error) {
	messages, err := o.store.ReadMessages(ctx, o.instanceID)
	if err != nil {
		return 0, err
	}

	totalTokens := 0
	for _, msg := range messages {
		totalTokens += o.calculateTokenCount(ctx, msg)
	}

	// Update the metadata for future use
	if err := o.store.SetTokenCount(ctx, o.instanceID, totalTokens); err != nil {
		o.recordMetadataError(ctx, "set_token_count_migration", err)
	}

	return totalTokens, nil
}

// recordMetadataError logs metadata operation errors without failing the main operation
func (o *Operations) recordMetadataError(_ context.Context, operation string, err error) {
	// Log metadata operation errors
	o.logger.Error("Metadata operation failed",
		"operation", operation,
		"error", err,
		"instance_id", o.instanceID)
}
