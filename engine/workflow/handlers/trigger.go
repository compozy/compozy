package handlers

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

type TriggerHandler struct {
	cmd           *pb.CmdWorkflowTrigger
	nc            *nats.Client
	repo          workflow.Repository
	projectConfig *project.Config
	workflows     []*workflow.Config
	publisher     core.EventPublisher
}

func NewTriggerHandler(
	nc *nats.Client,
	repo workflow.Repository,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) *TriggerHandler {
	publisher := nats.NewEventPublisher(nc)
	return &TriggerHandler{
		cmd:           &pb.CmdWorkflowTrigger{},
		nc:            nc,
		repo:          repo,
		projectConfig: projectConfig,
		workflows:     workflows,
		publisher:     publisher,
	}
}

func (h *TriggerHandler) Subjects(workflowExecID string, execID string) []string {
	return []string{
		h.cmd.ToSubjectParams(workflowExecID, execID),
	}
}

func (h *TriggerHandler) Handle(ctx context.Context, msg jetstream.Msg) error {
	data := msg.Data()
	if err := proto.Unmarshal(data, h.cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdWorkflowTrigger: %w", err)
	}
	cmd := h.cmd
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
	evt := events.NewCmdExecute(h.nc, h.cmd)
	if err := evt.Publish(ctx); err != nil {
		return fmt.Errorf("failed to send CmdWorkflowExecute: %w", err)
	}
	return nil
}
