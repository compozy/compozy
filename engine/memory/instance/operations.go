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
}

// NewOperations creates a new operations handler
func NewOperations(
	_ context.Context,
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
	}
}

// AppendMessage appends a message with proper locking and metrics
func (o *Operations) AppendMessage(ctx context.Context, msg llm.Message) (err error) {
	start := time.Now()
	unlock, err := o.lockManager.AcquireAppendLock(ctx, o.instanceID)
	if err != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), 0, err)
		return err
	}
	defer func() {
		unlockErr := o.handleLockRelease(ctx, unlock, "append")
		err = o.combineErrors(err, unlockErr, "append message")
	}()
	tokenCount := o.calculateTokenCount(ctx, msg)
	if opErr := o.store.AppendMessage(ctx, o.instanceID, msg); opErr != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, opErr)
		err = opErr
		return err
	}
	if err := o.store.IncrementTokenCount(ctx, o.instanceID, tokenCount); err != nil {
		o.recordMetadataError(ctx, "increment_token_count", err)
	}
	o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, nil)
	return nil
}

// AppendMessageWithTokenCount appends a message and updates token count atomically
func (o *Operations) AppendMessageWithTokenCount(ctx context.Context, msg llm.Message, tokenCount int) (err error) {
	start := time.Now()
	unlock, err := o.lockManager.AcquireAppendLock(ctx, o.instanceID)
	if err != nil {
		o.metrics.RecordAppend(ctx, time.Since(start), 0, err)
		return err
	}
	defer func() {
		unlockErr := o.handleLockRelease(ctx, unlock, "append")
		err = o.combineErrors(err, unlockErr, "append message with token count")
	}()
	if atomicStore, ok := o.store.(core.AtomicOperations); ok {
		err = atomicStore.AppendMessageWithTokenCount(ctx, o.instanceID, msg, tokenCount)
	} else {
		if err := o.store.AppendMessage(ctx, o.instanceID, msg); err != nil {
			return err
		} else if err := o.store.IncrementTokenCount(ctx, o.instanceID, tokenCount); err != nil {
			return err
		}
	}
	o.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
	return
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
func (o *Operations) ClearMessages(ctx context.Context) (err error) {
	unlock, err := o.lockManager.AcquireClearLock(ctx, o.instanceID)
	if err != nil {
		return err
	}
	defer func() {
		unlockErr := o.handleLockRelease(ctx, unlock, "clear")
		err = o.combineErrors(err, unlockErr, "clear messages")
	}()
	operationErr := o.store.DeleteMessages(ctx, o.instanceID)
	if operationErr == nil {
		if setErr := o.store.SetTokenCount(ctx, o.instanceID, 0); setErr != nil {
			o.recordMetadataError(ctx, "reset_token_count", setErr)
		}
	}
	return operationErr
}

// GetMessageCount returns the number of messages
func (o *Operations) GetMessageCount(ctx context.Context) (int, error) {
	log := logger.FromContext(ctx)
	log.Debug("GetMessageCount called", "instanceID", o.instanceID)
	count, err := o.store.GetMessageCount(ctx, o.instanceID)
	if err != nil {
		log.Error("Failed to get message count from store", "instanceID", o.instanceID, "error", err)
		return 0, err
	}
	log.Debug("Got message count from store", "instanceID", o.instanceID, "messageCount", count)
	return count, nil
}

// GetTokenCount returns the current token count
func (o *Operations) GetTokenCount(ctx context.Context) (int, error) {
	log := logger.FromContext(ctx)
	log.Debug("GetTokenCount called", "instanceID", o.instanceID)
	tokenCount, err := o.store.GetTokenCount(ctx, o.instanceID)
	if err != nil {
		log.Error("Failed to get token count from store", "instanceID", o.instanceID, "error", err)
		return 0, err
	}
	log.Debug("Got token count from store", "instanceID", o.instanceID, "tokenCount", tokenCount)
	if tokenCount == 0 {
		messageCount, err := o.store.GetMessageCount(ctx, o.instanceID)
		if err != nil {
			return 0, err
		}

		if messageCount > 0 {
			return o.calculateTokensFromMessages(ctx)
		}
	}
	return tokenCount, nil
}

// estimateTokenCount provides a consistent fallback token estimation
func (o *Operations) estimateTokenCount(text string) int {
	tokens := len(text) / 4
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
		log := logger.FromContext(ctx)
		log.Warn("Failed to count tokens, using fallback estimation",
			"error", err, "text_type", description, "instance_id", o.instanceID)
		return o.estimateTokenCount(text)
	}
	return count
}

// calculateTokenCount calculates tokens for a single message including role and structure overhead
func (o *Operations) calculateTokenCount(ctx context.Context, msg llm.Message) int {
	contentCount := o.calculateTokenCountWithFallback(ctx, msg.Content, "content")
	roleCount := o.calculateTokenCountWithFallback(ctx, string(msg.Role), "role")
	structureOverhead := 2
	return contentCount + roleCount + structureOverhead
}

// calculateTokensFromMessages calculates total tokens from all messages with caching optimization
func (o *Operations) calculateTokensFromMessages(ctx context.Context) (int, error) {
	messages, err := o.store.ReadMessages(ctx, o.instanceID)
	if err != nil {
		return 0, err
	}
	caches := tokenCountCaches{}
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += o.calculateMessageTokens(ctx, msg, &caches)
	}
	if err := o.store.SetTokenCount(ctx, o.instanceID, totalTokens); err != nil {
		o.recordMetadataError(ctx, "set_token_count_migration", err)
	}
	return totalTokens, nil
}

// calculateMessageTokens calculates the total tokens for a message using cached counts
func (o *Operations) calculateMessageTokens(ctx context.Context, msg llm.Message, caches *tokenCountCaches) int {
	contentCount := o.loadTokenCount(ctx, &caches.content, msg.Content, "content")
	roleCount := o.loadTokenCount(ctx, &caches.roles, string(msg.Role), "role")
	const structureOverhead = 2
	return contentCount + roleCount + structureOverhead
}

// loadTokenCount reads a cached token count or calculates and stores it when missing
func (o *Operations) loadTokenCount(ctx context.Context, cache *sync.Map, text, description string) int {
	if count, ok := cache.Load(text); ok {
		if cachedCount, isInt := count.(int); isInt {
			return cachedCount
		}
	}
	calculated := o.calculateTokenCountWithFallback(ctx, text, description)
	cache.Store(text, calculated)
	return calculated
}

type tokenCountCaches struct {
	content sync.Map
	roles   sync.Map
}

// recordMetadataError logs metadata operation errors without failing the main operation
func (o *Operations) recordMetadataError(ctx context.Context, operation string, err error) {
	log := logger.FromContext(ctx)
	log.Error("Metadata operation failed",
		"operation", operation,
		"error", err,
		"instance_id", o.instanceID)
}

// handleLockRelease standardizes lock release error handling across operations
func (o *Operations) handleLockRelease(ctx context.Context, unlock func() error, operation string) error {
	if err := unlock(); err != nil {
		log := logger.FromContext(ctx)
		log.Error("Failed to release lock",
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
