package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"google.golang.org/protobuf/types/known/structpb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendFailed(nc *nats.Client, info *pbcommon.WorkflowInfo, metadata *pbcommon.Metadata, err error) error {
	workflowID := common.ID(info.Id)
	corrID := common.ID(metadata.CorrelationId)
	wExecID := common.ID(info.ExecId)
	logger.With(
		"workflow_id", workflowID,
		"correlation_id", corrID,
		"workflow_execution_id", wExecID,
	).Debug("Sending EventWorkflowFailed")

	code := "WORKFLOW_EXECUTION_FAILED"
	cmd := pbwf.EventWorkflowFailed{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         metadata.State,
		},
		Workflow: info,
		Details: &pbwf.EventWorkflowFailed_Details{
			Status: pbwf.WorkflowStatus_WORKFLOW_STATUS_FAILED,
			Error: &pbcommon.ErrorResult{
				Code:    &code,
				Message: err.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventWorkflowFailed: %w", err)
	}

	return nil
}
