package wfevts

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

func SendExecutionStarted(nc *nats.Client, wf *pbcommon.WorkflowInfo, md *pbcommon.Metadata) error {
	wfID := common.ID(wf.Id)
	corrID := common.ID(md.CorrelationId)
	execID := common.ID(wf.ExecId)
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
			State:         md.State,
		},
		Workflow: wf,
		Details: &pbwf.WorkflowExecutionStartedEvent_Details{
			Status: pbwf.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		},
	}
	data, err := proto.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("failed to marshal WorkflowExecutionStartedEvent: %w", err)
	}

	conn := nc.Conn()
	js, err := conn.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get JetStream context: %w", err)
	}
	_, err = js.Publish(payload.ToSubject(), data)
	if err != nil {
		return fmt.Errorf("failed to publish WorkflowExecutionStartedEvent to JetStream: %w", err)
	}

	return nil
}
