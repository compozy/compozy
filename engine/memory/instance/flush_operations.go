package instance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
	"go.temporal.io/sdk/client"
)

var (
	// ErrFlushAlreadyPending is returned when a flush is already in progress
	ErrFlushAlreadyPending = errors.New("flush already pending by another process")
)

// FlushOperations handles memory flushing operations
type FlushOperations struct {
	instanceID        string
	resourceID        string
	projectID         string
	store             core.Store
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
	Store             core.Store
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
func (f *FlushOperations) ShouldScheduleFlush(ctx context.Context, config *core.Resource) (bool, error) {
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
	workflowID := fmt.Sprintf("flush-%s-%d", f.instanceID, time.Now().Unix())

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

// PerformFlush executes the complete memory flush operation
func (f *FlushOperations) PerformFlush(
	ctx context.Context,
	config *core.Resource,
) (*core.FlushMemoryActivityOutput, error) {
	start := time.Now()

	// Acquire flush lock
	unlock, err := f.lockManager.AcquireFlushLock(ctx, f.instanceID)
	if err != nil {
		f.metrics.RecordFlush(ctx, time.Since(start), 0, err)
		return nil, err
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
		f.metrics.RecordFlush(ctx, time.Since(start), 0, err)
		return nil, err
	}
	if isPending {
		f.metrics.RecordFlush(ctx, time.Since(start), 0, ErrFlushAlreadyPending)
		return nil, ErrFlushAlreadyPending
	}

	// Mark flush as pending
	if err := f.markFlushPending(ctx, true); err != nil {
		f.metrics.RecordFlush(ctx, time.Since(start), 0, err)
		return nil, err
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
	result, err := f.executeFlush(ctx, config)
	if result != nil {
		f.metrics.RecordFlush(ctx, time.Since(start), result.MessageCount, err)
	} else {
		f.metrics.RecordFlush(ctx, time.Since(start), 0, err)
	}

	return result, err
}

// executeFlush performs the actual flush operation
func (f *FlushOperations) executeFlush(
	ctx context.Context,
	config *core.Resource,
) (*core.FlushMemoryActivityOutput, error) {
	// Read all messages
	messages, err := f.store.ReadMessages(ctx, f.instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages for flush: %w", err)
	}

	if len(messages) == 0 {
		return &core.FlushMemoryActivityOutput{
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
	result *core.FlushMemoryActivityOutput,
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
	if atomicStore, ok := f.store.(core.AtomicOperations); ok {
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
	if flushStore, ok := f.store.(core.FlushStateStore); ok {
		return flushStore.IsFlushPending(ctx, f.instanceID)
	}
	return false, nil
}

// markFlushPending sets or clears the flush pending flag
func (f *FlushOperations) markFlushPending(ctx context.Context, pending bool) error {
	if flushStore, ok := f.store.(core.FlushStateStore); ok {
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
