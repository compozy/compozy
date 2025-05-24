package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
)

func SendStarted(nc *nats.Client, metadata *pb.TaskMetadata) error {
	source := pb.SourceTypeTaskExecutor
	metadata, err := metadata.Clone(source)
	if err != nil {
		return fmt.Errorf("failed to clone metadata: %w", err)
	}
	cmd := &pb.EventTaskStarted{
		Metadata: metadata,
		Details: &pb.EventTaskStarted_Details{
			Status: pb.TaskStatus_TASK_STATUS_RUNNING,
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return fmt.Errorf("failed to publish EventTaskStarted: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: TaskStarted")
	return nil
}
