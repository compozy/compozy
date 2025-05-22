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
// Execution Started
// -----------------------------------------------------------------------------

func SendExecutionStarted(natsClient *nats.Client, cmd *pbwf.WorkflowTriggerCommand) error {
	wfID := common.ID(cmd.GetWorkflow().Id)
	corrID := common.ID(cmd.GetMetadata().CorrelationId)
	execID := common.ID(cmd.GetWorkflow().ExecId)
	logger.With(
		"workflow_id", wfID,
		"execution_id", execID,
		"correlation_id", corrID,
	).Debug("Sending WorkflowExecutionStartedEvent")

	payload := pbwf.WorkflowExecutionStartedEvent{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         cmd.GetMetadata().State,
		},
		Workflow: cmd.GetWorkflow(),
		Details: &pbwf.WorkflowExecutionStartedEvent_Details{
			Status: pbwf.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		},
	}
	data, err := proto.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("failed to marshal WorkflowExecutionStartedEvent: %w", err)
	}

	nc := natsClient.Conn()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get JetStream context: %w", err)
	}
	_, err = js.Publish(payload.ToSubject(), data)
	if err != nil {
		return fmt.Errorf("failed to publish WorkflowExecutionStartedEvent to JetStream: %w", err)
	}

	return nil
}
