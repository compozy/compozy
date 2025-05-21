package orchestrator

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/nats"
	"github.com/nats-io/nats.go/jetstream"
)

func (o *Orchestrator) subWorkflowCmds(ctx context.Context) error {
	cs, err := o.natsClient.GetConsumerCmd(ctx, nats.ComponentWorkflow, nats.CmdTypeAll)
	if err != nil {
		return fmt.Errorf("failed to get consumer: %w", err)
	}

	subOpts := nats.DefaultSubscribeOpts(cs)
	err = nats.SubscribeConsumer(ctx, o.handleWorkflowCmd, subOpts)
	if err != nil {
		return fmt.Errorf("failed to subscribe to state events: %w", err)
	}

	return nil
}

func (o *Orchestrator) handleWorkflowCmd(subject string, data []byte, msg jetstream.Msg) error {
	subj, err := nats.ParseCmdSubject(subject)
	if err != nil {
		return fmt.Errorf("failed to parse command subject: %w", err)
	}
	if subj.CompType != nats.ComponentWorkflow {
		return fmt.Errorf("invalid component type: %s", subj.CompType)
	}

	fmt.Println("Received workflow command:", string(data))
	return nil
}
