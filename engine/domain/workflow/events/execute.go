package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendExecute(nc *nats.Client, triggerCmd *pb.CmdWorkflowTrigger) error {
	metadata := triggerCmd.GetMetadata()
	workflowID := common.ID(metadata.WorkflowId)
	corrID := common.ID(metadata.CorrelationId)
	wExecID := common.ID(metadata.WorkflowExecId)
	logger.With(
		"workflow_id", workflowID,
		"correlation_id", corrID,
		"workflow_execution_id", wExecID,
	).Debug("Sending CmdWorkflowExecute")

	cmd := pb.CmdWorkflowExecute{
		Metadata: &pb.WorkflowMetadata{
			Source:          "engine.Orchestrator",
			CorrelationId:   corrID.String(),
			WorkflowId:      metadata.WorkflowId,
			WorkflowExecId:  metadata.WorkflowExecId,
			WorkflowStateId: metadata.WorkflowStateId,
			Time:            timepb.Now(),
			Subject:         "",
		},
		Details: &pb.CmdWorkflowExecute_Details{
			TriggerInput: triggerCmd.GetDetails().GetTriggerInput(),
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish CmdWorkflowExecute: %w", err)
	}

	return nil
}
