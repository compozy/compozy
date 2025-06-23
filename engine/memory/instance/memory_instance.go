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
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
		privacyManager:    opts.PrivacyManager,
		logger:            instanceLogger,
		metrics:           NewDefaultMetrics(instanceLogger),
	}
	return instance, nil
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
	defer func() {
		_ = lock() //nolint:errcheck // TODO: Consider logging lock release errors
	}()
	tokenCount, err := mi.tokenCounter.CountTokens(ctx, msg.Content)
	if err != nil {
		mi.logger.Error("Failed to count tokens", "error", err)
		tokenCount = 0
	}
	if err := mi.store.AppendMessageWithTokenCount(ctx, mi.id, msg, tokenCount); err != nil {
		mi.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
		return fmt.Errorf("failed to append message to store: %w", err)
	}
	mi.metrics.RecordAppend(ctx, time.Since(start), tokenCount, nil)
	mi.metrics.RecordTokenCount(ctx, tokenCount)
	mi.checkFlushTrigger(ctx)
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

func (mi *memoryInstance) AppendWithPrivacy(ctx context.Context, msg llm.Message, _ core.PrivacyMetadata) error {
	return mi.Append(ctx, msg)
}

func (mi *memoryInstance) PerformFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	start := time.Now()
	lock, err := mi.lockManager.AcquireFlushLock(ctx, mi.id)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for flush on memory %s: %w", mi.id, err)
	}
	defer func() {
		if unlockErr := lock(); unlockErr != nil {
			mi.logger.Error("Failed to release flush lock", "error", unlockErr, "memory_id", mi.id)
		}
	}()
	messages, err := mi.Read(ctx)
	if err != nil {
		mi.metrics.RecordFlush(ctx, time.Since(start), 0, err)
		return nil, fmt.Errorf("failed to read messages for flush: %w", err)
	}
	output, err := mi.flushingStrategy.PerformFlush(ctx, messages, mi.resourceConfig)
	if err != nil {
		mi.metrics.RecordFlush(ctx, time.Since(start), len(messages), err)
		return nil, fmt.Errorf("failed to perform flush: %w", err)
	}
	mi.metrics.RecordFlush(ctx, time.Since(start), len(messages), nil)
	return output, nil
}

func (mi *memoryInstance) MarkFlushPending(ctx context.Context, pending bool) error {
	return mi.store.MarkFlushPending(ctx, mi.id, pending)
}

func (mi *memoryInstance) checkFlushTrigger(ctx context.Context) {
	go func() {
		tokenCount, err := mi.GetTokenCount(ctx)
		if err != nil {
			mi.logger.Error("Failed to get token count for flush check", "error", err)
			return
		}
		messageCount, err := mi.Len(ctx)
		if err != nil {
			mi.logger.Error("Failed to get message count for flush check", "error", err)
			return
		}
		if mi.flushingStrategy.ShouldFlush(tokenCount, messageCount, mi.resourceConfig) {
			mi.logger.Info("Flush triggered", "token_count", tokenCount, "message_count", messageCount)
		}
	}()
}
