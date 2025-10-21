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

	flushDebounceWait    = 100 * time.Millisecond
	flushDebounceMaxWait = time.Second
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
	instance := newBaseMemoryInstance(opts)
	instance.setupFlushScheduler(ctx, logger.FromContext(ctx))
	return instance, nil
}

func newBaseMemoryInstance(opts *BuilderOptions) *memoryInstance {
	strategyFactory := strategies.NewStrategyFactoryWithTokenCounter(opts.TokenCounter)
	return &memoryInstance{
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
}

func (mi *memoryInstance) setupFlushScheduler(ctx context.Context, log logger.Logger) {
	debouncedFunc, cancelFunc := debounce.NewWithMaxWait(
		flushDebounceWait,
		flushDebounceMaxWait,
		func() {
			if !mi.flushMutex.TryLock() {
				return
			}
			if mi.flushCancelFunc == nil {
				mi.flushMutex.Unlock()
				return
			}
			mi.flushWG.Add(1)
			defer mi.flushWG.Done()
			defer mi.flushMutex.Unlock()
			if err := mi.performAsyncFlushCheck(context.WithoutCancel(ctx)); err != nil && log != nil {
				log.Error("Failed to perform async flush check", "error", err, "memory_id", mi.id)
			}
		},
	)
	mi.debouncedFlush = debouncedFunc
	mi.flushCancelFunc = cancelFunc
}

// estimateTokenCount provides a consistent fallback token estimation
func (mi *memoryInstance) estimateTokenCount(text string) int {
	tokens := len(text) / estimatedCharsPerToken
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
	contentCount := mi.calculateTokenCountWithFallback(ctx, msg.Content, "content")
	roleCount := mi.calculateTokenCountWithFallback(ctx, string(msg.Role), "role")
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

func (mi *memoryInstance) Append(ctx context.Context, msg llm.Message) (err error) {
	start := time.Now()
	logger.FromContext(ctx).Debug("Append called",
		"message_role", msg.Role,
		"memory_id", mi.id,
		"operation", "append")
	release, releaseErrPtr, acquireErr := mi.acquireAppendLock(ctx, "append")
	if acquireErr != nil {
		mi.recordAppendMetrics(ctx, start, 0, acquireErr)
		return fmt.Errorf("failed to acquire lock for append on memory %s: %w", mi.id, acquireErr)
	}
	defer func() {
		release()
		mi.applyReleaseError(&err, releaseErrPtr)
	}()
	tokenCount := mi.calculateTokenCountForMessage(ctx, msg)
	if appendErr := mi.store.AppendMessageWithTokenCount(ctx, mi.id, msg, tokenCount); appendErr != nil {
		mi.recordAppendMetrics(ctx, start, tokenCount, appendErr)
		return fmt.Errorf("failed to append message to store: %w", appendErr)
	}
	mi.recordAppendMetrics(ctx, start, tokenCount, nil)
	mi.metrics.RecordTokenCount(ctx, tokenCount)
	mi.enqueueAsyncTokenReconciliation(ctx, msg)
	mi.updateExpirationIfNeeded(ctx)
	mi.checkFlushTrigger(ctx)
	return err
}

// reconcileAsyncTokenCount asynchronously reconciles the actual token count with the persisted estimate
func (mi *memoryInstance) reconcileAsyncTokenCount(ctx context.Context, msg llm.Message) {
	log := logger.FromContext(ctx)
	actualContent, err := mi.asyncTokenCounter.ProcessWithResult(ctx, mi.id, msg.Content)
	if err != nil {
		log.Debug("Async token reconciliation skipped", "error", err, "memory_id", mi.id)
		return
	}
	actualRole := mi.calculateTokenCountWithFallback(ctx, string(msg.Role), "role")
	actualTotal := actualContent + actualRole + messageStructureOverhead
	estTotal := mi.estimateTokenCount(msg.Content) + mi.estimateTokenCount(string(msg.Role)) + messageStructureOverhead
	delta := actualTotal - estTotal
	if delta == 0 {
		return
	}
	if err := mi.store.IncrementTokenCount(ctx, mi.id, delta); err != nil {
		log.Warn("Failed to reconcile token count delta", "delta", delta, "error", err, "memory_id", mi.id)
	} else {
		log.Debug("Reconciled token count delta", "delta", delta, "memory_id", mi.id)
	}
}

// AppendMany atomically appends multiple messages to the memory.
// This ensures all messages are stored together or none are stored.
func (mi *memoryInstance) AppendMany(ctx context.Context, msgs []llm.Message) (err error) {
	if len(msgs) == 0 {
		return nil
	}
	start := time.Now()
	log := logger.FromContext(ctx)
	log.Debug("AppendMany called",
		"message_count", len(msgs),
		"memory_id", mi.id,
		"operation", "append_many")
	release, releaseErrPtr, acquireErr := mi.acquireAppendLock(ctx, "append_many")
	if acquireErr != nil {
		mi.recordAppendMetrics(ctx, start, 0, acquireErr)
		return fmt.Errorf("failed to acquire lock for append_many on memory %s: %w", mi.id, acquireErr)
	}
	defer func() {
		release()
		mi.applyReleaseError(&err, releaseErrPtr)
	}()
	totalTokenCount := mi.calculateTotalTokenCount(ctx, msgs)
	if appendErr := mi.store.AppendMessages(ctx, mi.id, msgs); appendErr != nil {
		mi.recordAppendMetrics(ctx, start, totalTokenCount, appendErr)
		return fmt.Errorf("failed to append messages to store: %w", appendErr)
	}
	mi.updateMetadataAndMetrics(ctx, totalTokenCount)
	mi.recordAppendMetrics(ctx, start, totalTokenCount, nil)
	mi.enqueueAsyncTokenReconciliationForSlice(ctx, msgs)
	mi.updateExpirationIfNeeded(ctx)
	mi.checkFlushTrigger(ctx)
	return err
}

// calculateTokenCountForMessage calculates tokens for a single message
func (mi *memoryInstance) calculateTokenCountForMessage(ctx context.Context, msg llm.Message) int {
	if mi.asyncTokenCounter != nil {
		mi.asyncTokenCounter.ProcessAsync(ctx, mi.id, msg.Content)
		return mi.estimateTokenCount(msg.Content) + mi.estimateTokenCount(string(msg.Role)) + messageStructureOverhead
	}
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
	if err := mi.store.IncrementTokenCount(ctx, mi.id, totalTokenCount); err != nil {
		log.Warn("Failed to update token count metadata after append_many",
			"error", err,
			"memory_id", mi.id,
			"token_count", totalTokenCount)
		// NOTE: Continue processing because append persistence succeeded even if metadata update failed.
	}
	mi.metrics.RecordTokenCount(ctx, totalTokenCount)
}

func (mi *memoryInstance) recordAppendMetrics(ctx context.Context, start time.Time, tokenCount int, err error) {
	mi.metrics.RecordAppend(ctx, time.Since(start), tokenCount, err)
}

func (mi *memoryInstance) acquireAppendLock(ctx context.Context, operation string) (func(), *error, error) {
	lock, err := mi.lockManager.AcquireAppendLock(ctx, mi.id)
	if err != nil {
		return nil, nil, err
	}
	var releaseErr error
	release := func() {
		if unlockErr := lock(); unlockErr != nil {
			releaseErr = unlockErr
			logger.FromContext(ctx).Error("Failed to release lock",
				"error", unlockErr,
				"operation", operation,
				"memory_id", mi.id,
				"context", fmt.Sprintf("memory_%s_operation", operation))
		}
	}
	return release, &releaseErr, nil
}

func (mi *memoryInstance) applyReleaseError(resultErr *error, releaseErrPtr *error) {
	if resultErr == nil || releaseErrPtr == nil || *releaseErrPtr == nil || *resultErr == nil {
		return
	}
	releaseErr := *releaseErrPtr
	*resultErr = fmt.Errorf("%w (also failed to release lock: %v)", *resultErr, releaseErr)
}

func (mi *memoryInstance) enqueueAsyncTokenReconciliation(ctx context.Context, msg llm.Message) {
	if mi.asyncTokenCounter == nil {
		return
	}
	base := context.WithoutCancel(ctx)
	go mi.reconcileAsyncTokenCount(base, msg)
}

func (mi *memoryInstance) enqueueAsyncTokenReconciliationForSlice(ctx context.Context, msgs []llm.Message) {
	if mi.asyncTokenCounter == nil {
		return
	}
	base := context.WithoutCancel(ctx)
	for _, msg := range msgs {
		message := msg
		go mi.reconcileAsyncTokenCount(base, message)
	}
}

func (mi *memoryInstance) updateExpirationIfNeeded(ctx context.Context) {
	if mi.resourceConfig == nil || mi.resourceConfig.Persistence.ParsedTTL <= 0 {
		return
	}
	log := logger.FromContext(ctx)
	currentTTL, err := mi.store.GetKeyTTL(ctx, mi.id)
	if err != nil {
		log.Warn("Failed to get current TTL", "error", err, "memory_id", mi.id)
		currentTTL = 0
	}
	targetTTL := mi.resourceConfig.Persistence.ParsedTTL
	if currentTTL > 0 && currentTTL >= targetTTL/2 {
		return
	}
	if err := mi.store.SetExpiration(ctx, mi.id, targetTTL); err != nil {
		log.Error("Failed to set TTL on memory", "error", err, "memory_id", mi.id)
		return
	}
	log.Debug("Set TTL on memory", "memory_id", mi.id, "ttl", targetTTL)
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
	if metadata.DoNotPersist {
		log.Debug("Message marked as DoNotPersist, skipping storage",
			"message_role", msg.Role,
			"memory_id", mi.id)
		return nil
	}
	if mi.privacyManager != nil {
		log.Debug("Privacy manager available but not fully integrated yet",
			"memory_id", mi.id)
	}
	return mi.Append(ctx, msg)
}

func (mi *memoryInstance) PerformFlush(ctx context.Context) (*core.FlushMemoryActivityOutput, error) {
	return mi.PerformFlushWithStrategy(ctx, core.FlushingStrategyType(""))
}

// PerformFlushWithStrategy implements DynamicFlushableMemory interface
func (mi *memoryInstance) PerformFlushWithStrategy(
	ctx context.Context,
	strategyType core.FlushingStrategyType,
) (*core.FlushMemoryActivityOutput, error) {
	if strategyType != "" {
		if err := mi.validateStrategyType(string(strategyType)); err != nil {
			return nil, fmt.Errorf("invalid strategy type: %w", err)
		}
	}
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
	return mi.strategyFactory.ValidateStrategyType(strategyType)
}

func (mi *memoryInstance) MarkFlushPending(ctx context.Context, pending bool) error {
	return mi.store.MarkFlushPending(ctx, mi.id, pending)
}

func (mi *memoryInstance) checkFlushTrigger(_ context.Context) {
	mi.debouncedFlush()
}

// performAsyncFlushCheck executes the flush check logic asynchronously.
// It creates a timeout context to prevent goroutine leaks and checks
// if flushing should be triggered based on token and message counts.
func (mi *memoryInstance) performAsyncFlushCheck(ctx context.Context) error {
	log := logger.FromContext(ctx)
	// NOTE: Wrap flush checks with a timeout to prevent leaked goroutines when callers cancel late.
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if mi.isContextCanceled(timeoutCtx, "Flush trigger check canceled") {
		return nil
	}
	tokenCount, err := mi.getTokenCountWithCheck(timeoutCtx)
	if err != nil {
		return err
	}
	if mi.isContextCanceled(timeoutCtx, "Flush trigger check canceled after token count") {
		return nil
	}
	messageCount, err := mi.getMessageCountWithCheck(timeoutCtx)
	if err != nil {
		return err
	}
	if mi.isContextCanceled(timeoutCtx, "Flush trigger check canceled before flush decision") {
		return nil
	}
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
	mi.flushMutex.Lock()
	if mi.flushCancelFunc != nil {
		mi.flushCancelFunc()
		mi.flushCancelFunc = nil // Prevent double cancellation
	}
	mi.flushMutex.Unlock()
	mi.flushWG.Wait()
	mi.flushMutex.Lock()
	defer mi.flushMutex.Unlock()
	if err := mi.performAsyncFlushCheck(ctx); err != nil {
		log.Error("Failed to perform final flush during close", "error", err, "memory_id", mi.id)
		return fmt.Errorf("failed to perform final flush during close: %w", err)
	}
	log.Info("Memory instance flushed and closed successfully", "memory_id", mi.id)
	return nil
}
