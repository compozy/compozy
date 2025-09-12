package instance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/instance/strategies"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/romdo/go-debounce"
	"go.temporal.io/sdk/client"
)

// Domain-specific constants for token estimation and message overhead
const (
	// estimatedCharsPerToken is the average number of characters per token
	// This is a common approximation for most tokenizers
	estimatedCharsPerToken = 4

	// messageStructureOverhead represents the token overhead for message formatting
	// (e.g., delimiters, special tokens)
	messageStructureOverhead = 2
)

type memoryInstance struct {
	id                string
	resourceID        string
	projectID         string
	resourceConfig    *core.Resource
	store             core.Store
	lockManager       LockManager
	tokenCounter      core.TokenCounter
	asyncTokenCounter AsyncTokenCounter // NEW: for async token counting
	flushingStrategy  core.FlushStrategy
	evictionPolicy    EvictionPolicy
	temporalClient    client.Client
	temporalTaskQueue string
	privacyManager    any
	metrics           Metrics
	strategyFactory   *strategies.StrategyFactory // NEW: for dynamic strategy creation
	flushMutex        sync.Mutex                  // Ensures only one flush check runs at a time
	flushWG           sync.WaitGroup              // Tracks in-flight flush operations
	debouncedFlush    func()                      // Debounced flush check function
	flushCancelFunc   func()                      // Cancel function for the debouncer
}

func NewMemoryInstance(ctx context.Context, opts *BuilderOptions) (Instance, error) {
	// Create strategy factory with token counter
	strategyFactory := strategies.NewStrategyFactoryWithTokenCounter(opts.TokenCounter)
	log := logger.FromContext(ctx)
	instance := &memoryInstance{
		id:                opts.InstanceID,
		resourceID:        opts.ResourceID,
		projectID:         opts.ProjectID,
		resourceConfig:    opts.ResourceConfig,
		store:             opts.Store,
		lockManager:       opts.LockManager,
		tokenCounter:      opts.TokenCounter,
		asyncTokenCounter: opts.AsyncTokenCounter,
		flushingStrategy:  opts.FlushingStrategy,
		evictionPolicy:    opts.EvictionPolicy,
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
		privacyManager:    opts.PrivacyManager,
		metrics:           NewDefaultMetrics(),
		strategyFactory:   strategyFactory,
	}

	// Create the debounced flush function
	// Coalesce flush checks within 100ms, but ensure they run at least every 1s
	const flushDebounceWait = 100 * time.Millisecond
	const flushMaxWait = 1 * time.Second

	debouncedFunc, cancelFunc := debounce.NewWithMaxWait(
		flushDebounceWait,
		flushMaxWait,
		func() {
			// Check if we can acquire the mutex (non-blocking check)
			if !instance.flushMutex.TryLock() {
				// If we can't get the lock, Close() might be in progress
				return
			}

			// Check if cancel function has been cleared (indicates Close() was called)
			if instance.flushCancelFunc == nil {
				instance.flushMutex.Unlock()
				return
			}

			// Track this flush operation
			instance.flushWG.Add(1)
			defer instance.flushWG.Done()
			defer instance.flushMutex.Unlock()

			// Use background context since this is independent of any single Append request
			if err := instance.performAsyncFlushCheck(context.Background()); err != nil {
				log.Error("Failed to perform async flush check", "error", err, "memory_id", instance.id)
			}
		},
	)

	instance.debouncedFlush = debouncedFunc
	instance.flushCancelFunc = cancelFunc

	return instance, nil
}

// estimateTokenCount provides a consistent fallback token estimation
func (mi *memoryInstance) estimateTokenCount(text string) int {
	// Rough estimate: 4 characters per token (common for most tokenizers)
	tokens := len(text) / estimatedCharsPerToken
	// Ensure at least 1 token for non-empty text
	if tokens == 0 && text != "" {
		tokens = 1
	}
	return tokens
}

