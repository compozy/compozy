package temporal

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// -----------------------------------------------------------------------------
// Workflow Definition
// -----------------------------------------------------------------------------

type WorkflowInput struct {
	Metadata *pb.WorkflowMetadata
	Config   *wf.Config
	Input    core.Input
}

type TaskResult struct {
	Status core.StatusType
	Output map[string]interface{}
	Error  error
}

func CompozyWorkflow(ctx workflow.Context, input WorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting workflow", "workflow_id", input.Metadata.WorkflowExecId)

	// Set activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Execute tasks based on workflow config
	for _, taskSpec := range input.Config.Tasks {
		taskCmd := &pb.CmdTaskExecute{
			Metadata: &pb.TaskMetadata{
				WorkflowExecId: input.Metadata.WorkflowExecId,
				TaskExecId:     generateTaskExecID(taskSpec.ID),
				TaskId:         taskSpec.ID,
			},
		}

		// Execute the task activity
		err := workflow.ExecuteActivity(ctx, "TaskExecuteActivity", taskCmd).Get(ctx, nil)
		if err != nil {
			logger.Error("Task execution failed", "task_id", taskSpec.ID, "error", err)
			return fmt.Errorf("task %s failed: %w", taskSpec.ID, err)
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Signal Handlers
// -----------------------------------------------------------------------------

const (
	SignalPause  = "pause"
	SignalResume = "resume"
	SignalCancel = "cancel"
)

func RegisterSignalHandlers(ctx workflow.Context) {
	pauseChan := workflow.GetSignalChannel(ctx, SignalPause)
	resumeChan := workflow.GetSignalChannel(ctx, SignalResume)
	cancelChan := workflow.GetSignalChannel(ctx, SignalCancel)

	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			selector := workflow.NewSelector(ctx)

			selector.AddReceive(pauseChan, func(c workflow.ReceiveChannel, more bool) {
				var signal interface{}
				c.Receive(ctx, &signal)
				// Handle pause logic
			})

			selector.AddReceive(resumeChan, func(c workflow.ReceiveChannel, more bool) {
				var signal interface{}
				c.Receive(ctx, &signal)
				// Handle resume logic
			})

			selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, more bool) {
				var signal interface{}
				c.Receive(ctx, &signal)
				// Handle cancel logic
				workflow.GetLogger(ctx).Info("Workflow canceled via signal")
			})

			selector.Select(ctx)
		}
	})
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func generateTaskExecID(taskID string) string {
	return fmt.Sprintf("%s-%d", taskID, time.Now().UnixNano())
}
