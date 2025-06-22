package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/cache" // For cache.Lock
	"github.com/compozy/compozy/engine/llm"         // For llm.Message
	"github.com/compozy/compozy/pkg/logger"         // Standard logger
	"go.temporal.io/sdk/client"                     // For Temporal client
)

// Typed errors for robust error handling
var (
	ErrFlushAlreadyPending = errors.New("flush already pending by another process")
)

// Instance implements the Memory interface and orchestrates
// storage, locking, token management, and flushing for a single memory stream.
type Instance struct {
	id                string  // The resolved, unique key for this memory instance (e.g., "user123:chat")
	resourceID        string  // The ID of the MemoryResource config that defines its behavior
	projectID         string  // Project ID for namespacing, metrics, etc. (if applicable)
	resourceConfig    *Config // Pointer to the parsed memory.Config for this instance
	store             Store
	lockManager       *LockManager // Using the wrapper from Task 1
	tokenManager      *TokenMemoryManager
	flushingStrategy  *HybridFlushingStrategy // Used to decide *if* to flush
	temporalClient    client.Client           // Temporal client to schedule activities
	temporalTaskQueue string                  // Task queue for memory activities
	privacyManager    *PrivacyManager         // Privacy controls and data protection
	log               logger.Logger
	asyncLogger       *AsyncOperationLogger // Structured logging for async operations
}

// NewInstanceOptions holds options for creating a Instance.
type NewInstanceOptions struct {
	InstanceID        string // Resolved unique key for the instance
	ResourceID        string // ID of the memory.Config definition
	ProjectID         string // Optional project ID
	ResourceConfig    *Config
	Store             Store
	LockManager       *LockManager
	TokenManager      *TokenMemoryManager
	FlushingStrategy  *HybridFlushingStrategy
	TemporalClient    client.Client
	TemporalTaskQueue string
	PrivacyManager    *PrivacyManager
	Logger            logger.Logger
}

