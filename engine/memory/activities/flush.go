package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// MemoryActivities encapsulates Temporal activities related to memory management.
type MemoryActivities struct {
	// MemoryManager is the factory that creates properly configured memory instances
	// based on the MemoryResourceID provided in activity input.
	MemoryManager memory.ManagerInterface
	// Logger for activity-specific logging
	Logger logger.Logger
}

// NewMemoryActivities creates a new instance of MemoryActivities.
func NewMemoryActivities(
	manager memory.ManagerInterface,
	log logger.Logger,
) *MemoryActivities {
	if log == nil {
		log = logger.NewForTests()
	}
	return &MemoryActivities{
		MemoryManager: manager,
		Logger:        log,
	}
}

// FlushMemory is a Temporal activity that performs a memory flush operation.
// It's designed to be called asynchronously.
func (ma *MemoryActivities) FlushMemory(
	ctx context.Context,
	input memory.FlushMemoryActivityInput,
) (*memory.FlushMemoryActivityOutput, error) {
	log := activity.GetLogger(ctx)
	log.Info("FlushMemory activity started", "MemoryKey", input.MemoryInstanceKey, "ResourceID", input.MemoryResourceID)
	// Get memory instance from manager using the provided resource ID and instance key
	// The manager will dynamically construct the correct instance with all dependencies
	memRef := core.MemoryReference{
		ID:  input.MemoryResourceID,
		Key: input.MemoryInstanceKey, // Already resolved key from the workflow
	}
	// Build minimal workflow context for GetInstance
	// Since the key is already resolved, we don't need template evaluation data
	workflowContext := map[string]any{
		"memory_instance_key": input.MemoryInstanceKey,
		"memory_resource_id":  input.MemoryResourceID,
		"project.id":          input.ProjectID,
	}
	// Get the memory instance with all its configured dependencies
	memInstance, err := ma.MemoryManager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		log.Error("Failed to get memory instance", "Error", err)
		// Configuration errors should not be retried - use typed error checking
		var configErr *memory.ConfigError
		if errors.As(err, &configErr) {
			return nil, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("memory configuration error: %s", err.Error()),
				"MEMORY_CONFIG_ERROR",
				err,
			)
		}
		return nil, temporal.NewApplicationError(
			"failed to get memory instance",
			"INSTANCE_CREATION_FAILED",
			err,
		)
	}
	activity.RecordHeartbeat(ctx, "Memory instance retrieved, delegating to instance flush method")
	// Cast to FlushableMemory to access the PerformFlush method
	flushable, ok := memInstance.(memory.FlushableMemory)
	if !ok {
		return nil, temporal.NewNonRetryableApplicationError(
			"memory instance does not support flushing",
			"NOT_FLUSHABLE",
			nil,
		)
	}
	// Delegate the actual flush logic to the memory instance
	// The instance has all the required components (store, lock manager, token manager, flush strategy)
	// and handles its own locking, message reading, and flush execution
	output, err := flushable.PerformFlush(ctx)
	if err != nil {
		log.Error("Memory flush operation failed", "Error", err)
		// Determine if the error is retryable using typed error checking
		var lockErr *memory.LockError
		if errors.As(err, &lockErr) {
			// Lock contention is retryable
			return nil, temporal.NewApplicationError(
				fmt.Sprintf("flush failed due to lock contention: %s", err.Error()),
				"LOCK_CONTENTION",
				err,
			)
		}
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("flush operation failed: %s", err.Error()),
			"FLUSH_FAILED",
			err,
		)
	}
	log.Info("FlushMemory activity completed successfully",
		"MemoryKey", input.MemoryInstanceKey,
		"MessagesKept", output.MessageCount,
		"TokensKept", output.TokenCount,
		"SummaryGenerated", output.SummaryGenerated,
	)
	return output, nil
}

// ClearFlushPendingFlag is a cleanup activity that clears the flush pending flag for a memory instance.
// This is used in workflow defer blocks to ensure the flag is cleared even if the main workflow fails.
func (ma *MemoryActivities) ClearFlushPendingFlag(
	ctx context.Context,
	input memory.ClearFlushPendingFlagInput,
) error {
	log := activity.GetLogger(ctx)
	log.Info("ClearFlushPendingFlag activity started", "MemoryKey", input.MemoryInstanceKey)
	// Use the real resource ID to get a proper memory instance
	memRef := core.MemoryReference{
		ID:  input.MemoryResourceID, // Use the real resource ID
		Key: input.MemoryInstanceKey,
	}
	workflowContext := map[string]any{
		"memory_instance_key": input.MemoryInstanceKey,
		"memory_resource_id":  input.MemoryResourceID,
		"project.id":          input.ProjectID,
	}
	// Try to get the memory instance to access its store
	memInstance, err := ma.MemoryManager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		// If we can't get the instance, this cleanup is likely not critical
		// Log the error but don't fail the cleanup activity
		log.Warn("Could not get memory instance for cleanup, flag will expire naturally", "Error", err)
		return nil
	}
	// Cast to FlushableMemory to access the MarkFlushPending method
	flushable, ok := memInstance.(memory.FlushableMemory)
	if !ok {
		// This is an application error - memory instance should implement FlushableMemory
		// if we're trying to clear a flush pending flag
		log.Error("Memory instance does not implement FlushableMemory interface",
			"MemoryKey", input.MemoryInstanceKey,
			"ResourceID", input.MemoryResourceID)
		return temporal.NewNonRetryableApplicationError(
			"memory instance does not support flush operations",
			"MEMORY_NOT_FLUSHABLE",
			fmt.Errorf("memory instance %s does not implement FlushableMemory", input.MemoryInstanceKey),
		)
	}
	if err := flushable.MarkFlushPending(ctx, false); err != nil {
		log.Warn("Failed to clear flush pending flag during cleanup", "Error", err)
		// Don't fail the activity, just log the warning
	} else {
		log.Info("Successfully cleared flush pending flag", "MemoryKey", input.MemoryInstanceKey)
	}
	return nil
}

// Helper function (should be in memory package or a common place)
// func MessagesWithTokensToLLMMessages(mwt []MessageWithTokens) []llm.Message { ... }
// For now, it's assumed to exist in the memory package.
// Add it to memory/types.go or a new memory/utils.go if it doesn't.
