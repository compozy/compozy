package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
)

func SendStarted(nc *nats.Client, metadata *pb.TaskMetadata) error {
	logger.With(
		"correlation_id", metadata.CorrelationId,
		"workflow_id", metadata.WorkflowId,
		"workflow_execution_id", metadata.WorkflowExecId,
		"task_id", metadata.TaskId,
		"task_execution_id", metadata.TaskExecId,
	).Debug("Sending EventTaskStarted")

	cmd := pb.EventTaskStarted{
		Metadata: metadata.Clone("task.Executor"),
		Details: &pb.EventTaskStarted_Details{
			Status: pb.TaskStatus_TASK_STATUS_RUNNING,
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventTaskStarted: %w", err)
	}

	return nil
}
