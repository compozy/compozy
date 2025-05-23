package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/structpb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendFailed(nc *nats.Client, metadata *pb.WorkflowMetadata, err error) error {
	workflowID := common.ID(metadata.WorkflowId)
	corrID := common.ID(metadata.CorrelationId)
	wExecID := common.ID(metadata.WorkflowExecId)
	logger.With(
		"workflow_id", workflowID,
		"correlation_id", corrID,
		"workflow_execution_id", wExecID,
	).Debug("Sending EventWorkflowFailed")

	code := "WORKFLOW_EXECUTION_FAILED"
	cmd := pb.EventWorkflowFailed{
		Metadata: &pb.WorkflowMetadata{
			Source:          "engine.Orchestrator",
			CorrelationId:   metadata.CorrelationId,
			WorkflowId:      metadata.WorkflowId,
			WorkflowExecId:  metadata.WorkflowExecId,
			WorkflowStateId: metadata.WorkflowStateId,
			Time:            timepb.Now(),
			Subject:         "",
		},
		Details: &pb.EventWorkflowFailed_Details{
			Status: pb.WorkflowStatus_WORKFLOW_STATUS_FAILED,
			Error: &pb.ErrorResult{
				Code:    &code,
				Message: err.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventWorkflowFailed: %w", err)
	}

	return nil
}
