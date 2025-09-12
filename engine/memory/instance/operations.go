package instance

import (
	"context"
	"fmt"
	"sync"
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
	ctx context.Context,
	instanceID string,
	store core.Store,
	lockManager LockManager,
	tokenCounter core.TokenCounter,
	metrics Metrics,
) *Operations {
	return &Operations{
		instanceID:   instanceID,
		store:        store,
		lockManager:  lockManager,
		tokenCounter: tokenCounter,
		metrics:      metrics,
		logger:       logger.FromContext(ctx),
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
		lockReleaseErr = o.handleLockRelease(unlock, "append")
	}()

	// Calculate token count
	tokenCount := o.calculateTokenCount(ctx, msg)

	// Append to store
	var operationErr error
	if err := o.store.AppendMessage(ctx, o.instanceID, msg); err != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
		operationErr = err
		return o.combineErrors(operationErr, lockReleaseErr, "append message")
	}

	// Update token count metadata
	if err := o.store.IncrementTokenCount(ctx, o.instanceID, tokenCount); err != nil {
		// Log but don't fail the operation
		o.recordMetadataError(ctx, "increment_token_count", err)
	}

	o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, nil)
	return o.combineErrors(nil, lockReleaseErr, "append message")
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
		lockReleaseErr = o.handleLockRelease(unlock, "append")
	}()

	// Use atomic operation if available
	var operationErr error
	if atomicStore, ok := o.store.(core.AtomicOperations); ok {
		operationErr = atomicStore.AppendMessageWithTokenCount(ctx, o.instanceID, msg, tokenCount)
	} else {
		// Fallback to separate operations
		if err := o.store.AppendMessage(ctx, o.instanceID, msg); err != nil {
			operationErr = err
		} else if err := o.store.IncrementTokenCount(ctx, o.instanceID, tokenCount); err != nil {
			operationErr = err
		}
	}

	o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, operationErr)
	return o.combineErrors(operationErr, lockReleaseErr, "append message with token count")
}

// ReadMessages retrieves all messages
func (o *Operations) ReadMessages(ctx context.Context) ([]llm.Message, error) {
	start := time.Now()

	messages, err := o.store.ReadMessages(ctx, o.instanceID)
	messageCount := len(messages)

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
		lockReleaseErr = o.handleLockRelease(unlock, "clear")
	}()

	// Clear messages
	operationErr := o.store.DeleteMessages(ctx, o.instanceID)

	// Reset token count
	if operationErr == nil {
		if setErr := o.store.SetTokenCount(ctx, o.instanceID, 0); setErr != nil {
			o.recordMetadataError(ctx, "reset_token_count", setErr)
		}
	}

	return o.combineErrors(operationErr, lockReleaseErr, "clear messages")
}

// GetMessageCount returns the number of messages
func (o *Operations) GetMessageCount(ctx context.Context) (int, error) {
	o.logger.Debug("GetMessageCount called", "instanceID", o.instanceID)
	count, err := o.store.GetMessageCount(ctx, o.instanceID)
	if err != nil {
		o.logger.Error("Failed to get message count from store", "instanceID", o.instanceID, "error", err)
		return 0, err
	}
	o.logger.Debug("Got message count from store", "instanceID", o.instanceID, "messageCount", count)
	return count, nil
}

// GetTokenCount returns the current token count
func (o *Operations) GetTokenCount(ctx context.Context) (int, error) {
	o.logger.Debug("GetTokenCount called", "instanceID", o.instanceID)
	tokenCount, err := o.store.GetTokenCount(ctx, o.instanceID)
	if err != nil {
		o.logger.Error("Failed to get token count from store", "instanceID", o.instanceID, "error", err)
		return 0, err
	}
	o.logger.Debug("Got token count from store", "instanceID", o.instanceID, "tokenCount", tokenCount)

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

	// Use sync.Map for better concurrent performance
	var contentCache sync.Map
	var roleCache sync.Map

	totalTokens := 0
	for _, msg := range messages {
		// Count content tokens with caching
		var contentCount int
		if count, exists := contentCache.Load(msg.Content); exists {
			if cachedCount, ok := count.(int); ok {
				contentCount = cachedCount
			} else {
				// Fallback if type assertion fails
				contentCount = o.calculateTokenCountWithFallback(ctx, msg.Content, "content")
				contentCache.Store(msg.Content, contentCount)
			}
		} else {
			contentCount = o.calculateTokenCountWithFallback(ctx, msg.Content, "content")
			contentCache.Store(msg.Content, contentCount)
		}

		// Count role tokens with caching
		roleStr := string(msg.Role)
		var roleCount int
		if count, exists := roleCache.Load(roleStr); exists {
			if cachedCount, ok := count.(int); ok {
				roleCount = cachedCount
			} else {
				// Fallback if type assertion fails
				roleCount = o.calculateTokenCountWithFallback(ctx, roleStr, "role")
				roleCache.Store(roleStr, roleCount)
			}
		} else {
			roleCount = o.calculateTokenCountWithFallback(ctx, roleStr, "role")
			roleCache.Store(roleStr, roleCount)
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

// handleLockRelease standardizes lock release error handling across operations
func (o *Operations) handleLockRelease(unlock func() error, operation string) error {
	if err := unlock(); err != nil {
		o.logger.Error("Failed to release lock",
			"error", err,
			"operation", operation,
			"instance_id", o.instanceID)
		return err
	}
	return nil
}

// combineErrors returns a combined error message when both operation and lock release fail
func (o *Operations) combineErrors(operationErr error, lockErr error, operation string) error {
	if operationErr != nil && lockErr != nil {
		return fmt.Errorf("failed to %s: %w (also failed to release lock: %v)", operation, operationErr, lockErr)
	}
	if operationErr != nil {
		return operationErr
	}
	if lockErr != nil {
		return fmt.Errorf("operation completed but failed to release lock: %w", lockErr)
	}
	return nil
}
