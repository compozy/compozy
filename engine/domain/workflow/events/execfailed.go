package wfevts

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

func SendExecutionFailed(nc *nats.Client, wf *pbcommon.WorkflowInfo, md *pbcommon.Metadata, err error) error {
	wfID := common.ID(wf.Id)
	corrID := common.ID(md.CorrelationId)
	execID := common.ID(wf.ExecId)
	logger.With(
		"workflow_id", wfID,
		"execution_id", execID,
		"correlation_id", corrID,
	).Debug("Sending WorkflowExecutionFailedEvent")

	code := "WORKFLOW_EXECUTION_FAILED"
	payload := pbwf.WorkflowExecutionFailedEvent{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         md.State,
		},
		Workflow: wf,
		Details: &pbwf.WorkflowExecutionFailedEvent_Details{
			Status: pbwf.WorkflowStatus_WORKFLOW_STATUS_FAILED,
			Error: &pbcommon.ErrorResult{
				Code:    &code,
				Message: err.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	data, err := proto.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("failed to marshal WorkflowExecutionFailedEvent: %w", err)
	}

	conn := nc.Conn()
	js, err := conn.JetStream()
	if err != nil {
		return fmt.Errorf("failed to get JetStream context: %w", err)
	}
	_, err = js.Publish(payload.ToSubject(), data)
	if err != nil {
		return fmt.Errorf("failed to publish WorkflowExecutionFailedEvent to JetStream: %w", err)
	}

	return nil
}
