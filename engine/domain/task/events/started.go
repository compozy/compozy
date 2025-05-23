package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendStarted(
	nc *nats.Client,
	workflowInfo *pbcommon.WorkflowInfo,
	taskInfo *pbcommon.TaskInfo,
	metadata *pbcommon.Metadata,
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
	).Debug("Sending EventTaskStarted")

	cmd := pbtask.EventTaskStarted{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         metadata.State,
		},
		Workflow: workflowInfo,
		Task:     taskInfo,
		Details: &pbtask.EventTaskStarted_Details{
			Status: pbtask.TaskStatus_TASK_STATUS_RUNNING,
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish EventTaskStarted: %w", err)
	}

	return nil
}
