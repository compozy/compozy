package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

func (o *Orchestrator) subscribeWorkflowCmds(ctx context.Context) error {
	if err := o.subscribeWorkflowTriggerCmd(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to workflow trigger commands: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Trigger Command
// -----------------------------------------------------------------------------

func (o *Orchestrator) subscribeWorkflowTriggerCmd(ctx context.Context) error {
	subCtx := context.Background()
	cs, err := o.natsClient.GetConsumerCmd(ctx, nats.ComponentWorkflow, nats.CmdTypeTrigger)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}
	subOpts := nats.DefaultSubscribeOpts(cs)
	err = nats.SubscribeConsumer(subCtx, o.handleWorkflowTriggerCmd, subOpts)
	if err != nil {
		return fmt.Errorf("failed to subscribe to state events: %w", err)
	}
	return nil
}

func (o *Orchestrator) handleWorkflowTriggerCmd(subject string, _ []byte, _ jetstream.Msg) error {
	subj, err := nats.ParseCmdSubject(subject)
	if err != nil {
		return fmt.Errorf("failed to parse command subject: %w", err)
	}
	if subj.CompType != nats.ComponentWorkflow {
		return fmt.Errorf("invalid component type: %s", subj.CompType)
	}
	return nil
}
