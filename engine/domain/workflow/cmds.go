package workflow

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

type TriggerWorkflowResponse struct {
	CorrID     common.CorrID `json:"correlation_id"`
	WorkflowID common.CompID `json:"workflow_id"`
	ExecID     common.ExecID `json:"execution_id"`
}

func TriggerWorkflow(wfID common.CompID, natsClient *nats.Client, ex *Execution) (*TriggerWorkflowResponse, error) {
	corrID := ex.CorrID
	execID := ex.WorkflowExecID
	logger.With(
		"workflow_id", wfID,
		"execution_id", execID,
		"correlation_id", corrID,
	).Info("Sending WorkflowTriggerCommand")

	exMap, err := ex.ToProtoBufMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert execution to protobuf map: %w", err)
	}
	ctx, err := structpb.NewStruct(exMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload context: %w", err)
	}

	payload := pbwf.WorkflowTriggerCommand{
		Metadata: &pbcommon.Metadata{
			CorrelationId:   corrID.String(),
			SourceComponent: "engine.Orchestrator",
			CreatedAt:       timepb.Now(),
		},
		Workflow: &pbcommon.WorkflowInfo{
			Id:     wfID.String(),
			ExecId: execID.String(),
		},
		Payload: &pbwf.WorkflowTriggerCommand_Payload{
			Context: ctx,
		},
	}
	data, err := proto.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow trigger command: %w", err)
	}

	nc := natsClient.Conn()
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to get JetStream context: %w", err)
	}
	_, err = js.Publish(payload.ToSubject(), data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish workflow trigger command to JetStream: %w", err)
	}

	return &TriggerWorkflowResponse{
		CorrID:     corrID,
		WorkflowID: wfID,
		ExecID:     execID,
	}, nil
}
