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

func SendDispatch(
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
	cmd := &pb.CmdTaskDispatch{
		Metadata: &pb.TaskMetadata{
			Source:          pb.SourceTypeWorkflowExecutor.String(),
			CorrelationId:   corrID.String(),
			WorkflowId:      wConfig.ID,
			WorkflowExecId:  wStateID.ExecID.String(),
			WorkflowStateId: wStateID.String(),
			TaskId:          taskID,
			TaskExecId:      tExecID.String(),
			TaskStateId:     tStateID.String(),
			Time:            timepb.Now(),
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	if err := nc.PublishCmd(cmd); err != nil {
		return fmt.Errorf("failed to publish TaskDispatchCommand: %w", err)
	}

	logger.With("metadata", cmd.Metadata).Debug("Sent: TaskDispatch")
	return nil
}
