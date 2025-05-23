package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	wfevts "github.com/compozy/compozy/engine/domain/workflow/events"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// Events
// -----------------------------------------------------------------------------

func (o *Orchestrator) SendWorkflowTrigger(ti *common.Input, workflowID string) (*wfevts.TriggerResponse, error) {
	return wfevts.SendTrigger(o.nc, ti, workflowID)
}

func (o *Orchestrator) SendWorkflowExecute(cmd *pb.CmdWorkflowTrigger) error {
	return wfevts.SendExecute(o.nc, cmd)
}

// -----------------------------------------------------------------------------
// Subscriber
// -----------------------------------------------------------------------------

func (o *Orchestrator) subscribeWorkflow(ctx context.Context) error {
	if err := o.subscribeWorkflowTrigger(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to CmdWorkflowTrigger: %w", err)
	}
	return nil
}

func (o *Orchestrator) subscribeWorkflowTrigger(ctx context.Context) error {
	comp := nats.ComponentWorkflow
	cmd := nats.CmdTypeTrigger
	err := o.nc.SubscribeCmd(ctx, comp, cmd, o.handleWorkflowTrigger)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------------------

func (o *Orchestrator) handleWorkflowTrigger(_ string, data []byte, _ jetstream.Msg) error {
	var cmd pb.CmdWorkflowTrigger
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdWorkflowTrigger: %w", err)
	}
	if err := o.SendWorkflowExecute(&cmd); err != nil {
		return fmt.Errorf("failed to send CmdWorkflowExecute: %w", err)
	}
	return nil
}
