package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// MemoryActivities encapsulates Temporal activities related to memory management.
type MemoryActivities struct {
	// MemoryManager is the factory that creates properly configured memory instances
	// based on the MemoryResourceID provided in activity input.
	MemoryManager memcore.ManagerInterface
	// Logger for activity-specific logging
	Logger logger.Logger
}

// NewMemoryActivities creates a new instance of MemoryActivities.
func NewMemoryActivities(
	manager memcore.ManagerInterface,
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
	input memcore.FlushMemoryActivityInput,
) (*memcore.FlushMemoryActivityOutput, error) {
	log := activity.GetLogger(ctx)
	log.Info("FlushMemory activity started", "MemoryKey", input.MemoryInstanceKey, "ResourceID", input.MemoryResourceID)
	memInstance, err := ma.getMemoryInstance(ctx, input)
	if err != nil {
		return nil, err
	}
	flushable, err := ma.validateFlushable(memInstance)
	if err != nil {
		return nil, err
	}
	activity.RecordHeartbeat(ctx, "Memory instance retrieved, delegating to instance flush method")
	output, err := ma.performFlushOperation(ctx, flushable)
	if err != nil {
		return nil, err
	}
	ma.logFlushSuccess(log, input, output)
	return output, nil
}

// getMemoryInstance retrieves the memory instance from the manager
func (ma *MemoryActivities) getMemoryInstance(
	ctx context.Context,
	input memcore.FlushMemoryActivityInput,
) (memcore.Memory, error) {
	log := activity.GetLogger(ctx)
	memRef := core.MemoryReference{
		ID:  input.MemoryResourceID,
		Key: input.MemoryInstanceKey,
	}
	workflowContext := map[string]any{
		"memory_instance_key": input.MemoryInstanceKey,
		"memory_resource_id":  input.MemoryResourceID,
		"project.id":          input.ProjectID,
	}
	memInstance, err := ma.MemoryManager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		log.Error("Failed to get memory instance", "Error", err)
		return nil, ma.handleInstanceError(err)
	}
	if memInstance == nil {
		return nil, temporal.NewNonRetryableApplicationError(
			"memory instance is nil",
			"INSTANCE_NOT_FOUND",
			nil,
		)
	}
	return memInstance, nil
}

// handleInstanceError handles errors from GetInstance with proper retry logic
func (ma *MemoryActivities) handleInstanceError(err error) error {
	var configErr *memcore.ConfigError
	if errors.As(err, &configErr) {
		return temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("memory configuration error: %s", err.Error()),
			"MEMORY_CONFIG_ERROR",
			err,
		)
	}
	return temporal.NewApplicationError(
		"failed to get memory instance",
		"INSTANCE_CREATION_FAILED",
		err,
	)
}

// validateFlushable validates that the instance supports flushing
func (ma *MemoryActivities) validateFlushable(memInstance memcore.Memory) (memcore.FlushableMemory, error) {
	flushable, ok := memInstance.(memcore.FlushableMemory)
	if !ok {
		return nil, temporal.NewNonRetryableApplicationError(
			"memory instance does not support flushing",
			"NOT_FLUSHABLE",
			nil,
		)
	}
	return flushable, nil
}

// performFlushOperation executes the flush and handles errors
func (ma *MemoryActivities) performFlushOperation(
	ctx context.Context,
	flushable memcore.FlushableMemory,
) (*memcore.FlushMemoryActivityOutput, error) {
	log := activity.GetLogger(ctx)
	output, err := flushable.PerformFlush(ctx)
	if err != nil {
		log.Error("Memory flush operation failed", "Error", err)
		return nil, ma.handleFlushError(err)
	}
	return output, nil
}

// handleFlushError handles flush operation errors with proper retry logic
func (ma *MemoryActivities) handleFlushError(err error) error {
	var lockErr *memcore.LockError
	if errors.As(err, &lockErr) {
		return temporal.NewApplicationError(
			fmt.Sprintf("flush failed due to lock contention: %s", err.Error()),
			"LOCK_CONTENTION",
			err,
		)
	}
	return temporal.NewNonRetryableApplicationError(
		fmt.Sprintf("flush operation failed: %s", err.Error()),
		"FLUSH_FAILED",
		err,
	)
}

// logFlushSuccess logs successful flush completion
func (ma *MemoryActivities) logFlushSuccess(
	log interface{ Info(string, ...any) },
	input memcore.FlushMemoryActivityInput,
	output *memcore.FlushMemoryActivityOutput,
) {
	log.Info("FlushMemory activity completed successfully",
		"MemoryKey", input.MemoryInstanceKey,
		"MessagesKept", output.MessageCount,
		"TokensKept", output.TokenCount,
		"SummaryGenerated", output.SummaryGenerated,
	)
}

// ClearFlushPendingFlag is a cleanup activity that clears the flush pending flag for a memory instance.
// This is used in workflow defer blocks to ensure the flag is cleared even if the main workflow fails.
func (ma *MemoryActivities) ClearFlushPendingFlag(
	ctx context.Context,
	input memcore.ClearFlushPendingFlagInput,
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
	flushable, ok := memInstance.(memcore.FlushableMemory)
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
