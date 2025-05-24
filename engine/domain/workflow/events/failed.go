package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/structpb"
)

func SendFailed(nc *nats.Client, metadata *pb.WorkflowMetadata, errData error) error {
	code := "WORKFLOW_EXECUTION_FAILED"
	source := pb.SourceTypeWorkflowExecutor
	metadata, err := metadata.Clone(source)
	if err != nil {
		return fmt.Errorf("failed to clone metadata: %w", err)
	}
	cmd := &pb.EventWorkflowFailed{
		Metadata: metadata,
		Details: &pb.EventWorkflowFailed_Details{
			Status: pb.WorkflowStatus_WORKFLOW_STATUS_FAILED,
			Error: &pb.ErrorResult{
				Code:    &code,
				Message: errData.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return fmt.Errorf("failed to publish EventWorkflowFailed: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: WorkflowFailed")
	return nil
}
