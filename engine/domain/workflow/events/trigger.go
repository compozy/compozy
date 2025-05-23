package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

type TriggerResponse struct {
	StateID    string `json:"state_id"`
	WorkflowID string `json:"workflow_id"`
}

func SendTrigger(nc *nats.Client, input *common.Input, workflowID string) (*TriggerResponse, error) {
	corrID, err := common.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate correlation ID: %w", err)
	}
	wExecID, err := common.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate workflow execution ID: %w", err)
	}
	stateID := state.NewID(nats.ComponentWorkflow, corrID, wExecID)

	logger.With(
		"correlation_id", corrID,
		"workflow_id", workflowID,
		"workflow_execution_id", wExecID,
	).Debug("Sending CmdWorkflowTrigger")

	triggerInput, err := input.ToStruct()
	if err != nil {
		return nil, fmt.Errorf("failed to convert trigger to struct: %w", err)
	}

	cmd := pbwf.CmdWorkflowTrigger{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State: &pbcommon.State{
				Id: stateID.String(),
			},
		},
		Workflow: &pbcommon.WorkflowInfo{
			Id:     workflowID,
			ExecId: wExecID.String(),
		},
		Details: &pbwf.CmdWorkflowTrigger_Details{
			TriggerInput: triggerInput,
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return nil, fmt.Errorf("failed to publish CmdWorkflowTrigger: %w", err)
	}

	return &TriggerResponse{
		StateID:    stateID.String(),
		WorkflowID: workflowID,
	}, nil
}
