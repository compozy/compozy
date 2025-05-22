package wfevts

import (
	"fmt"

	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"google.golang.org/protobuf/proto"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

type TriggerResponse struct {
	StateID    string `json:"state_id"`
	WorkflowID string `json:"workflow_id"`
}

func SendTrigger(wfID string, nc *nats.Client, st *workflow.State) (*TriggerResponse, error) {
	stCtx := st.GetContext()
	corrID := stCtx.CorrID
	execID := stCtx.WorkflowExecID
	logger.With(
		"workflow_id", wfID,
		"execution_id", execID,
		"correlation_id", corrID,
	).Debug("Sending WorkflowTriggerCommand")

	tgInput, err := st.GetTrigger().ToStruct()
	if err != nil {
		return nil, fmt.Errorf("failed to convert trigger to struct: %w", err)
	}

	cmd := pbwf.WorkflowTriggerCommand{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State: &pbcommon.State{
				Id: st.GetID().String(),
			},
		},
		Workflow: &pbcommon.WorkflowInfo{
			Id:     wfID,
			ExecId: execID.String(),
		},
		Details: &pbwf.WorkflowTriggerCommand_Details{
			TriggerInput: tgInput,
		},
	}
	data, err := proto.Marshal(&cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal WorkflowTriggerCommand: %w", err)
	}

	conn := nc.Conn()
	js, err := conn.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get JetStream context: %w", err)
	}
	_, err = js.Publish(cmd.ToSubject(), data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish WorkflowTriggerCommand to JetStream: %w", err)
	}

	return &TriggerResponse{
		StateID:    st.GetID().String(),
		WorkflowID: wfID,
	}, nil
}
