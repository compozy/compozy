package executor

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/pkg/nats"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

func (e *Executor) subscribeExecute(ctx context.Context) error {
	comp := nats.ComponentTask
	cmd := nats.CmdTypeExecute
	err := e.nc.SubscribeCmd(ctx, comp, cmd, e.handleExecute)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

func (e *Executor) handleExecute(_ string, data []byte, _ jetstream.Msg) error {
	// Unmarshal command from event
	var cmd pbtask.CmdTaskExecute
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdTaskExecute: %w", err)
	}
	_, err := task.InfoFromEvent(&cmd)
	if err != nil {
		return fmt.Errorf("failed to parse task payload info: %w", err)
	}

	return nil
}
