package memory

import (
	"time"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// FlushMemoryWorkflow orchestrates the memory flushing process.
// It executes the FlushMemory activity with appropriate retry policies and timeouts.
func FlushMemoryWorkflow(
	ctx workflow.Context,
	input memcore.FlushMemoryActivityInput,
) (*memcore.FlushMemoryActivityOutput, error) {
	// Configure activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Allow enough time for large memory flushes
		HeartbeatTimeout:    30 * time.Second, // Activity should heartbeat every 30s
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
			NonRetryableErrorTypes: []string{
				"ACTIVITY_DEPENDENCY_NIL", // Configuration errors should not retry
				"MEMORY_CONFIG_ERROR",
				"NOT_FLUSHABLE",
			},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Track workflow error for cleanup purposes
	var workflowErr error

	// Defer cleanup of flush pending flag if workflow fails
	defer func() {
		if workflowErr != nil {
			// Use disconnected context to ensure cleanup runs even if workflow is canceled
			dCtx, _ := workflow.NewDisconnectedContext(ctx)
			dCtx = workflow.WithActivityOptions(dCtx, workflow.ActivityOptions{
				StartToCloseTimeout: 1 * time.Minute,
			})
			// Clear the flush pending flag using the cleanup activity
			cleanupInput := memcore.ClearFlushPendingFlagInput{
				MemoryInstanceKey: input.MemoryInstanceKey,
				MemoryResourceID:  input.MemoryResourceID,
				ProjectID:         input.ProjectID,
			}
			if err := workflow.ExecuteActivity(dCtx, "ClearFlushPendingFlag", cleanupInput).Get(dCtx, nil); err != nil {
				// Log the error but don't fail the cleanup
				workflow.GetLogger(ctx).Error("Failed to execute cleanup activity", "error", err)
			}
		}
	}()

	// Execute the flush activity
	var result memcore.FlushMemoryActivityOutput
	workflowErr = workflow.ExecuteActivity(ctx, "FlushMemory", input).Get(ctx, &result)
	if workflowErr != nil {
		return nil, workflowErr
	}

	// Log workflow completion
	workflow.GetLogger(ctx).Info("FlushMemoryWorkflow completed",
		"memory_key", input.MemoryInstanceKey,
		"messages_kept", result.MessageCount,
		"tokens_kept", result.TokenCount,
		"success", result.Success,
	)

	return &result, nil
}
