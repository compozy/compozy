package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
	"google.golang.org/protobuf/types/known/structpb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendFailed(
	nc *nats.Client,
	workflowInfo *pbcommon.WorkflowInfo,
	taskInfo *pbcommon.TaskInfo,
	metadata *pbcommon.Metadata,
	err error,
) error {
	workflowID := common.ID(workflowInfo.Id)
	corrID := common.ID(metadata.CorrelationId)
	wExecID := common.ID(workflowInfo.ExecId)
	taskID := common.ID(taskInfo.Id)
	tExecID := common.ID(taskInfo.ExecId)
	logger.With(
		"correlation_id", corrID,
		"workflow_id", workflowID,
		"workflow_execution_id", wExecID,
		"task_id", taskID,
		"task_execution_id", tExecID,
	).Debug("Sending EventTaskFailed")

	code := "TASK_EXECUTION_FAILED"
	cmd := pbtask.EventTaskFailed{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         metadata.State,
		},
		Workflow: workflowInfo,
		Task:     taskInfo,
		Details: &pbtask.EventTaskFailed_Details{
			Status: pbtask.TaskStatus_TASK_STATUS_FAILED,
			Error: &pbcommon.ErrorResult{
				Code:    &code,
				Message: err.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventTaskFailed: %w", err)
	}

	return nil
}
