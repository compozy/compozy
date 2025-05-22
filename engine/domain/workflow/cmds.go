package workflow

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"google.golang.org/protobuf/proto"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

// -----------------------------------------------------------------------------
// Trigger Workflow
// -----------------------------------------------------------------------------

type TriggerWorkflowResponse struct {
	CorrID     common.ID `json:"correlation_id"`
	WorkflowID common.ID `json:"workflow_id"`
	ExecID     common.ID `json:"execution_id"`
}

func SendTrigger(wfID common.ID, natsClient *nats.Client, st *State) (*TriggerWorkflowResponse, error) {
	ex := st.Exec()
	corrID := ex.CorrID
	execID := ex.WorkflowExecID
	logger.With(
		"workflow_id", wfID,
		"execution_id", execID,
		"correlation_id", corrID,
	).Debug("Sending WorkflowTriggerCommand")

	tgInput, err := st.GetTrigger().ToStruct()
	if err != nil {
		return nil, fmt.Errorf("failed to convert trigger to struct: %w", err)
	}

	payload := pbwf.WorkflowTriggerCommand{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State: &pbcommon.State{
				Id: st.GetID().String(),
			},
		},
		Workflow: &pbcommon.WorkflowInfo{
			Id:     wfID.String(),
			ExecId: execID.String(),
		},
		Details: &pbwf.WorkflowTriggerCommand_Details{
			TriggerInput: tgInput,
		},
	}
	data, err := proto.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal WorkflowTriggerCommand: %w", err)
	}

	nc := natsClient.Conn()
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get JetStream context: %w", err)
	}
	_, err = js.Publish(payload.ToSubject(), data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish WorkflowTriggerCommand to JetStream: %w", err)
	}

	return &TriggerWorkflowResponse{
		CorrID:     corrID,
		WorkflowID: wfID,
		ExecID:     execID,
	}, nil
}

// -----------------------------------------------------------------------------
// Execute Workflow
// -----------------------------------------------------------------------------

func SendExecute(natsClient *nats.Client, cmd *pbwf.WorkflowTriggerCommand) error {
	wfID := common.ID(cmd.GetWorkflow().Id)
	corrID := common.ID(cmd.GetMetadata().CorrelationId)
	execID := common.ID(cmd.GetWorkflow().ExecId)
	logger.With(
		"workflow_id", wfID,
		"execution_id", execID,
		"correlation_id", corrID,
	).Debug("Sending WorkflowExecuteCommand")

	payload := pbwf.WorkflowExecuteCommand{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         cmd.GetMetadata().State,
		},
		Workflow: cmd.GetWorkflow(),
		Details: &pbwf.WorkflowExecuteCommand_Details{
			TriggerInput: cmd.GetDetails().GetTriggerInput(),
		},
	}
	data, err := proto.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("failed to marshal WorkflowExecuteCommand: %w", err)
	}

	nc := natsClient.Conn()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get JetStream context: %w", err)
	}
	_, err = js.Publish(payload.ToSubject(), data)
	if err != nil {
		return fmt.Errorf("failed to publish WorkflowExecuteCommand to JetStream: %w", err)
	}

	return nil
}
