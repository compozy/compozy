package events

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/state"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
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
	tStateID := state.NewID(nats.ComponentTask, corrID, tExecID)

	logger.With(
		"correlation_id", corrID,
		"workflow_id", wConfig.ID,
		"workflow_execution_id", wStateID.ExecID,
		"task_id", taskID,
		"task_execution_id", tExecID,
	).Debug("Sending TaskDispatchCommand")

	cmd := pb.CmdTaskDispatch{
		Metadata: &pb.TaskMetadata{
			Source:          "workflow.Executor",
			CorrelationId:   corrID.String(),
			WorkflowId:      wConfig.ID,
			WorkflowExecId:  wStateID.ExecID.String(),
			WorkflowStateId: wStateID.String(),
			TaskId:          taskID,
			TaskExecId:      tExecID.String(),
			TaskStateId:     tStateID.String(),
			Time:            timepb.Now(),
			Subject:         "",
		},
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish TaskDispatchCommand: %w", err)
	}

	return nil
}
