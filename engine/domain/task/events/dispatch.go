package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbcommon "github.com/compozy/compozy/pkg/pb/common"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

func SendTaskDispatch(
	nc *nats.Client,
	wConfig *workflow.Config,
	wState *workflow.State,
	taskID string,
) error {
	corrID := wState.GetID().CorrID
	tExecID, err := common.NewID()
	if err != nil {
		return fmt.Errorf("failed to generate task execution ID: %w", err)
	}
	wStateID := wState.GetID()
	wStateIDStr := wStateID.String()
	tStateID := state.NewID(nats.ComponentTask, corrID, tExecID)

	logger.With(
		"correlation_id", corrID,
		"workflow_id", wConfig.ID,
		"workflow_execution_id", wStateID.ExecID,
		"task_id", taskID,
		"task_execution_id", tExecID,
	).Debug("Sending TaskDispatchCommand")

	cmd := pbtask.CmdTaskDispatch{
		Metadata: &pbcommon.Metadata{
			CorrelationId: corrID.String(),
			Source:        "workflow.Executor",
			Time:          timepb.Now(),
			State: &pbcommon.State{
				Id:       tStateID.String(),
				ParentId: &wStateIDStr,
			},
		},
		Workflow: &pbcommon.WorkflowInfo{
			Id:     wConfig.ID,
			ExecId: wStateID.ExecID.String(),
		},
		Task: &pbcommon.TaskInfo{
			Id:     taskID,
			ExecId: tExecID.String(),
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish TaskDispatchCommand: %w", err)
	}

	return nil
}
