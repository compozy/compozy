package instance

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/client"
)

type memoryInstance struct {
	id                string
	resourceID        string
	projectID         string
	resourceConfig    *core.Resource
	store             core.Store
	lockManager       LockManager
	tokenCounter      core.TokenCounter
	flushingStrategy  FlushStrategy
	evictionPolicy    EvictionPolicy
	temporalClient    client.Client
	temporalTaskQueue string
	privacyManager    any
	logger            logger.Logger
	metrics           Metrics
}

func NewMemoryInstance(opts *BuilderOptions) (Instance, error) {
	instanceLogger := opts.Logger.With(
		"memory_instance_id", opts.InstanceID,
		"memory_resource_id", opts.ResourceID,
	)
	instance := &memoryInstance{
		id:                opts.InstanceID,
		resourceID:        opts.ResourceID,
		projectID:         opts.ProjectID,
		resourceConfig:    opts.ResourceConfig,
		store:             opts.Store,
		lockManager:       opts.LockManager,
		tokenCounter:      opts.TokenCounter,
		flushingStrategy:  opts.FlushingStrategy,
		evictionPolicy:    opts.EvictionPolicy,
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
		privacyManager:    opts.PrivacyManager,
		logger:            instanceLogger,
		metrics:           NewDefaultMetrics(instanceLogger),
	}
	return instance, nil
}

// estimateTokenCount provides a consistent fallback token estimation
func (mi *memoryInstance) estimateTokenCount(text string) int {
	// Rough estimate: 4 characters per token (common for most tokenizers)
	tokens := len(text) / 4
	// Ensure at least 1 token for non-empty text
	if tokens == 0 && text != "" {
		tokens = 1
	}
	return tokens
}

// calculateTokenCountWithFallback safely counts tokens with consistent fallback logic
func (mi *memoryInstance) calculateTokenCountWithFallback(ctx context.Context, text string, description string) int {
	if mi.tokenCounter == nil {
		return mi.estimateTokenCount(text)
	}
	count, err := mi.tokenCounter.CountTokens(ctx, text)
	if err != nil {
		mi.logger.Warn("Failed to count tokens, using fallback estimation",
			"error", err, "text_type", description)
		return mi.estimateTokenCount(text)
	}
	return count
}

// calculateMessageTokenCount calculates tokens for a single message including role and structure overhead
func (mi *memoryInstance) calculateMessageTokenCount(ctx context.Context, msg llm.Message) int {
	// Count content tokens with consistent fallback
	contentCount := mi.calculateTokenCountWithFallback(ctx, msg.Content, "content")

	// Count role tokens with consistent fallback
	roleCount := mi.calculateTokenCountWithFallback(ctx, string(msg.Role), "role")

	// Add structure overhead for message formatting
	structureOverhead := 2
	return contentCount + roleCount + structureOverhead
}

func (mi *memoryInstance) GetID() string {
	return mi.id
}

func (mi *memoryInstance) GetResource() *core.Resource {
	return mi.resourceConfig
}

func (mi *memoryInstance) GetStore() core.Store {
	return mi.store
}

func (mi *memoryInstance) GetTokenCounter() core.TokenCounter {
	return mi.tokenCounter
}

func (mi *memoryInstance) GetMetrics() Metrics {
	return mi.metrics
}

func (mi *memoryInstance) GetLockManager() LockManager {
	return mi.lockManager
}

func (mi *memoryInstance) GetEvictionPolicy() EvictionPolicy {
	return mi.evictionPolicy
}

