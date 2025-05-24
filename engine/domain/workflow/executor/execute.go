package executor

import (
	"context"
	"fmt"

	tkevts "github.com/compozy/compozy/engine/domain/task/events"
	wfevts "github.com/compozy/compozy/engine/domain/workflow/events"
	wfuc "github.com/compozy/compozy/engine/domain/workflow/uc"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

func (e *Executor) subscribeExecute(ctx context.Context) error {
	comp := nats.ComponentWorkflow
	cmd := nats.CmdTypeExecute
	err := e.nc.SubscribeCmd(ctx, comp, cmd, e.handleExecute)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

func (e *Executor) handleExecute(_ string, data []byte, _ jetstream.Msg) error {
	// Unmarshal command from event
	var cmd pb.CmdWorkflowExecute
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdWorkflowExecute: %w", err)
	}
	logger.With("metadata", cmd.Metadata).Info("Received: WorkflowExecute")

	// Send WorkflowExecutionStart
	metadata := cmd.GetMetadata()
	if err := wfevts.SendStarted(e.nc, metadata); err != nil {
		return fmt.Errorf("failed to send EventWorkflowStarted: %w", err)
	}

	// Execute next task
	state, config, err := wfuc.FindStateAndConfig(e.stManager, e.workflows, metadata)
	if err != nil {
		return err
	}
	var taskID string
	if cmd.GetDetails().TaskId != nil {
		taskID = *cmd.GetDetails().TaskId
	} else {
		taskID = config.Tasks[0].ID
	}
	if err := tkevts.SendDispatch(e.nc, config, state, taskID); err != nil {
		return nil
	}
	return nil
}