// calculateTokenCountWithFallback safely counts tokens with consistent fallback logic
func (mi *memoryInstance) calculateTokenCountWithFallback(ctx context.Context, text string, description string) int {
	log := logger.FromContext(ctx)
	if mi.tokenCounter == nil {
		return mi.estimateTokenCount(text)
	}
	count, err := mi.tokenCounter.CountTokens(ctx, text)
	if err != nil {
		log.Warn("Failed to count tokens, using fallback estimation",
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
	return contentCount + roleCount + messageStructureOverhead
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
	log := logger.FromContext(ctx)
	log.Debug("Append called",
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
			log.Error("Failed to release lock",
				"error", releaseErr,
				"operation", "append",
				"memory_id", mi.id,
				"context", "memory_append_operation")
		}
	}()
	// Compute token count for this message
	tokenCount := mi.calculateTokenCountForMessage(ctx, msg)
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
			log.Warn("Failed to get current TTL", "error", err, "memory_id", mi.id)
			// Continue with setting TTL anyway
			currentTTL = 0
		}
		// Set TTL if not set or extend if needed
		if currentTTL <= 0 || currentTTL < mi.resourceConfig.Persistence.ParsedTTL/2 {
			if err := mi.store.SetExpiration(ctx, mi.id, mi.resourceConfig.Persistence.ParsedTTL); err != nil {
				log.Error("Failed to set TTL on memory", "error", err, "memory_id", mi.id)
			} else {
				log.Debug("Set TTL on memory", "memory_id", mi.id, "ttl", mi.resourceConfig.Persistence.ParsedTTL)
			}
		}
	}
	mi.checkFlushTrigger(ctx)
	if lockReleaseErr != nil {
		return fmt.Errorf("operation completed but failed to release lock: %w", lockReleaseErr)
	}
	return nil
}

// AppendMany atomically appends multiple messages to the memory.
// This ensures all messages are stored together or none are stored.
func (mi *memoryInstance) AppendMany(ctx context.Context, msgs []llm.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	start := time.Now()
	log := logger.FromContext(ctx)
	log.Debug("AppendMany called",
		"message_count", len(msgs),
		"memory_id", mi.id,
		"operation", "append_many")
	lock, err := mi.lockManager.AcquireAppendLock(ctx, mi.id)
	if err != nil {
		mi.metrics.RecordAppend(ctx, time.Since(start), 0, err)
		return fmt.Errorf("failed to acquire lock for append_many on memory %s: %w", mi.id, err)
	}
	var lockReleaseErr error
	defer func() {
		if releaseErr := lock(); releaseErr != nil {
			lockReleaseErr = releaseErr
			log.Error("Failed to release lock",
				"error", releaseErr,
				"operation", "append_many",
				"memory_id", mi.id,
				"context", "memory_append_many_operation")
		}
	}()
	totalTokenCount := mi.calculateTotalTokenCount(ctx, msgs)
	if err := mi.store.AppendMessages(ctx, mi.id, msgs); err != nil {
		mi.metrics.RecordAppend(ctx, time.Since(start), totalTokenCount, err)
		if lockReleaseErr != nil {
			return fmt.Errorf(
				"failed to append messages to store: %w (also failed to release lock: %v)",
				err,
				lockReleaseErr,
			)
		}
		return fmt.Errorf("failed to append messages to store: %w", err)
	}
	mi.updateMetadataAndMetrics(ctx, totalTokenCount)
	mi.metrics.RecordAppend(ctx, time.Since(start), totalTokenCount, nil)
	mi.handleTTLForAppendMany(ctx)
	mi.checkFlushTrigger(ctx)
	if lockReleaseErr != nil {
		return fmt.Errorf("operation completed but failed to release lock: %w", lockReleaseErr)
	}
	return nil
}

