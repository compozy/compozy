package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/common"
	wfevts "github.com/compozy/compozy/engine/domain/workflow/events"
	wfuc "github.com/compozy/compozy/engine/domain/workflow/uc"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// Events
// -----------------------------------------------------------------------------

func (o *Orchestrator) SendWorkflowTrigger(wfID string, ti *common.Input) (*wfevts.TriggerResponse, error) {
	// Find and validate workflow config input
	if err := wfuc.FindAndValidateInput(o.workflows, wfID, *ti); err != nil {
		return nil, fmt.Errorf("failed to validate workflow: %w", err)
	}
	// Create initial workflow state
	st, err := o.stManager.CreateWorkflowState(ti, o.pjc)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow state: %w", err)
	}
	return wfevts.SendTrigger(wfID, o.nc, st)
}

func (o *Orchestrator) SendExecuteWorkflow(cmd *pbwf.WorkflowTriggerCommand) error {
	return wfevts.SendExecute(o.nc, cmd)
}

// -----------------------------------------------------------------------------
// Subscriber
// -----------------------------------------------------------------------------

func (o *Orchestrator) subscribeWorkflow(ctx context.Context) error {
	if err := o.subscribeWorkflowTrigger(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to WorkflowTriggerCmd: %w", err)
	}
	return nil
}

func (o *Orchestrator) subscribeWorkflowTrigger(ctx context.Context) error {
	subCtx := context.Background()
	cs, err := o.nc.GetConsumerCmd(ctx, nats.ComponentWorkflow, nats.CmdTypeTrigger)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}
	subOpts := nats.DefaultSubscribeOpts(cs)
	errCh := nats.SubscribeConsumer(subCtx, o.handleWorkflowTrigger, subOpts)
	go func() {
		for err := range errCh {
			if err != nil {
				logger.Error("Error in workflow trigger subscription", "error", err)
			}
		}
	}()
	return nil
}

// -----------------------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------------------

func (o *Orchestrator) handleWorkflowTrigger(_ string, data []byte, _ jetstream.Msg) error {
	var cmd pbwf.WorkflowTriggerCommand
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal WorkflowTriggerCommand: %w", err)
	}
	wf := cmd.GetWorkflow()
	md := cmd.GetMetadata()
	if err := wfevts.SendExecutionStarted(o.nc, wf, md); err != nil {
		return fmt.Errorf("failed to send WorkflowExecutionStarted: %w", err)
	}
	if err := o.SendExecuteWorkflow(&cmd); err != nil {
		return fmt.Errorf("failed to send ExecuteWorkflow: %w", err)
	}
	return nil
}
