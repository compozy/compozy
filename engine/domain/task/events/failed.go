package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/structpb"
)

func SendFailed(nc *nats.Client, metadata *pb.TaskMetadata, errData error) error {
	code := "TASK_EXECUTION_FAILED"
	source := pb.SourceTypeTaskExecutor
	metadata, err := metadata.Clone(source)
	if err != nil {
		return fmt.Errorf("failed to clone metadata: %w", err)
	}
	cmd := &pb.EventTaskFailed{
		Metadata: metadata,
		Details: &pb.EventTaskFailed_Details{
			Status: pb.TaskStatus_TASK_STATUS_FAILED,
			Error: &pb.ErrorResult{
				Code:    &code,
				Message: errData.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return fmt.Errorf("failed to publish EventTaskFailed: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: TaskFailed")
	return nil
}