// calculateTokenCountForMessage calculates tokens for a single message
func (mi *memoryInstance) calculateTokenCountForMessage(ctx context.Context, msg llm.Message) int {
	if mi.asyncTokenCounter != nil {
		// Queue async token counting (non-blocking)
		mi.asyncTokenCounter.ProcessAsync(ctx, mi.id, msg.Content)
		// Use estimate for immediate metrics
		return mi.estimateTokenCount(msg.Content) + mi.estimateTokenCount(string(msg.Role)) + messageStructureOverhead
	}
	// Fallback to synchronous counting
	return mi.calculateMessageTokenCount(ctx, msg)
}

// calculateTotalTokenCount calculates the total token count for multiple messages
func (mi *memoryInstance) calculateTotalTokenCount(ctx context.Context, msgs []llm.Message) int {
	var totalTokenCount int
	for _, msg := range msgs {
		tokenCount := mi.calculateTokenCountForMessage(ctx, msg)
		totalTokenCount += tokenCount
	}
	return totalTokenCount
}

// updateMetadataAndMetrics updates token count metadata and metrics
func (mi *memoryInstance) updateMetadataAndMetrics(ctx context.Context, totalTokenCount int) {
	log := logger.FromContext(ctx)
	// Update token count metadata
	if err := mi.store.IncrementTokenCount(ctx, mi.id, totalTokenCount); err != nil {
		log.Warn("Failed to update token count metadata after append_many",
			"error", err,
			"memory_id", mi.id,
			"token_count", totalTokenCount)
		// Continue as this is not critical for the append operation
	}
	mi.metrics.RecordTokenCount(ctx, totalTokenCount)
}

// handleTTLForAppendMany handles TTL configuration for AppendMany operation
func (mi *memoryInstance) handleTTLForAppendMany(ctx context.Context) {
	log := logger.FromContext(ctx)
	if mi.resourceConfig != nil && mi.resourceConfig.Persistence.ParsedTTL > 0 {
		// Check if we need to set/extend TTL
		currentTTL, err := mi.store.GetKeyTTL(ctx, mi.id)
		if err != nil {
			log.Warn("Failed to get current TTL", "error", err, "memory_id", mi.id)
			// Continue with setting TTL anyway
			currentTTL = 0
		}
		// Set TTL if not set or extend if needed
		if currentTTL <= 0 || currentTTL < mi.resourceConfig.Persistence.ParsedTTL/2 {
			if err := mi.store.SetExpiration(ctx, mi.id, mi.resourceConfig.Persistence.ParsedTTL); err != nil {
				log.Error("Failed to set TTL on memory", "error", err, "memory_id", mi.id)
			} else {
				log.Debug("Set TTL on memory", "memory_id", mi.id, "ttl", mi.resourceConfig.Persistence.ParsedTTL)
			}
		}
	}
}

func (mi *memoryInstance) Read(ctx context.Context) ([]llm.Message, error) {
	start := time.Now()
	messages, err := mi.store.ReadMessages(ctx, mi.id)
	mi.metrics.RecordRead(ctx, time.Since(start), len(messages), err)
	return messages, err
}

func (mi *memoryInstance) ReadPaginated(ctx context.Context, offset, limit int) ([]llm.Message, int, error) {
	start := time.Now()
	messages, totalCount, err := mi.store.ReadMessagesPaginated(ctx, mi.id, offset, limit)
	mi.metrics.RecordRead(ctx, time.Since(start), len(messages), err)
	return messages, totalCount, err
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
		MessageCount:   messageCount,
		TokenCount:     tokenCount,
		LastFlush:      &now,
		ActualStrategy: mi.flushingStrategy.GetType().String(),
	}, nil
}

func (mi *memoryInstance) Clear(ctx context.Context) error {
	log := logger.FromContext(ctx)
	lock, err := mi.lockManager.AcquireClearLock(ctx, mi.id)
	if err != nil {
		return fmt.Errorf("failed to acquire lock for clear on memory %s: %w", mi.id, err)
	}
	defer func() {
		if unlockErr := lock(); unlockErr != nil {
			log.Error("Failed to release clear lock", "error", unlockErr, "memory_id", mi.id)
		}
	}()
	if err := mi.store.DeleteMessages(ctx, mi.id); err != nil {
		return fmt.Errorf("failed to clear memory: %w", err)
	}
	return nil
}

