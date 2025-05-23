package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/structpb"
)

func SendFailed(nc *nats.Client, metadata *pb.TaskMetadata, err error) error {
	logger.With(
		"correlation_id", metadata.CorrelationId,
		"workflow_id", metadata.WorkflowId,
		"workflow_execution_id", metadata.WorkflowExecId,
		"task_id", metadata.TaskId,
		"task_execution_id", metadata.TaskExecId,
	).Debug("Sending EventTaskFailed")

	code := "TASK_EXECUTION_FAILED"
	cmd := pb.EventTaskFailed{
		Metadata: metadata.Clone("task.Executor"),
		Details: &pb.EventTaskFailed_Details{
			Status: pb.TaskStatus_TASK_STATUS_FAILED,
			Error: &pb.ErrorResult{
				Code:    &code,
				Message: err.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventTaskFailed: %w", err)
	}

	return nil
}
