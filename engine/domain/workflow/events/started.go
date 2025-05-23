package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendStarted(nc *nats.Client, info *pbcommon.WorkflowInfo, metadata *pbcommon.Metadata) error {
	workflowID := common.ID(info.Id)
	corrID := common.ID(metadata.CorrelationId)
	wExecID := common.ID(info.ExecId)
	logger.With(
		"workflow_id", workflowID,
		"correlation_id", corrID,
		"workflow_execution_id", wExecID,
	).Debug("Sending EventWorkflowStarted")

	cmd := pbwf.EventWorkflowStarted{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         metadata.State,
		},
		Workflow: info,
		Details: &pbwf.EventWorkflowStarted_Details{
			Status: pbwf.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventWorkflowStarted: %w", err)
	}

	return nil
}
