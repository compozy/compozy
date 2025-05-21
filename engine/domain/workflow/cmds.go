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

func TriggerWorkflow(wfID common.CompID, natsClient *nats.Client, ex *Execution) error {
	corrID := ex.CorrID
	execID := ex.ExecID
	logger.With(
		"workflow_id", wfID,
		"execution_id", execID,
		"correlation_id", corrID,
	).Info("Sending WorkflowTriggerCommand")

	// Create context from input
	payloadCtx, err := structpb.NewStruct(map[string]any{
		"execution": ex,
	})
	if err != nil {
		return fmt.Errorf("failed to create payload context: %w", err)
	}

	// Create the payload
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
			Context: payloadCtx,
		},
	}

	// Publish the command to NATS JetStream for durable consumption
	nc := natsClient.Conn()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get JetStream context: %w", err)
	}

	data, err := proto.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow trigger command: %w", err)
	}

	// Publish the message and wait for an acknowledgement from the JetStream server.
	// This ensures the message is persisted in the stream.
	pubAck, err := js.Publish(payload.ToSubject(), data)
	if err != nil {
		return fmt.Errorf("failed to publish workflow trigger command to JetStream: %w", err)
	}

	logger.With(
		"stream", pubAck.Stream,
		"sequence", pubAck.Sequence,
	).Info("WorkflowTriggerCommand durably published to JetStream")

	return nil
}