// NewInstance creates a new memory instance.
func NewInstance(opts *NewInstanceOptions) (*Instance, error) {
	if opts.InstanceID == "" {
		return nil, fmt.Errorf("instance ID cannot be empty")
	}
	if opts.ResourceConfig == nil {
		return nil, fmt.Errorf("resource config cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.Store == nil {
		return nil, fmt.Errorf("memory store cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.LockManager == nil {
		return nil, fmt.Errorf("lock manager cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.TokenManager == nil {
		return nil, fmt.Errorf("token manager cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.FlushingStrategy == nil { // Even if not summarizing, strategy obj might be needed for ShouldFlush
		return nil, fmt.Errorf("flushing strategy cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.TemporalClient == nil {
		return nil, fmt.Errorf("temporal client cannot be nil for instance %s", opts.InstanceID)
	}
	if opts.TemporalTaskQueue == "" {
		opts.TemporalTaskQueue = "memory-operations" // Default task queue
	}
	if opts.Logger == nil {
		opts.Logger = logger.NewForTests() // Default to test logger if none provided
	}

	// Privacy manager is optional
	privacyManager := opts.PrivacyManager
	if privacyManager == nil {
		privacyManager = NewPrivacyManager()
	}
	instanceLogger := opts.Logger.With(
		"memory_instance_id",
		opts.InstanceID,
		"memory_resource_id",
		opts.ResourceID,
	)

	instance := &Instance{
		id:                opts.InstanceID,
		resourceID:        opts.ResourceID,
		projectID:         opts.ProjectID,
		resourceConfig:    opts.ResourceConfig,
		store:             opts.Store,
		lockManager:       opts.LockManager,
		tokenManager:      opts.TokenManager,
		flushingStrategy:  opts.FlushingStrategy,
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
		privacyManager:    privacyManager,
		log:               instanceLogger,
		asyncLogger:       NewAsyncOperationLogger(instanceLogger),
	}

	// Register with global health service
	RegisterInstanceGlobally(opts.InstanceID)

	return instance, nil
}

// GetID returns the unique identifier of this memory instance.
func (mi *Instance) GetID() string {
	return mi.id
}

// Append adds a message to the memory.
func (mi *Instance) Append(ctx context.Context, msg llm.Message) error {
	start := time.Now()
	mi.logAppendStart(msg)
	lock, err := mi.acquireAppendLockWithMetrics(ctx, start)
	if err != nil {
		return err
	}
	defer mi.releaseLock(ctx, lock)
	return mi.performAppendOperation(ctx, msg, start)
}

func (mi *Instance) acquireAppendLockWithMetrics(ctx context.Context, start time.Time) (cache.Lock, error) {
	RecordMemoryLockAcquire(ctx, mi.id, mi.projectID)
	return mi.acquireAppendLock(ctx, start)
}

func (mi *Instance) performAppendOperation(ctx context.Context, msg llm.Message, start time.Time) error {
	tokenCount := mi.calculateTokenCount(ctx, msg)
	if err := mi.appendMessageToStore(ctx, msg, tokenCount, start); err != nil {
		return err
	}
	mi.recordAndUpdateMetrics(ctx, tokenCount)
	mi.performPostAppendOperations(ctx)
	mi.recordSuccessfulAppend(ctx, start)
	return nil
}

func (mi *Instance) performPostAppendOperations(ctx context.Context) {
	mi.handleFlushScheduling(ctx)
	mi.handleTTLRefresh(ctx)
}

func (mi *Instance) recordSuccessfulAppend(ctx context.Context, start time.Time) {
	RecordMemoryOperation(ctx, "append", mi.id, mi.projectID, time.Since(start))
	UpdateHealthState(mi.id, true, 0)
}

func (mi *Instance) logAppendStart(msg llm.Message) {
	mi.log.Debug("Append called",
		"message_role", msg.Role,
		"memory_id", mi.id,
		"operation", "append",
		"resource_id", mi.resourceID,
		"project_id", mi.projectID)
}

func (mi *Instance) acquireAppendLock(ctx context.Context, start time.Time) (cache.Lock, error) {
	lockTTL := mi.resourceConfig.GetAppendLockTTL()
	lock, err := mi.lockManager.Acquire(ctx, mi.id, lockTTL)
	if err != nil {
		mi.log.Error("Failed to acquire lock for append", "error", err)
		RecordMemoryLockContention(ctx, mi.id, mi.projectID)
		RecordMemoryOperation(ctx, "append", mi.id, mi.projectID, time.Since(start))
		return nil, fmt.Errorf("failed to acquire lock for append on memory %s: %w", mi.id, err)
	}
	return lock, nil
}

func (mi *Instance) releaseLock(ctx context.Context, lock cache.Lock) {
	if err := lock.Release(ctx); err != nil {
		mi.log.Error("Failed to release lock after append", "error", err)
	}
}

func (mi *Instance) calculateTokenCount(ctx context.Context, msg llm.Message) int {
	if mi.tokenManager == nil {
		return 0
	}
	tokens, err := mi.tokenManager.tokenCounter.CountTokens(ctx, msg.Content)
	if err != nil {
		mi.log.Warn("Failed to count tokens", "error", err)
		return 0
	}
	return tokens
}

func (mi *Instance) appendMessageToStore(
	ctx context.Context,
	msg llm.Message,
	tokenCount int,
	start time.Time,
) error {
	if redisStore, ok := mi.store.(*RedisMemoryStore); ok {
		return mi.appendWithRedisStore(ctx, redisStore, msg, tokenCount, start)
	}
	return mi.appendWithGenericStore(ctx, msg, tokenCount, start)
}

func (mi *Instance) appendWithRedisStore(
	ctx context.Context,
	redisStore *RedisMemoryStore,
	msg llm.Message,
	tokenCount int,
	start time.Time,
) error {
	if err := redisStore.AppendMessagesWithMetadata(ctx, mi.id, []llm.Message{msg}, tokenCount); err != nil {
		mi.log.Error("Failed to append message to store", "error", err)
		RecordMemoryOperation(ctx, "append", mi.id, mi.projectID, time.Since(start))
		return fmt.Errorf("failed to append message to store for memory %s: %w", mi.id, err)
	}
	return nil
}

func (mi *Instance) appendWithGenericStore(
	ctx context.Context,
	msg llm.Message,
	tokenCount int,
	start time.Time,
) error {
	// Use atomic append with token count to prevent race conditions
	if err := mi.store.AppendMessageWithTokenCount(ctx, mi.id, msg, tokenCount); err != nil {
		mi.log.Error("Failed to append message to store", "error", err)
		RecordMemoryOperation(ctx, "append", mi.id, mi.projectID, time.Since(start))
		return fmt.Errorf("failed to append message to store for memory %s: %w", mi.id, err)
	}
	return nil
}

func (mi *Instance) recordAndUpdateMetrics(ctx context.Context, tokenCount int) {
	RecordMemoryMessage(ctx, mi.id, mi.projectID, int64(tokenCount))
	if mi.tokenManager == nil {
		return
	}
	totalTokens, err := mi.GetTokenCount(ctx)
	if err != nil {
		return
	}
	UpdateTokenUsageState(mi.id, int64(totalTokens), int64(mi.resourceConfig.MaxTokens))
	mi.asyncLogger.LogTokenManagement(ctx, mi.id, "append", totalTokens, mi.resourceConfig.MaxTokens, map[string]any{
		"new_message_tokens": tokenCount,
		"resource_id":        mi.resourceID,
	})
}

func (mi *Instance) handleFlushScheduling(ctx context.Context) {
	if !mi.isFlushingEnabled() {
		return
	}
	if !mi.shouldScheduleFlush(ctx) {
		return
	}
	mi.executeFlushScheduling(ctx)
}

func (mi *Instance) isFlushingEnabled() bool {
	return mi.resourceConfig.Flushing != nil && mi.resourceConfig.Flushing.Type != ""
}

func (mi *Instance) executeFlushScheduling(ctx context.Context) {
	workflowID := fmt.Sprintf("memory-flush-%s", mi.id)
	options := mi.createFlushWorkflowOptions(workflowID)
	flushInput := mi.createFlushInput()
	mi.logFlushWorkflow(ctx, workflowID)
	mi.scheduleFlushWorkflow(ctx, &options, flushInput, workflowID)
}

func (mi *Instance) createFlushWorkflowOptions(workflowID string) client.StartWorkflowOptions {
	return client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                mi.temporalTaskQueue,
		WorkflowExecutionTimeout: 15 * time.Minute,
	}
}

func (mi *Instance) createFlushInput() FlushMemoryActivityInput {
	return FlushMemoryActivityInput{
		MemoryInstanceKey: mi.id,
		MemoryResourceID:  mi.resourceID,
		ProjectID:         mi.projectID,
	}
}

func (mi *Instance) logFlushWorkflow(ctx context.Context, workflowID string) {
	mi.asyncLogger.LogTemporalWorkflow(ctx, workflowID, "FlushMemoryWorkflow", mi.id, map[string]any{
		"resource_id": mi.resourceID,
		"project_id":  mi.projectID,
		"flush_type":  string(mi.resourceConfig.Flushing.Type),
	})
}

func (mi *Instance) scheduleFlushWorkflow(
	ctx context.Context,
	options *client.StartWorkflowOptions,
	flushInput FlushMemoryActivityInput,
	workflowID string,
) {
	if err := mi.MarkFlushPending(ctx, true); err != nil {
		if errors.Is(err, ErrFlushAlreadyPending) {
			mi.log.Debug("Skipping flush schedule, another process won the race")
		} else {
			mi.log.Warn("Failed to mark flush as pending, skipping schedule", "error", err)
		}
		return
	}
	_, err := mi.temporalClient.ExecuteWorkflow(ctx, *options, "FlushMemoryWorkflow", flushInput)
	if err != nil {
		mi.log.Error("Failed to schedule FlushMemory workflow", "error", err, "workflow_id", workflowID)
		// Clear the flag for all errors including AlreadyStartedError
		// This ensures we don't block flush for 30 minutes if Temporal accepted the request
		if clearErr := mi.MarkFlushPending(ctx, false); clearErr != nil {
			mi.log.Warn("Failed to clear flush pending flag", "error", clearErr)
		}
	} else {
		RecordTemporalActivity(ctx, mi.id, "flush_workflow_scheduled", mi.projectID)
		mi.log.Debug("Flush workflow scheduled", "workflow_id", workflowID)
	}
}

func (mi *Instance) handleTTLRefresh(ctx context.Context) {
	if mi.resourceConfig.Persistence.TTL == "" {
		return
	}
	ttl, parseErr := time.ParseDuration(mi.resourceConfig.Persistence.TTL)
	if parseErr != nil || ttl <= 0 {
		return
	}
	if err := mi.store.SetExpiration(ctx, mi.id, ttl); err != nil {
		mi.log.Warn("Failed to set/refresh TTL after append", "error", err)
	}
}

// Read retrieves all messages from the memory.
func (mi *Instance) Read(ctx context.Context) ([]llm.Message, error) {
	start := time.Now()
	mi.log.Debug("Read called")
	// No lock needed for read by default, assuming eventual consistency is acceptable
	// or that Redis list operations are sufficiently atomic for reads.
	// If strong consistency is needed even for reads relative to writes/flushes,
	// a read lock or different strategy might be required.
	messages, err := mi.store.ReadMessages(ctx, mi.id)
	if err != nil {
		mi.log.Error("Failed to read messages from store", "error", err)
		RecordMemoryOperation(ctx, "read", mi.id, mi.projectID, time.Since(start))
		UpdateHealthState(mi.id, false, 1)
		return nil, fmt.Errorf("failed to read messages from store for memory %s: %w", mi.id, err)
	}
	RecordMemoryOperation(ctx, "read", mi.id, mi.projectID, time.Since(start))
	UpdateHealthState(mi.id, true, 0)
	return messages, nil
}

// Len returns the number of messages in the memory.
func (mi *Instance) Len(ctx context.Context) (int, error) {
	mi.log.Debug("Len called")
	count, err := mi.store.CountMessages(ctx, mi.id)
	if err != nil {
		mi.log.Error("Failed to count messages in store", "error", err)
		return 0, fmt.Errorf("failed to count messages in store for memory %s: %w", mi.id, err)
	}
	return count, nil
}

// GetTokenCount returns the current estimated token count of the messages in memory.
// Optimized to use cached metadata for O(1) performance.
func (mi *Instance) GetTokenCount(ctx context.Context) (int, error) {
	mi.log.Debug("GetTokenCount called")
	// First try to get from metadata (O(1) operation)
	tokenCount, err := mi.store.GetTokenCount(ctx, mi.id)
	if err != nil {
		mi.log.Error("Failed to get token count from metadata", "error", err)
		return 0, fmt.Errorf("failed to get token count from metadata for memory %s: %w", mi.id, err)
	}
	// If we have a cached count, return it
	if tokenCount > 0 {
		mi.log.Debug("Token count retrieved from metadata", "count", tokenCount)
		return tokenCount, nil
	}
	// No metadata exists - perform one-time migration with lock
	mi.log.Info("Token count metadata not found, performing migration", "memory_id", mi.id)

	// Acquire lock for migration to ensure atomicity
	migrationLockKey := mi.id + ":token_migration"
	lock, err := mi.lockManager.Acquire(ctx, migrationLockKey, 30*time.Second)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire lock for token count migration: %w", err)
	}
	defer mi.releaseLock(ctx, lock)

	// Re-check after acquiring lock in case another process just finished migration
	tokenCount, err = mi.store.GetTokenCount(ctx, mi.id)
	if err == nil && tokenCount > 0 {
		mi.log.Debug("Token count found after acquiring lock, another process completed migration", "count", tokenCount)
		return tokenCount, nil
	}

	messages, err := mi.Read(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to read messages for token count migration: %w", err)
	}
	if len(messages) == 0 {
		// Set metadata to 0 for empty memory
		if err := mi.store.SetTokenCount(ctx, mi.id, 0); err != nil {
			mi.log.Warn("Failed to set initial token count metadata", "error", err)
		}
		return 0, nil
	}
	// Calculate tokens for all messages
	_, totalTokens, err := mi.tokenManager.CalculateMessagesWithTokens(ctx, messages)
	if err != nil {
		mi.log.Error("Failed to calculate tokens during migration", "error", err)
		return 0, fmt.Errorf("failed to calculate tokens for memory %s: %w", mi.id, err)
	}
	// Store the calculated count in metadata for future use
	if err := mi.store.SetTokenCount(ctx, mi.id, totalTokens); err != nil {
		mi.log.Warn("Failed to store token count metadata during migration", "error", err)
		// Continue anyway - we have the count
	} else {
		mi.log.Info("Token count metadata migration completed",
			"memory_id", mi.id,
			"token_count", totalTokens,
			"message_count", len(messages))
	}
	return totalTokens, nil
}

// GetMemoryHealth returns diagnostic information about the memory instance.
func (mi *Instance) GetMemoryHealth(ctx context.Context) (*Health, error) {
	mi.log.Debug("GetMemoryHealth called")
	count, err := mi.Len(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count for health check: %w", err)
	}
	tokens, err := mi.GetTokenCount(ctx) // Now O(1) with metadata
	if err != nil {
		// Don't fail health check entirely, but report partial data
		mi.log.Warn("Failed to get token count for health check", "error", err)
		tokens = -1 // Indicate error or unknown
	}

	// LastFlush and FlushStrategy would require more state tracking or querying
	// For now, these are placeholders.
	health := &Health{
		MessageCount: count,
		TokenCount:   tokens,
		// LastFlush: nil, // Need a way to get this, e.g. from Temporal activity logs or separate store field
		FlushStrategy: string(mi.resourceConfig.Flushing.Type),
	}
	return health, nil
}

// Clear removes all messages from the memory.
func (mi *Instance) Clear(ctx context.Context) error {
	start := time.Now()
	mi.log.Info("Clear called, deleting all messages")

	// Record lock acquisition attempt
	RecordMemoryLockAcquire(ctx, mi.id, mi.projectID)

	lockTTL := mi.resourceConfig.GetClearLockTTL()
	lock, err := mi.lockManager.Acquire(ctx, mi.id, lockTTL)
	if err != nil {
		mi.log.Error("Failed to acquire lock for clear", "error", err)
		RecordMemoryLockContention(ctx, mi.id, mi.projectID)
		RecordMemoryOperation(ctx, "clear", mi.id, mi.projectID, time.Since(start))
		return fmt.Errorf("failed to acquire lock for clear on memory %s: %w", mi.id, err)
	}
	defer func() {
		if err := lock.Release(ctx); err != nil {
			mi.log.Error("Failed to release lock after clear", "error", err)
		}
	}()

	if err := mi.store.DeleteMessages(ctx, mi.id); err != nil {
		mi.log.Error("Failed to delete messages from store", "error", err)
		RecordMemoryOperation(ctx, "clear", mi.id, mi.projectID, time.Since(start))
		UpdateHealthState(mi.id, false, 1)
		return fmt.Errorf("failed to delete messages from store for memory %s: %w", mi.id, err)
	}
	// Reset token count metadata
	if err := mi.store.SetTokenCount(ctx, mi.id, 0); err != nil {
		mi.log.Warn("Failed to reset token count metadata", "error", err)
		// Non-fatal - continue
	}

	// Update token usage state to 0
	UpdateTokenUsageState(mi.id, 0, int64(mi.resourceConfig.MaxTokens))

	// Record successful clear
	RecordMemoryOperation(ctx, "clear", mi.id, mi.projectID, time.Since(start))
	UpdateHealthState(mi.id, true, 0)

	return nil
}

// HealthCheck performs a basic health check of the memory instance's dependencies (e.g., store).
// This is different from GetMemoryHealth which returns operational stats.
func (mi *Instance) HealthCheck(ctx context.Context) error {
	// Check store health (e.g., Redis ping if store is Redis-based)
	// This requires Store to have a HealthCheck method, or we check a raw client.
	// For now, let's assume the underlying client used by the store can be pinged.
	// This is a simplification.
	if storePinger, ok := mi.store.(interface{ Ping(context.Context) error }); ok {
		if err := storePinger.Ping(ctx); err != nil {
			return fmt.Errorf("memory store ping failed for instance %s: %w", mi.id, err)
		}
	}
	// Could also check lock manager connectivity if it has a similar ping/health method.
	mi.log.Debug("HealthCheck passed")
	return nil
}

// Verify that Instance implements the FlushableMemory interface
var _ FlushableMemory = (*Instance)(nil)

// PerformFlush executes the complete memory flush operation.
// This includes reading messages, checking flush conditions, applying strategies, and persisting results.
func (mi *Instance) PerformFlush(ctx context.Context) (*FlushMemoryActivityOutput, error) {
	start := time.Now()
	mi.log.Debug("PerformFlush called", "memory_id", mi.id)
	lock, err := mi.acquireFlushLock(ctx)
	if err != nil {
		return nil, err
	}
	defer mi.releaseFlushLock(ctx, lock)
	return mi.executeFlushWithLock(ctx, start)
}

func (mi *Instance) executeFlushWithLock(
	ctx context.Context,
	start time.Time,
) (*FlushMemoryActivityOutput, error) {
	mi.log.Info("Lock acquired for flushing", "memory_id", mi.id)
	currentRawMessages, err := mi.readMessagesForFlush(ctx)
	if err != nil {
		return nil, err
	}
	if len(currentRawMessages) == 0 {
		return mi.createEmptyFlushResult(), nil
	}
	return mi.processMessagesForFlush(ctx, currentRawMessages, start)
}

func (mi *Instance) processMessagesForFlush(
	ctx context.Context,
	currentRawMessages []llm.Message,
	start time.Time,
) (*FlushMemoryActivityOutput, error) {
	currentMessagesWithTokens, currentTotalTokens, err := mi.calculateTokensForFlush(ctx, currentRawMessages)
	if err != nil {
		return nil, err
	}
	if !mi.flushingStrategy.ShouldFlush(ctx, currentMessagesWithTokens, currentTotalTokens) {
		return mi.handleLimitsOnlyFlush(ctx, currentMessagesWithTokens, currentTotalTokens, start)
	}
	return mi.performActualFlush(ctx, currentMessagesWithTokens, start)
}

func (mi *Instance) acquireFlushLock(ctx context.Context) (cache.Lock, error) {
	lockTTL := mi.resourceConfig.GetFlushLockTTL()
	lock, err := mi.lockManager.Acquire(ctx, mi.id, lockTTL)
	if err != nil {
		mi.log.Error("Failed to acquire lock for flushing", "error", err)
		RecordMemoryLockContention(ctx, mi.id, mi.projectID)
		return nil, fmt.Errorf("failed to acquire lock for flush: %w", err)
	}
	return lock, nil
}

func (mi *Instance) releaseFlushLock(ctx context.Context, lock cache.Lock) {
	if err := lock.Release(ctx); err != nil {
		mi.log.Error("Failed to release lock after flushing", "error", err)
	}
}

func (mi *Instance) readMessagesForFlush(ctx context.Context) ([]llm.Message, error) {
	currentRawMessages, err := mi.store.ReadMessages(ctx, mi.id)
	if err != nil {
		mi.log.Error("Failed to read messages for flushing", "error", err)
		return nil, fmt.Errorf("failed to read messages: %w", err)
	}
	return currentRawMessages, nil
}

func (mi *Instance) createEmptyFlushResult() *FlushMemoryActivityOutput {
	mi.log.Info("No messages to flush", "memory_id", mi.id)
	return &FlushMemoryActivityOutput{
		MessageCount: 0,
		TokenCount:   0,
		Success:      true,
	}
}

func (mi *Instance) calculateTokensForFlush(
	ctx context.Context,
	currentRawMessages []llm.Message,
) ([]MessageWithTokens, int, error) {
	currentMessagesWithTokens, currentTotalTokens, err := mi.tokenManager.CalculateMessagesWithTokens(
		ctx,
		currentRawMessages,
	)
	if err != nil {
		mi.log.Error("Failed to calculate tokens during flush", "error", err)
		return nil, 0, fmt.Errorf("token calculation failed: %w", err)
	}
	mi.log.Info(
		"Calculated initial tokens",
		"memory_id",
		mi.id,
		"messages",
		len(currentMessagesWithTokens),
		"tokens",
		currentTotalTokens,
	)
	return currentMessagesWithTokens, currentTotalTokens, nil
}

func (mi *Instance) handleLimitsOnlyFlush(
	ctx context.Context,
	currentMessagesWithTokens []MessageWithTokens,
	currentTotalTokens int,
	start time.Time,
) (*FlushMemoryActivityOutput, error) {
	mi.log.Info("Flush condition not met, applying limits only", "memory_id", mi.id, "tokens", currentTotalTokens)
	finalMessages, finalTokens, err := mi.tokenManager.EnforceLimits(ctx, currentMessagesWithTokens, currentTotalTokens)
	if err != nil {
		mi.log.Error("Error enforcing limits", "error", err)
	}
	if len(finalMessages) != len(currentMessagesWithTokens) {
		if err := mi.saveMessagesAfterLimits(ctx, finalMessages, finalTokens); err != nil {
			return nil, err
		}
		mi.log.Info(
			"Applied general limits",
			"memory_id",
			mi.id,
			"messages_kept",
			len(finalMessages),
			"tokens_kept",
			finalTokens,
		)
	}
	RecordMemoryOperation(ctx, "flush_check", mi.id, mi.projectID, time.Since(start))
	return &FlushMemoryActivityOutput{
		MessageCount: len(finalMessages),
		TokenCount:   finalTokens,
		Success:      true,
	}, nil
}

func (mi *Instance) performActualFlush(
	ctx context.Context,
	currentMessagesWithTokens []MessageWithTokens,
	start time.Time,
) (*FlushMemoryActivityOutput, error) {
	mi.log.Info("Flush condition met, proceeding with flush", "memory_id", mi.id)
	flushResult, err := mi.executeFlushStrategy(ctx, currentMessagesWithTokens)
	if err != nil {
		return nil, err
	}
	finalResult, err := mi.applyLimitsAfterFlush(ctx, flushResult)
	if err != nil {
		return nil, err
	}
	if err := mi.saveFlushResults(ctx, finalResult.messages, finalResult.tokens); err != nil {
		return nil, err
	}
	mi.completeFlushOperation(ctx, finalResult.tokens, flushResult.summaryGenerated, start)
	return mi.createFlushOutput(finalResult, flushResult.summaryGenerated), nil
}

type flushStrategyResult struct {
	messages         []MessageWithTokens
	tokens           int
	summaryGenerated bool
}

type finalFlushResult struct {
	messages []MessageWithTokens
	tokens   int
}

func (mi *Instance) executeFlushStrategy(
	ctx context.Context,
	currentMessagesWithTokens []MessageWithTokens,
) (*flushStrategyResult, error) {
	newMessagesWithTokens, newTotalTokens, summaryGenerated, flushErr := mi.flushingStrategy.FlushMessages(
		ctx,
		currentMessagesWithTokens,
	)
	if flushErr != nil {
		mi.log.Error("Failed to flush messages", "error", flushErr)
		return nil, fmt.Errorf("flushing failed: %w", flushErr)
	}
	mi.log.Info(
		"Flush executed",
		"memory_id",
		mi.id,
		"messages_after",
		len(newMessagesWithTokens),
		"tokens_after",
		newTotalTokens,
		"summary_generated",
		summaryGenerated,
	)
	return &flushStrategyResult{
		messages:         newMessagesWithTokens,
		tokens:           newTotalTokens,
		summaryGenerated: summaryGenerated,
	}, nil
}

func (mi *Instance) applyLimitsAfterFlush(
	ctx context.Context,
	flushResult *flushStrategyResult,
) (*finalFlushResult, error) {
	finalMessagesAfterLimits, finalTokensAfterLimits, limitErr := mi.tokenManager.EnforceLimits(
		ctx,
		flushResult.messages,
		flushResult.tokens,
	)
	if limitErr != nil {
		mi.log.Error("Failed to enforce limits after flushing", "error", limitErr)
		return nil, fmt.Errorf("limit enforcement post-flush failed: %w", limitErr)
	}
	mi.log.Info(
		"Limits enforced post-flush",
		"memory_id",
		mi.id,
		"messages_final",
		len(finalMessagesAfterLimits),
		"tokens_final",
		finalTokensAfterLimits,
	)
	return &finalFlushResult{
		messages: finalMessagesAfterLimits,
		tokens:   finalTokensAfterLimits,
	}, nil
}

func (mi *Instance) createFlushOutput(
	finalResult *finalFlushResult,
	summaryGenerated bool,
) *FlushMemoryActivityOutput {
	return &FlushMemoryActivityOutput{
		MessageCount:     len(finalResult.messages),
		TokenCount:       finalResult.tokens,
		SummaryGenerated: summaryGenerated,
		Success:          true,
	}
}

func (mi *Instance) saveMessagesAfterLimits(
	ctx context.Context,
	finalMessages []MessageWithTokens,
	finalTokens int,
) error {
	finalLLMMessages := MessagesWithTokensToLLMMessages(finalMessages)
	return mi.saveMessagesToStore(ctx, finalLLMMessages, finalTokens, "failed to save messages post-limit")
}

func (mi *Instance) saveFlushResults(
	ctx context.Context,
	finalMessages []MessageWithTokens,
	finalTokens int,
) error {
	finalLLMMessages := MessagesWithTokensToLLMMessages(finalMessages)
	return mi.saveMessagesToStore(ctx, finalLLMMessages, finalTokens, "failed to save messages post-flush")
}

func (mi *Instance) saveMessagesToStore(
	ctx context.Context,
	messages []llm.Message,
	tokens int,
	errorMsg string,
) error {
	if err := mi.store.ReplaceMessagesWithMetadata(ctx, mi.id, messages, tokens); err != nil {
		mi.log.Error("Failed to save messages with metadata", "error", err)
		return fmt.Errorf("%s: %w", errorMsg, err)
	}
	return nil
}

func (mi *Instance) completeFlushOperation(
	ctx context.Context,
	finalTokens int,
	summaryGenerated bool,
	start time.Time,
) {
	RecordMemoryOperation(ctx, "flush_complete", mi.id, mi.projectID, time.Since(start))
	flushType := "simple_fifo"
	if summaryGenerated {
		flushType = "hybrid_summary"
	}
	RecordMemoryFlush(ctx, mi.id, mi.projectID, flushType)
	UpdateTokenUsageState(mi.id, int64(finalTokens), int64(mi.resourceConfig.MaxTokens))
	if err := mi.MarkFlushPending(ctx, false); err != nil {
		mi.log.Warn("Failed to clear flush pending flag after flush", "error", err)
	}
	mi.log.Info("FlushMemory completed successfully", "memory_id", mi.id, "tokens_kept", finalTokens)
}

// Placeholder for the actual workflow function.
// AppendWithPrivacy adds a message to memory with privacy controls applied.
// This is a privacy-aware variant of Append that handles redaction and selective persistence.
func (mi *Instance) AppendWithPrivacy(ctx context.Context, msg llm.Message, metadata PrivacyMetadata) error {
	// Check if message should be persisted based on privacy metadata
	if metadata.DoNotPersist {
		mi.privacyManager.LogPrivacyExclusion(ctx, mi.resourceID, "do_not_persist_flag", map[string]any{
			"message_role":  msg.Role,
			"privacy_level": metadata.PrivacyLevel,
		})
		RecordPrivacyExclusion(ctx, mi.id, "do_not_persist_flag", mi.projectID)
		mi.asyncLogger.LogPrivacyOperation(ctx, mi.id, "message_excluded", map[string]any{
			"reason":        "do_not_persist_flag",
			"message_role":  msg.Role,
			"privacy_level": metadata.PrivacyLevel,
		})
		mi.log.Debug("Message not persisted due to privacy metadata", "message_role", msg.Role)
		return nil
	}
	// Check if message type should be persisted based on privacy policy
	if !mi.privacyManager.ShouldPersistMessage(mi.resourceID, msg) {
		mi.privacyManager.LogPrivacyExclusion(ctx, mi.resourceID, "non_persistable_message_type", map[string]any{
			"message_role": msg.Role,
		})
		RecordPrivacyExclusion(ctx, mi.id, "non_persistable_message_type", mi.projectID)
		mi.log.Debug("Message not persisted due to privacy policy", "message_role", msg.Role)
		return nil
	}
	// Apply redaction if not already applied
	redactedMsg := msg
	if !metadata.RedactionApplied {
		var err error
		redactedMsg, err = mi.privacyManager.RedactMessage(ctx, mi.resourceID, msg)
		if err != nil {
			mi.log.Error("Failed to apply redaction", "error", err)
			// Record circuit breaker trip if it's a circuit breaker error
			if strings.Contains(err.Error(), "circuit breaker open") {
				RecordCircuitBreakerTrip(ctx, mi.id, mi.projectID)
			}
			return fmt.Errorf("failed to apply redaction: %w", err)
		}
		// Record redaction if fields were redacted
		if len(metadata.SensitiveFields) > 0 {
			RecordRedactionOperation(ctx, mi.id, int64(len(metadata.SensitiveFields)), mi.projectID)
		}
	}
	// Call regular Append with redacted message
	return mi.Append(ctx, redactedMsg)
}

// shouldScheduleFlush performs a synchronous check to determine if a flush workflow should be scheduled.
// This prevents flooding Temporal with unnecessary workflows.
func (mi *Instance) shouldScheduleFlush(ctx context.Context) bool {
	// First check if a flush is already pending
	pending, err := mi.isFlushPending(ctx)
	if err != nil {
		mi.log.Warn("Failed to check flush pending status", "error", err)
		// On error, assume not pending to avoid missing flushes
	} else if pending {
		mi.log.Debug("Flush already pending, skipping schedule")
		return false
	}
	// Get current token count from metadata (O(1) operation)
	tokenCount, err := mi.GetTokenCount(ctx)
	if err != nil {
		mi.log.Warn("Failed to get token count for flush check", "error", err)
		return false
	}
	// Get current message count from metadata (O(1) operation)
	messageCount, err := mi.store.GetMessageCount(ctx, mi.id)
	if err != nil {
		mi.log.Warn("Failed to get message count for flush check", "error", err)
		return false
	}
	// Check if we've reached the flush threshold
	if mi.flushingStrategy != nil {
		// Create a dummy slice with the correct length for the strategy check
		// The content of messages doesn't matter, only the count and total tokens
		dummyMessages := make([]MessageWithTokens, messageCount)
		if mi.flushingStrategy.ShouldFlush(ctx, dummyMessages, tokenCount) {
			mi.log.Debug("Flush threshold reached", "token_count", tokenCount, "message_count", messageCount)
			return true
		}
	}
	return false
}

// isFlushPending checks if a flush workflow is already pending for this instance.
func (mi *Instance) isFlushPending(ctx context.Context) (bool, error) {
	return mi.store.IsFlushPending(ctx, mi.id)
}

// MarkFlushPending sets or clears the flush pending flag for this instance.
// Returns a special error "flush already pending by another process" if the flag is already set.
func (mi *Instance) MarkFlushPending(ctx context.Context, pending bool) error {
	return mi.store.MarkFlushPending(ctx, mi.id, pending)
}

// This would typically live in a different file, e.g., memory_workflows.go
// func FlushMemoryWorkflow(ctx workflow.Context,
//	input activities.FlushMemoryActivityInput) (*activities.FlushMemoryActivityOutput, error) {
// 	ao := workflow.ActivityOptions{
// 		StartToCloseTimeout: 10 * time.Minute, // Example timeout
// 		RetryPolicy: &temporal.RetryPolicy{
// 			MaximumAttempts: 3,
// 		},
// 	}
// 	ctx = workflow.WithActivityOptions(ctx, ao)
// 	var result activities.FlushMemoryActivityOutput
// 	err := workflow.ExecuteActivity(ctx, "FlushMemory", input).Get(ctx, &result)
// 	// "FlushMemory" is the activity func name
// 	return &result, err
// }