func (mi *memoryInstance) Append(ctx context.Context, msg llm.Message) error {
	start := time.Now()
	mi.logger.Debug("Append called",
		"message_role", msg.Role,
		"memory_id", mi.id,
		"operation", "append")
	lock, err := mi.lockManager.AcquireAppendLock(ctx, mi.id)
	if err != nil {
		mi.metrics.RecordAppend(ctx, time.Since(start), 0, err)
		return fmt.Errorf("failed to acquire lock for append on memory %s: %w", mi.id, err)
	}
	var lockReleaseErr error
	defer func() {
		if releaseErr := lock(); releaseErr != nil {
			lockReleaseErr = releaseErr
			mi.logger.Error("Failed to release lock",
				"error", releaseErr,
				"operation", "append",
				"memory_id", mi.id,
				"context", "memory_append_operation")
		}
	}()
	tokenCount := mi.calculateMessageTokenCount(ctx, msg)
	if err := mi.store.AppendMessageWithTokenCount(ctx, mi.id, msg, tokenCount); err != nil {
		mi.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
		if lockReleaseErr != nil {
			return fmt.Errorf(
				"failed to append message to store: %w (also failed to release lock: %v)",
				err,
				lockReleaseErr,
			)
		}
		return fmt.Errorf("failed to append message to store: %w", err)
	}
	mi.metrics.RecordAppend(ctx, time.Since(start), tokenCount, nil)
	mi.metrics.RecordTokenCount(ctx, tokenCount)
	// Set TTL if configured and this is the first message
	if mi.resourceConfig != nil && mi.resourceConfig.Persistence.ParsedTTL > 0 {
		// Check if we need to set/extend TTL
		currentTTL, err := mi.store.GetKeyTTL(ctx, mi.id)
		if err != nil {
			mi.logger.Warn("Failed to get current TTL", "error", err, "memory_id", mi.id)
			// Continue with setting TTL anyway
			currentTTL = 0
		}
		// Set TTL if not set or extend if needed
		if currentTTL <= 0 || currentTTL < mi.resourceConfig.Persistence.ParsedTTL/2 {
			if err := mi.store.SetExpiration(ctx, mi.id, mi.resourceConfig.Persistence.ParsedTTL); err != nil {
				mi.logger.Error("Failed to set TTL on memory", "error", err, "memory_id", mi.id)
			} else {
				mi.logger.Debug("Set TTL on memory", "memory_id", mi.id, "ttl", mi.resourceConfig.Persistence.ParsedTTL)
			}
		}
	}
	mi.checkFlushTrigger(ctx)
	if lockReleaseErr != nil {
		return fmt.Errorf("operation completed but failed to release lock: %w", lockReleaseErr)
	}
	return nil
}

func (mi *memoryInstance) Read(ctx context.Context) ([]llm.Message, error) {
	start := time.Now()
	messages, err := mi.store.ReadMessages(ctx, mi.id)
	mi.metrics.RecordRead(ctx, time.Since(start), len(messages), err)
	return messages, err
}

func (mi *memoryInstance) Len(ctx context.Context) (int, error) {
	count, err := mi.store.GetMessageCount(ctx, mi.id)
	if err != nil {
		return 0, err
	}
	mi.metrics.RecordMessageCount(ctx, count)
	return count, nil
}

func (mi *memoryInstance) GetTokenCount(ctx context.Context) (int, error) {
	count, err := mi.store.GetTokenCount(ctx, mi.id)
	if err != nil {
		return 0, err
	}
	mi.metrics.RecordTokenCount(ctx, count)
	return count, nil
}

func (mi *memoryInstance) GetMemoryHealth(ctx context.Context) (*core.Health, error) {
	messageCount, err := mi.Len(ctx)
	if err != nil {
		return nil, err
	}
	tokenCount, err := mi.GetTokenCount(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &core.Health{
		MessageCount:  messageCount,
		TokenCount:    tokenCount,
		LastFlush:     &now,
		FlushStrategy: mi.flushingStrategy.GetType().String(),
	}, nil
}

func (mi *memoryInstance) Clear(ctx context.Context) error {
	lock, err := mi.lockManager.AcquireClearLock(ctx, mi.id)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for clear on memory %s: %w", mi.id, err)
	}
	defer func() {
		if unlockErr := lock(); unlockErr != nil {
			mi.logger.Error("Failed to release clear lock", "error", unlockErr, "memory_id", mi.id)
		}
	}()
	if err := mi.store.DeleteMessages(ctx, mi.id); err != nil {
		return fmt.Errorf("failed to clear memory: %w", err)
	}
	return nil
}

