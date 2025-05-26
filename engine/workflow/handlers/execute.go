package handlers

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	tkevts "github.com/compozy/compozy/engine/task/events"
	"github.com/compozy/compozy/engine/workflow"
	wfevts "github.com/compozy/compozy/engine/workflow/events"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type ExecuteHandler struct {
	cmd       *pb.CmdWorkflowExecute
	nc        *nats.Client
	repo      workflow.Repository
	workflows []*workflow.Config
	publisher core.EventPublisher
}

func NewExecuteHandler(
	nc *nats.Client,
	repo workflow.Repository,
	workflows []*workflow.Config,
) *ExecuteHandler {
	publisher := nats.NewEventPublisher(nc)
	return &ExecuteHandler{
		cmd:       &pb.CmdWorkflowExecute{},
		nc:        nc,
		repo:      repo,
		workflows: workflows,
		publisher: publisher,
	}
}

func (h *ExecuteHandler) Subjects(workflowExecID string, execID string) []string {
	return []string{
		h.cmd.ToSubjectParams(workflowExecID, execID),
	}
}

func (h *ExecuteHandler) Handle(ctx context.Context, msg jetstream.Msg) error {
	data := msg.Data()
	if err := proto.Unmarshal(data, h.cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdWorkflowExecute: %w", err)
	}
	h.cmd.Metadata.Logger().Debug(fmt.Sprintf("Received: %T", h.cmd))

	// Send EventWorkflowStarted
	metadata := h.cmd.GetMetadata()
	evt := wfevts.NewEventStarted(h.nc, metadata)
	if err := evt.Publish(ctx); err != nil {
		return fmt.Errorf("failed to send EventWorkflowStarted: %w", err)
	}

	// Execute next task
	workflowID := metadata.GetWorkflowId()
	config, err := workflow.FindConfig(h.workflows, workflowID)
	if err != nil {
		return err
	}
	// Get task ID from command or use first task ID from config
	var taskID string
	if h.cmd.GetDetails().TaskId != nil {
		taskID = *h.cmd.GetDetails().TaskId
	} else {
		taskID = config.Tasks[0].ID
	}

	// Dispatch task command
	dispatchCmd, err := tkevts.NewCmdDispatch(h.nc, h.cmd.Metadata, taskID)
	if err != nil {
		return fmt.Errorf("failed to create CmdTaskDispatch: %w", err)
	}
	if err := dispatchCmd.Publish(ctx); err != nil {
		return nil
	}

	return nil
}
