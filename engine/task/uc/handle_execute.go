package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/engine/task"
	tkevts "github.com/compozy/compozy/engine/task/events"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

type HandleExecute struct {
	CMD       *pb.CmdTaskExecute
	nc        *nats.Client
	repo      task.Repository
	publisher core.EventPublisher
}

func NewHandleExecute(
	nc *nats.Client,
	repo task.Repository,
) *HandleExecute {
	publisher := nats.NewEventPublisher(nc)
	return &HandleExecute{
		CMD:       &pb.CmdTaskExecute{},
		nc:        nc,
		repo:      repo,
		publisher: publisher,
	}
}

func (h *HandleExecute) Handler(ctx context.Context, msg jetstream.Msg) error {
	if err := h.Execute(ctx, msg); err != nil {
		metadata := h.CMD.GetMetadata()
		evt := tkevts.NewEventFailed(h.nc, metadata, err)
		if err := evt.Publish(ctx); err != nil {
			return fmt.Errorf("failed to send EventTaskFailed: %w", err)
		}
		return err
	}
	return nil
}

func (h *HandleExecute) Execute(ctx context.Context, msg jetstream.Msg) error {
	data := msg.Data()
	// Unmarshal command from event
	if err := proto.Unmarshal(data, h.CMD); err != nil {
		return fmt.Errorf("failed to unmarshal CmdTaskExecute: %w", err)
	}
	h.CMD.Metadata.Logger().Debug(fmt.Sprintf("Received: %T", h.CMD))

	// Send EventTaskStarted
	metadata := h.CMD.GetMetadata()
	evt := tkevts.NewEventStarted(h.nc, metadata)
	if err := evt.Publish(ctx); err != nil {
		return fmt.Errorf("failed to send EventTaskStarted: %w", err)
	}
	return nil
}
