package events

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
)

func SendExecute(nc *nats.Client, dispatchCmd *pb.CmdTaskDispatch) error {
	metadata := dispatchCmd.GetMetadata()
	logger.With(
		"correlation_id", metadata.CorrelationId,
		"workflow_id", metadata.WorkflowId,
		"workflow_execution_id", metadata.WorkflowExecId,
		"task_id", metadata.TaskId,
		"task_execution_id", metadata.TaskExecId,
	).Debug("Sending TaskDispatchCommand")

	cmd := pb.CmdTaskDispatch{
		Metadata: metadata.Clone("engine.Orchestrator"),
	}
	if err := nc.PublishCmd(&cmd); err != nil {
		return fmt.Errorf("failed to publish CmdTaskDispatch: %w", err)
	}

	return nil
}
