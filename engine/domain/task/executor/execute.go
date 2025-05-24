package executor

import (
	"context"
	"fmt"

	tkevts "github.com/compozy/compozy/engine/domain/task/events"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/pb"
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
	var cmd pb.CmdTaskExecute
	if err := proto.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal CmdTaskExecute: %w", err)
	}
	logger.With("metadata", cmd.Metadata).Info("Received: TaskExecute")

	// Send TaskExecutionStart
	metadata := cmd.GetMetadata()
	if err := tkevts.SendStarted(e.nc, metadata); err != nil {
		return fmt.Errorf("failed to send EventTaskStarted: %w", err)
	}

	return nil
}
