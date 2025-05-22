package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	pbwf "github.com/compozy/compozy/pkg/pb/workflow"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

func (o *Orchestrator) subscribeWorkflow(ctx context.Context) error {
	if err := o.subscribeWorkflowTrigger(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to WorkflowTriggerCmd: %w", err)
	}
	if err := o.subscribeWorkflowExecute(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to WorkflowExecuteCmd: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Trigger Command
// -----------------------------------------------------------------------------

func (o *Orchestrator) subscribeWorkflowTrigger(ctx context.Context) error {
	subCtx := context.Background()
	cs, err := o.natsClient.GetConsumerCmd(ctx, nats.ComponentWorkflow, nats.CmdTypeTrigger)
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

func (o *Orchestrator) handleWorkflowTrigger(_ string, data []byte, _ jetstream.Msg) error {
	var cmd pbwf.WorkflowTriggerCommand
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal WorkflowTriggerCommand: %w", err)
	}
	if err := o.SendWorkflowExecutionStarted(&cmd); err != nil {
		return fmt.Errorf("failed to send WorkflowExecutionStarted: %w", err)
	}
	if err := o.SendExecuteWorkflow(&cmd); err != nil {
		return fmt.Errorf("failed to send ExecuteWorkflow: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Execute Workflow
// -----------------------------------------------------------------------------

func (o *Orchestrator) subscribeWorkflowExecute(ctx context.Context) error {
	subCtx := context.Background()
	cs, err := o.natsClient.GetConsumerCmd(ctx, nats.ComponentWorkflow, nats.CmdTypeExecute)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}
	subOpts := nats.DefaultSubscribeOpts(cs)
	errCh := nats.SubscribeConsumer(subCtx, o.handleWorkflowExecute, subOpts)
	go func() {
		for err := range errCh {
			if err != nil {
				logger.Error("Error in workflow execute subscription", "error", err)
			}
		}
	}()
	return nil
}

func (o *Orchestrator) handleWorkflowExecute(_ string, data []byte, _ jetstream.Msg) error {
	var cmd pbwf.WorkflowExecuteCommand
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal WorkflowExecuteCommand: %w", err)
	}
	return nil
}
