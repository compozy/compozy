package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

type TriggerResponse struct {
	StateID    string `json:"state_id"`
	WorkflowID string `json:"workflow_id"`
}

func SendTrigger(nc *nats.Client, input *common.Input, workflowID string) (*TriggerResponse, error) {
	corrID := common.MustNewID()
	wExecID := common.MustNewID()
	stateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)
	triggerInput, err := input.ToStruct()
	if err != nil {
		return nil, fmt.Errorf("failed to convert trigger to struct: %w", err)
	}

	cmd := &pb.CmdWorkflowTrigger{
		Metadata: &pb.WorkflowMetadata{
			Source:          pb.SourceTypeOrchestrator.String(),
			CorrelationId:   corrID.String(),
			WorkflowId:      workflowID,
			WorkflowExecId:  wExecID.String(),
			WorkflowStateId: stateID.String(),
			Time:            timepb.Now(),
		},
		Details: &pb.CmdWorkflowTrigger_Details{
			TriggerInput: triggerInput,
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return nil, fmt.Errorf("failed to publish CmdWorkflowTrigger: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: WorkflowTrigger")
	return &TriggerResponse{
		StateID:    stateID.String(),
		WorkflowID: workflowID,
	}, nil
}
