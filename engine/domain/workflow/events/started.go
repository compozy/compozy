package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendStarted(nc *nats.Client, metadata *pb.WorkflowMetadata) error {
	workflowID := common.ID(metadata.WorkflowId)
	corrID := common.ID(metadata.CorrelationId)
	wExecID := common.ID(metadata.WorkflowExecId)
	logger.With(
		"workflow_id", workflowID,
		"correlation_id", corrID,
		"workflow_execution_id", wExecID,
	).Debug("Sending EventWorkflowStarted")

	cmd := pb.EventWorkflowStarted{
		Metadata: &pb.WorkflowMetadata{
			Source:          "engine.Orchestrator",
			CorrelationId:   metadata.CorrelationId,
			WorkflowId:      metadata.WorkflowId,
			WorkflowExecId:  metadata.WorkflowExecId,
			WorkflowStateId: metadata.WorkflowStateId,
			Time:            timepb.Now(),
			Subject:         "",
		},
		Details: &pb.EventWorkflowStarted_Details{
			Status: pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventWorkflowStarted: %w", err)
	}

	return nil
}
