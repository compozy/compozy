package instance

import (
	"context"
	"fmt"
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
	var lockReleaseErr error
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			lockReleaseErr = unlockErr
			o.logger.Error("Failed to release append lock", "error", unlockErr, "instance_id", o.instanceID)
		}
	}()

	// Calculate token count
	tokenCount := o.calculateTokenCount(ctx, msg)

	// Append to store
	if err := o.store.AppendMessage(ctx, o.instanceID, msg); err != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
		if lockReleaseErr != nil {
			return fmt.Errorf("failed to append message: %w (also failed to release lock: %v)", err, lockReleaseErr)
		}
		return err
	}

	// Update token count metadata
	if err := o.store.IncrementTokenCount(ctx, o.instanceID, tokenCount); err != nil {
		// Log but don't fail the operation
		o.recordMetadataError(ctx, "increment_token_count", err)
	}

	o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, nil)
	if lockReleaseErr != nil {
		return fmt.Errorf("operation completed but failed to release lock: %w", lockReleaseErr)
	}
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
	var lockReleaseErr error
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			lockReleaseErr = unlockErr
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
	if err != nil && lockReleaseErr != nil {
		return fmt.Errorf(
			"failed to append message with token count: %w (also failed to release lock: %v)",
			err,
			lockReleaseErr,
		)
	}
	if err != nil {
		return err
	}
	if lockReleaseErr != nil {
		return fmt.Errorf("operation completed but failed to release lock: %w", lockReleaseErr)
	}
	return nil
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
	var lockReleaseErr error
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			lockReleaseErr = unlockErr
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

	if err != nil && lockReleaseErr != nil {
		return fmt.Errorf("failed to clear messages: %w (also failed to release lock: %v)", err, lockReleaseErr)
	}
	if err != nil {
		return err
	}
	if lockReleaseErr != nil {
		return fmt.Errorf("operation completed but failed to release lock: %w", lockReleaseErr)
	}
	return nil
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

// estimateTokenCount provides a consistent fallback token estimation
func (o *Operations) estimateTokenCount(text string) int {
	// Rough estimate: 4 characters per token (common for most tokenizers)
	tokens := len(text) / 4
	// Ensure at least 1 token for non-empty text
	if tokens == 0 && text != "" {
		tokens = 1
	}
	return tokens
}

// calculateTokenCountWithFallback safely counts tokens with consistent fallback logic
func (o *Operations) calculateTokenCountWithFallback(ctx context.Context, text string, description string) int {
	if o.tokenCounter == nil {
		return o.estimateTokenCount(text)
	}
	count, err := o.tokenCounter.CountTokens(ctx, text)
	if err != nil {
		o.logger.Warn("Failed to count tokens, using fallback estimation",
			"error", err, "text_type", description, "instance_id", o.instanceID)
		return o.estimateTokenCount(text)
	}
	return count
}

// calculateTokenCount calculates tokens for a single message including role and structure overhead
func (o *Operations) calculateTokenCount(ctx context.Context, msg llm.Message) int {
	// Count content tokens with consistent fallback
	contentCount := o.calculateTokenCountWithFallback(ctx, msg.Content, "content")

	// Count role tokens with consistent fallback
	roleCount := o.calculateTokenCountWithFallback(ctx, string(msg.Role), "role")

	// Add structure overhead for message formatting
	structureOverhead := 2
	return contentCount + roleCount + structureOverhead
}

// calculateTokensFromMessages calculates total tokens from all messages with caching optimization
func (o *Operations) calculateTokensFromMessages(ctx context.Context) (int, error) {
	messages, err := o.store.ReadMessages(ctx, o.instanceID)
	if err != nil {
		return 0, err
	}

	// Use caching to reduce redundant token counting calls
	contentCache := make(map[string]int)
	roleCache := make(map[string]int)

	totalTokens := 0
	for _, msg := range messages {
		// Count content tokens with caching
		var contentCount int
		if count, exists := contentCache[msg.Content]; exists {
			contentCount = count
		} else {
			contentCount = o.calculateTokenCountWithFallback(ctx, msg.Content, "content")
			contentCache[msg.Content] = contentCount
		}

		// Count role tokens with caching
		roleStr := string(msg.Role)
		var roleCount int
		if count, exists := roleCache[roleStr]; exists {
			roleCount = count
		} else {
			roleCount = o.calculateTokenCountWithFallback(ctx, roleStr, "role")
			roleCache[roleStr] = roleCount
		}

		// Add structure overhead and accumulate
		structureOverhead := 2
		totalTokens += contentCount + roleCount + structureOverhead
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
