package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/events"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type HandleTrigger struct {
	CMD           *pb.CmdWorkflowTrigger
	nc            *nats.Client
	repo          workflow.Repository
	projectConfig *project.Config
	workflows     []*workflow.Config
	publisher     core.EventPublisher
	subscriber    core.EventSubscriber
}

func NewHandleTrigger(
	nc *nats.Client,
	repo workflow.Repository,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) *HandleTrigger {
	publisher := nats.NewEventPublisher(nc)
	subscriber := nats.NewEventSubscriber(nc)
	return &HandleTrigger{
		CMD:           &pb.CmdWorkflowTrigger{},
		nc:            nc,
		repo:          repo,
		projectConfig: projectConfig,
		workflows:     workflows,
		publisher:     publisher,
		subscriber:    subscriber,
	}
}

func (h *HandleTrigger) Handler(ctx context.Context, msg jetstream.Msg) error {
	if err := h.Execute(ctx, msg); err != nil {
		metadata := h.CMD.GetMetadata()
		evt := events.NewEventFailed(h.nc, metadata, err)
		if err := evt.Publish(ctx); err != nil {
			return fmt.Errorf("failed to send EventWorkflowFailed: %w", err)
		}
		return err
	}
	return nil
}

func (h *HandleTrigger) Execute(ctx context.Context, msg jetstream.Msg) error {
	data := msg.Data()
	if err := proto.Unmarshal(data, h.CMD); err != nil {
		return fmt.Errorf("failed to unmarshal CmdWorkflowTrigger: %w", err)
	}
	cmd := h.CMD
	cmd.Metadata.Logger().Debug(fmt.Sprintf("Received: %T", cmd))

	// Create workflow execution
	metadata := cmd.GetMetadata()
	workflowID := metadata.WorkflowId
	details := cmd.GetDetails()
	input := core.NewInput(details.GetTriggerInput().AsMap())
	wConfig, err := workflow.FindConfig(h.workflows, workflowID)
	if err != nil {
		return err
	}
	_, err = h.repo.CreateExecution(ctx, metadata, wConfig, &input)
	if err != nil {
		return fmt.Errorf("failed to create workflow state: %w", err)
	}

	// Send CmdWorkflowExecute
	evt := events.NewCmdExecute(h.nc, h.CMD)
	if err := evt.Publish(ctx); err != nil {
		return fmt.Errorf("failed to send CmdWorkflowExecute: %w", err)
	}
	return nil
}