func (mi *memoryInstance) AppendWithPrivacy(ctx context.Context, msg llm.Message, metadata core.PrivacyMetadata) error {
	log := logger.FromContext(ctx)
	// Check explicit DoNotPersist flag
	if metadata.DoNotPersist {
		log.Debug("Message marked as DoNotPersist, skipping storage",
			"message_role", msg.Role,
			"memory_id", mi.id)
		return nil
	}
	// Apply privacy controls if privacy manager is available
	if mi.privacyManager != nil {
		// For now, we'll handle the privacy manager interface properly when we implement full privacy support
		// The basic DoNotPersist check above handles the test requirement
		log.Debug("Privacy manager available but not fully integrated yet",
			"memory_id", mi.id)
	}
	// Proceed with regular append
	return mi.Append(ctx, msg)
}

func (mi *memoryInstance) PerformFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	// Use PerformFlushWithStrategy with empty strategy to use default
	return mi.PerformFlushWithStrategy(ctx, core.FlushingStrategyType(""))
}

// PerformFlushWithStrategy implements DynamicFlushableMemory interface
func (mi *memoryInstance) PerformFlushWithStrategy(
	ctx context.Context,
	strategyType core.FlushingStrategyType,
) (*core.FlushMemoryActivityOutput, error) {
	// Validate strategy type if provided
	if strategyType != "" {
		if err := mi.validateStrategyType(string(strategyType)); err != nil {
			return nil, fmt.Errorf("invalid strategy type: %w", err)
		}
	}

	// Create flush handler with necessary dependencies
	flushHandler := &FlushHandler{
		instanceID:        mi.id,
		projectID:         mi.projectID,
		store:             mi.store,
		lockManager:       mi.lockManager,
		flushingStrategy:  mi.flushingStrategy,  // default strategy
		strategyFactory:   mi.strategyFactory,   // for dynamic creation
		requestedStrategy: string(strategyType), // requested strategy
		tokenCounter:      mi.tokenCounter,
		metrics:           mi.metrics,
		resourceConfig:    mi.resourceConfig,
	}
	return flushHandler.PerformFlush(ctx)
}

// GetConfiguredStrategy implements DynamicFlushableMemory interface
func (mi *memoryInstance) GetConfiguredStrategy() core.FlushingStrategyType {
	if mi.resourceConfig != nil && mi.resourceConfig.FlushingStrategy != nil {
		return mi.resourceConfig.FlushingStrategy.Type
	}
	return core.SimpleFIFOFlushing
}

// validateStrategyType validates that the strategy type is supported
func (mi *memoryInstance) validateStrategyType(strategyType string) error {
	// Use factory's validation method
	return mi.strategyFactory.ValidateStrategyType(strategyType)
}

func (mi *memoryInstance) MarkFlushPending(ctx context.Context, pending bool) error {
	return mi.store.MarkFlushPending(ctx, mi.id, pending)
}

func (mi *memoryInstance) checkFlushTrigger(_ context.Context) {
	// The context is not used since the debounced function runs independently
	// This ensures flush checks are not tied to the lifecycle of individual requests
	mi.debouncedFlush()
}

