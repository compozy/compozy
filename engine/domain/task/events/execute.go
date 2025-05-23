package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendExecute(nc *nats.Client, dispatchCmd *pbtask.CmdTaskDispatch) error {
	info, err := task.InfoFromEvent(dispatchCmd)
	if err != nil {
		return fmt.Errorf("failed to parse task payload info: %w", err)
	}
	logger.With(
		"correlation_id", info.CorrID,
		"workflow_id", info.WorkflowID,
		"workflow_execution_id", info.WorkflowExecID,
		"task_id", info.TaskID,
		"task_execution_id", info.TaskExecID,
	).Debug("Sending TaskDispatchCommand")

	cmd := pbtask.CmdTaskDispatch{
		Metadata: &pbcommon.Metadata{
			CorrelationId: info.Metadata().CorrelationId,
			Source:        "engine.Orchestrator",
			Time:          timepb.Now(),
			State:         info.Metadata().State,
		},
		Workflow: info.Workflow(),
		Task:     info.Task(),
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish CmdTaskDispatch: %w", err)
	}

	return nil
}
