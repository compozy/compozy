package instance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sethvargo/go-retry"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"go.temporal.io/sdk/client"
)

var (
	// ErrFlushAlreadyPending is returned when a flush is already in progress
	ErrFlushAlreadyPending = errors.New("flush already pending by another process")
)

// isTransientError determines if an error is transient and should be retried
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Redis-specific transient errors
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "temporary failure") ||
		strings.Contains(err.Error(), "network is unreachable") {
		return true
	}

	// Redis library specific errors
	if errors.Is(err, redis.Nil) {
		return false // Not found is not transient
	}

	// Network errors
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Syscall errors that are transient
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ETIMEDOUT, syscall.ECONNRESET:
			return true
		}
	}

	// Lock timeout errors are transient
	if strings.Contains(err.Error(), "lock timeout") ||
		strings.Contains(err.Error(), "failed to acquire lock") {
		return true
	}

	return false
}

// FlushOperations handles memory flushing operations
type FlushOperations struct {
	instanceID        string
	resourceID        string
	projectID         string
	store             memcore.Store
	lockManager       LockManager
	flushingStrategy  FlushStrategy
	operations        *Operations
	metrics           Metrics
	temporalClient    client.Client
	temporalTaskQueue string
}

// FlushOperationsOptions contains options for creating flush operations
type FlushOperationsOptions struct {
	InstanceID        string
	ResourceID        string
	ProjectID         string
	Store             memcore.Store
	LockManager       LockManager
	FlushingStrategy  FlushStrategy
	Operations        *Operations
	Metrics           Metrics
	TemporalClient    client.Client
	TemporalTaskQueue string
}

// NewFlushOperations creates a new flush operations handler
func NewFlushOperations(opts *FlushOperationsOptions) *FlushOperations {
	return &FlushOperations{
		instanceID:        opts.InstanceID,
		resourceID:        opts.ResourceID,
		projectID:         opts.ProjectID,
		store:             opts.Store,
		lockManager:       opts.LockManager,
		flushingStrategy:  opts.FlushingStrategy,
		operations:        opts.Operations,
		metrics:           opts.Metrics,
		temporalClient:    opts.TemporalClient,
		temporalTaskQueue: opts.TemporalTaskQueue,
	}
}

// ShouldScheduleFlush determines if a flush should be scheduled
func (f *FlushOperations) ShouldScheduleFlush(ctx context.Context, config *memcore.Resource) (bool, error) {
	// Check if flushing is disabled
	if config.DisableFlush {
		return false, nil
	}

	// Check if flush is already pending
	isPending, err := f.isFlushPending(ctx)
	if err != nil {
		return false, err
	}
	if isPending {
		return false, nil
	}

	// Get current state
	tokenCount, err := f.operations.GetTokenCount(ctx)
	if err != nil {
		return false, err
	}

	messageCount, err := f.operations.GetMessageCount(ctx)
	if err != nil {
		return false, err
	}

	// Check with strategy
	return f.flushingStrategy.ShouldFlush(tokenCount, messageCount, config), nil
}

// ScheduleFlushWorkflow schedules a flush workflow
func (f *FlushOperations) ScheduleFlushWorkflow(ctx context.Context) error {
	workflowID := fmt.Sprintf("flush-%s-%s", f.instanceID, core.MustNewID())
	workflowOptions := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: f.temporalTaskQueue,
	}

	input := f.createFlushInput()
	_, err := f.temporalClient.ExecuteWorkflow(ctx, workflowOptions, "FlushMemoryWorkflow", input)
	if err != nil {
		return fmt.Errorf("failed to start flush workflow: %w", err)
	}

	return nil
}

// PerformFlush executes the complete memory flush operation with retry logic
func (f *FlushOperations) PerformFlush(
	ctx context.Context,
	config *memcore.Resource,
) (*memcore.FlushMemoryActivityOutput, error) {
	start := time.Now()

	var result *memcore.FlushMemoryActivityOutput
	var finalErr error

	// Retry logic with exponential backoff for transient failures
	retryConfig := retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond))

	err := retry.Do(ctx, retryConfig, func(ctx context.Context) error {
		// Acquire flush lock
		unlock, err := f.lockManager.AcquireFlushLock(ctx, f.instanceID)
		if err != nil {
			if isTransientError(err) {
				f.operations.logger.Warn("Transient error acquiring flush lock, will retry",
					"error", err,
					"instance_id", f.instanceID)
				return retry.RetryableError(err)
			}
			return err // Permanent error, don't retry
		}
		defer func() {
			if err := unlock(); err != nil {
				f.operations.logger.Error("Failed to release flush lock",
					"error", err,
					"instance_id", f.instanceID,
					"operation", "flush_lock_release")
			}
		}()

		// Check if already pending
		isPending, err := f.isFlushPending(ctx)
		if err != nil {
			if isTransientError(err) {
				f.operations.logger.Warn("Transient error checking flush pending state, will retry",
					"error", err,
					"instance_id", f.instanceID)
				return retry.RetryableError(err)
			}
			return err // Permanent error, don't retry
		}
		if isPending {
			return ErrFlushAlreadyPending // Don't retry this condition
		}

		// Mark flush as pending
		if err := f.markFlushPending(ctx, true); err != nil {
			if isTransientError(err) {
				f.operations.logger.Warn("Transient error marking flush as pending, will retry",
					"error", err,
					"instance_id", f.instanceID)
				return retry.RetryableError(err)
			}
			return err // Permanent error, don't retry
		}
		defer func() {
			// Always clear the pending flag
			if err := f.markFlushPending(ctx, false); err != nil {
				f.operations.logger.Error("Failed to clear flush pending flag during cleanup",
					"error", err,
					"instance_id", f.instanceID,
					"operation", "flush_cleanup")
			}
		}()

		// Execute the flush
		result, err = f.executeFlush(ctx, config)
		if err != nil && isTransientError(err) {
			f.operations.logger.Warn("Transient error during flush execution, will retry",
				"error", err,
				"instance_id", f.instanceID)
			return retry.RetryableError(err)
		}

		finalErr = err
		return err // Either success (nil) or permanent error
	})

	// Record metrics with the final result
	if result != nil {
		f.metrics.RecordFlush(ctx, time.Since(start), result.MessageCount, finalErr)
	} else {
		f.metrics.RecordFlush(ctx, time.Since(start), 0, finalErr)
	}

	// Return the actual error from retry.Do if it failed
	if err != nil {
		return nil, err
	}

	return result, finalErr
}

