package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
)

func SendStarted(nc *nats.Client, metadata *pb.WorkflowMetadata) error {
	source := pb.SourceTypeWorkflowExecutor
	metadata, err := metadata.Clone(source)
	if err != nil {
		return fmt.Errorf("failed to clone metadata: %w", err)
	}
	cmd := &pb.EventWorkflowStarted{
		Metadata: metadata,
		Details: &pb.EventWorkflowStarted_Details{
			Status: pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return fmt.Errorf("failed to publish EventWorkflowStarted: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: WorkflowStarted")
	return nil
}