func (mi *memoryInstance) AppendWithPrivacy(ctx context.Context, msg llm.Message, metadata core.PrivacyMetadata) error {
	// Check explicit DoNotPersist flag
	if metadata.DoNotPersist {
		mi.logger.Debug("Message marked as DoNotPersist, skipping storage",
			"message_role", msg.Role,
			"memory_id", mi.id)
		return nil
	}
	// Apply privacy controls if privacy manager is available
	if mi.privacyManager != nil {
		// For now, we'll handle the privacy manager interface properly when we implement full privacy support
		// The basic DoNotPersist check above handles the test requirement
		mi.logger.Debug("Privacy manager available but not fully integrated yet",
			"memory_id", mi.id)
	}
	// Proceed with regular append
	return mi.Append(ctx, msg)
}

func (mi *memoryInstance) PerformFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	// Create flush handler with necessary dependencies
	flushHandler := &FlushHandler{
		instanceID:       mi.id,
		store:            mi.store,
		lockManager:      mi.lockManager,
		flushingStrategy: mi.flushingStrategy,
		logger:           mi.logger,
		metrics:          mi.metrics,
		resourceConfig:   mi.resourceConfig,
	}
	return flushHandler.PerformFlush(ctx)
}

func (mi *memoryInstance) MarkFlushPending(ctx context.Context, pending bool) error {
	return mi.store.MarkFlushPending(ctx, mi.id, pending)
}

func (mi *memoryInstance) checkFlushTrigger(ctx context.Context) {
	go func() {
		// Create a timeout context to prevent goroutine leaks
		timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Check if the context is already canceled before starting work
		select {
		case <-timeoutCtx.Done():
			mi.logger.Debug("Flush trigger check canceled", "memory_id", mi.id, "reason", timeoutCtx.Err())
			return
		default:
		}

		tokenCount, err := mi.GetTokenCount(timeoutCtx)
		if err != nil {
			// Check if error is due to context cancellation
			if timeoutCtx.Err() != nil {
				mi.logger.Debug("Token count check canceled", "memory_id", mi.id, "reason", timeoutCtx.Err())
				return
			}
			mi.logger.Error("Failed to get token count for flush check", "error", err, "memory_id", mi.id)
			return
		}

		// Check cancellation again before the second call
		select {
		case <-timeoutCtx.Done():
			mi.logger.Debug(
				"Flush trigger check canceled after token count",
				"memory_id",
				mi.id,
				"reason",
				timeoutCtx.Err(),
			)
			return
		default:
		}

		messageCount, err := mi.Len(timeoutCtx)
		if err != nil {
			// Check if error is due to context cancellation
			if timeoutCtx.Err() != nil {
				mi.logger.Debug("Message count check canceled", "memory_id", mi.id, "reason", timeoutCtx.Err())
				return
			}
			mi.logger.Error("Failed to get message count for flush check", "error", err, "memory_id", mi.id)
			return
		}

		// Final cancellation check before flush decision
		select {
		case <-timeoutCtx.Done():
			mi.logger.Debug(
				"Flush trigger check canceled before flush decision",
				"memory_id",
				mi.id,
				"reason",
				timeoutCtx.Err(),
			)
			return
		default:
		}

		if mi.flushingStrategy.ShouldFlush(tokenCount, messageCount, mi.resourceConfig) {
			mi.logger.Info(
				"Flush triggered",
				"token_count",
				tokenCount,
				"message_count",
				messageCount,
				"memory_id",
				mi.id,
			)
		}
	}()
}