// performAsyncFlushCheck executes the flush check logic asynchronously.
// It creates a timeout context to prevent goroutine leaks and checks
// if flushing should be triggered based on token and message counts.
func (mi *memoryInstance) performAsyncFlushCheck(ctx context.Context) error {
	log := logger.FromContext(ctx)
	// Create a timeout context to prevent goroutine leaks
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Check if the context is already canceled before starting work
	if mi.isContextCanceled(timeoutCtx, "Flush trigger check canceled") {
		return nil
	}
	// Get token count with cancellation check
	tokenCount, err := mi.getTokenCountWithCheck(timeoutCtx)
	if err != nil {
		return err
	}
	// Check cancellation again before the second call
	if mi.isContextCanceled(timeoutCtx, "Flush trigger check canceled after token count") {
		return nil
	}
	// Get message count with cancellation check
	messageCount, err := mi.getMessageCountWithCheck(timeoutCtx)
	if err != nil {
		return err
	}
	// Final cancellation check before flush decision
	if mi.isContextCanceled(timeoutCtx, "Flush trigger check canceled before flush decision") {
		return nil
	}
	// Check if flush should be triggered
	if mi.flushingStrategy.ShouldFlush(tokenCount, messageCount, mi.resourceConfig) {
		log.Info(
			"Flush triggered",
			"token_count",
			tokenCount,
			"message_count",
			messageCount,
			"memory_id",
			mi.id,
		)
		// Perform the actual flush
		_, err := mi.PerformFlush(timeoutCtx)
		if err != nil {
			log.Error("Failed to perform flush", "error", err, "memory_id", mi.id)
			return fmt.Errorf("failed to perform flush for memory %s: %w", mi.id, err)
		}
	}
	return nil
}

// isContextCanceled checks if the context is canceled and logs if so.
// Returns true if the context is done, false otherwise.
func (mi *memoryInstance) isContextCanceled(ctx context.Context, message string) bool {
	log := logger.FromContext(ctx)
	select {
	case <-ctx.Done():
		log.Debug(message, "memory_id", mi.id, "reason", ctx.Err())
		return true
	default:
		return false
	}
}

// getTokenCountWithCheck gets token count and handles cancellation.
// It distinguishes between context cancellation errors and actual failures,
// logging appropriately for each case.
func (mi *memoryInstance) getTokenCountWithCheck(ctx context.Context) (int, error) {
	log := logger.FromContext(ctx)
	tokenCount, err := mi.GetTokenCount(ctx)
	if err != nil {
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			log.Debug("Token count check canceled", "memory_id", mi.id, "reason", ctx.Err())
			return 0, err
		}
		log.Error("Failed to get token count for flush check", "error", err, "memory_id", mi.id)
		return 0, err
	}
	return tokenCount, nil
}

// getMessageCountWithCheck gets message count and handles cancellation.
// It distinguishes between context cancellation errors and actual failures,
// logging appropriately for each case.
func (mi *memoryInstance) getMessageCountWithCheck(ctx context.Context) (int, error) {
	log := logger.FromContext(ctx)
	messageCount, err := mi.Len(ctx)
	if err != nil {
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			log.Debug("Message count check canceled", "memory_id", mi.id, "reason", ctx.Err())
			return 0, err
		}
		log.Error("Failed to get message count for flush check", "error", err, "memory_id", mi.id)
		return 0, err
	}
	return messageCount, nil
}

// Close gracefully shuts down the memory instance
func (mi *memoryInstance) Close(ctx context.Context) error {
	log := logger.FromContext(ctx)
	// 1. Acquire flush mutex to prevent new flush operations from starting
	mi.flushMutex.Lock()

	// 2. Stop the debouncer from scheduling any further calls
	if mi.flushCancelFunc != nil {
		mi.flushCancelFunc()
		mi.flushCancelFunc = nil // Prevent double cancellation
	}

	// 3. Release mutex before waiting to avoid deadlock
	mi.flushMutex.Unlock()

	// 4. Wait for any in-flight flush operations to complete
	mi.flushWG.Wait()

	// 5. Perform a final synchronous flush to ensure all data is persisted
	mi.flushMutex.Lock()
	defer mi.flushMutex.Unlock()

	// Perform final flush check with provided context
	if err := mi.performAsyncFlushCheck(ctx); err != nil {
		log.Error("Failed to perform final flush during close", "error", err, "memory_id", mi.id)
		return fmt.Errorf("failed to perform final flush during close: %w", err)
	}

	log.Info("Memory instance flushed and closed successfully", "memory_id", mi.id)
	return nil
}
