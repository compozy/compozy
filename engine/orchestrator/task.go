package orchestrator

import (
	"context"
	"fmt"

	tkevts "github.com/compozy/compozy/engine/domain/task/events"
	taskuc "github.com/compozy/compozy/engine/domain/task/uc"
	"github.com/compozy/compozy/pkg/nats"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// Events
// -----------------------------------------------------------------------------

func (o *Orchestrator) SendTaskExecute(cmd *pbtask.CmdTaskDispatch) error {
	return tkevts.SendExecute(o.nc, cmd)
}

// -----------------------------------------------------------------------------
// Subscriber
// -----------------------------------------------------------------------------

func (o *Orchestrator) subscribeTask(ctx context.Context) error {
	if err := o.subscribeTaskDispatch(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to CmdTaskDispatch: %w", err)
	}
	return nil
}

func (o *Orchestrator) subscribeTaskDispatch(ctx context.Context) error {
	comp := nats.ComponentTask
	cmd := nats.CmdTypeDispatch
	err := o.nc.SubscribeCmd(ctx, comp, cmd, o.handleTaskDispatch)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------------------

func (o *Orchestrator) handleTaskDispatch(_ string, data []byte, _ jetstream.Msg) error {
	var cmd pbtask.CmdTaskDispatch
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdTaskDispatch: %w", err)
	}
	_, _, err := taskuc.CreateInitState(o.stManager, &cmd, o.workflows)
	if err != nil {
		return fmt.Errorf("failed to create and validate task state: %w", err)
	}
	if err := o.SendTaskExecute(&cmd); err != nil {
		return fmt.Errorf("failed to send CmdTaskExecute: %w", err)
	}
	return nil
}