// executeFlush performs the actual flush operation
func (f *FlushOperations) executeFlush(
	ctx context.Context,
	config *memcore.Resource,
) (*memcore.FlushMemoryActivityOutput, error) {
	// Read all messages
	messages, err := f.store.ReadMessages(ctx, f.instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages for flush: %w", err)
	}

	if len(messages) == 0 {
		return &memcore.FlushMemoryActivityOutput{
			Success:          true,
			MessageCount:     0,
			TokenCount:       0,
			SummaryGenerated: false,
		}, nil
	}

	// Calculate current tokens
	currentTokens := 0
	for _, msg := range messages {
		currentTokens += f.calculateTokenCount(ctx, msg)
	}

	// Execute flushing strategy
	result, err := f.flushingStrategy.PerformFlush(ctx, messages, config)
	if err != nil {
		return nil, fmt.Errorf("flushing strategy failed: %w", err)
	}

	// Apply the flush results
	if err := f.applyFlushResults(ctx, result, messages, currentTokens); err != nil {
		return nil, fmt.Errorf("failed to apply flush results: %w", err)
	}

	return result, nil
}

// applyFlushResults applies the results of a flush operation
func (f *FlushOperations) applyFlushResults(
	ctx context.Context,
	result *memcore.FlushMemoryActivityOutput,
	originalMessages []llm.Message,
	originalTokens int,
) error {
	// If no messages were flushed, nothing to do
	if result.MessageCount == 0 {
		return nil
	}

	// Calculate remaining messages and tokens
	remainingMessages := originalMessages[result.MessageCount:]
	remainingTokens := originalTokens - result.TokenCount

	// Replace messages atomically
	if atomicStore, ok := f.store.(memcore.AtomicOperations); ok {
		return atomicStore.ReplaceMessagesWithMetadata(ctx, f.instanceID, remainingMessages, remainingTokens)
	}

	// Fallback to non-atomic operations
	if err := f.store.ReplaceMessages(ctx, f.instanceID, remainingMessages); err != nil {
		return err
	}

	return f.store.SetTokenCount(ctx, f.instanceID, remainingTokens)
}

// isFlushPending checks if a flush is already pending
func (f *FlushOperations) isFlushPending(ctx context.Context) (bool, error) {
	if flushStore, ok := f.store.(memcore.FlushStateStore); ok {
		return flushStore.IsFlushPending(ctx, f.instanceID)
	}
	return false, nil
}

// markFlushPending sets or clears the flush pending flag
func (f *FlushOperations) markFlushPending(ctx context.Context, pending bool) error {
	if flushStore, ok := f.store.(memcore.FlushStateStore); ok {
		return flushStore.MarkFlushPending(ctx, f.instanceID, pending)
	}
	return nil
}

// createFlushInput creates the input for the flush workflow
func (f *FlushOperations) createFlushInput() FlushMemoryActivityInput {
	return FlushMemoryActivityInput{
		MemoryInstanceKey: f.instanceID,
		MemoryResourceID:  f.resourceID,
		ProjectID:         f.projectID,
	}
}

// calculateTokenCount calculates tokens for a message
func (f *FlushOperations) calculateTokenCount(ctx context.Context, msg llm.Message) int {
	// Delegate to operations
	if f.operations != nil {
		return f.operations.calculateTokenCount(ctx, msg)
	}
	// Fallback if operations not available
	return f.estimateTokenCount(msg.Content)
}

// estimateTokenCount provides a fallback token estimation
func (f *FlushOperations) estimateTokenCount(text string) int {
	// Rough estimate: 4 characters per token (common for most tokenizers)
	tokens := len(text) / 4
	// Ensure at least 1 token for non-empty text
	if tokens == 0 && text != "" {
		tokens = 1
	}
	return tokens
}

// FlushMemoryActivityInput defines the input for flush activities
type FlushMemoryActivityInput struct {
	MemoryInstanceKey string
	MemoryResourceID  string
	ProjectID         string
}
