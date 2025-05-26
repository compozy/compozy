package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/engine/task"
	tkevts "github.com/compozy/compozy/engine/task/events"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type HandleDispatch struct {
	CMD       *pb.CmdTaskDispatch
	nc        *nats.Client
	repo      task.Repository
	workflows []*workflow.Config
	publisher core.EventPublisher
}

func NewHandleDispatch(
	nc *nats.Client,
	repo task.Repository,
	workflows []*workflow.Config,
) *HandleDispatch {
	publisher := nats.NewEventPublisher(nc)
	return &HandleDispatch{
		CMD:       &pb.CmdTaskDispatch{},
		nc:        nc,
		repo:      repo,
		publisher: publisher,
		workflows: workflows,
	}
}

func (h *HandleDispatch) Handler(ctx context.Context, msg jetstream.Msg) error {
	if err := h.Execute(ctx, msg); err != nil {
		metadata := h.CMD.GetMetadata()
		evt := tkevts.NewEventFailed(h.nc, metadata, err)
		if err := evt.Publish(ctx); err != nil {
			return fmt.Errorf("failed to send EventTaskFailed: %w", err)
		}
		return err
	}
	return nil
}

func (h *HandleDispatch) Execute(ctx context.Context, msg jetstream.Msg) error {
	data := msg.Data()
	if err := proto.Unmarshal(data, h.CMD); err != nil {
		return fmt.Errorf("failed to unmarshal CmdTaskDispatch: %w", err)
	}
	h.CMD.Metadata.Logger().Debug(fmt.Sprintf("Received: %T", h.CMD))

	// Find workflow execution and config
	metadata := h.CMD.GetMetadata()
	workflowID := metadata.WorkflowId
	wConfig, err := workflow.FindConfig(h.workflows, workflowID)
	if err != nil {
		return fmt.Errorf("failed to find workflow config: %w", err)
	}

	// Find task config and create execution
	taskID := metadata.TaskId
	tConfig, err := task.FindConfig(wConfig.Tasks, taskID)
	if err != nil {
		return fmt.Errorf("failed to find task config: %w", err)
	}
	_, err = h.repo.CreateExecution(ctx, metadata, tConfig)
	if err != nil {
		return fmt.Errorf("failed to create task execution: %w", err)
	}

	// Send CmdTaskExecute
	evt := tkevts.NewCmdExecute(h.nc, h.CMD)
	if err := evt.Publish(ctx); err != nil {
		return fmt.Errorf("failed to send CmdTaskExecute: %w", err)
	}
	return nil
}
