package events

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
)

type CmdExecute struct {
	*pb.CmdTaskExecute
	publisher core.EventPublisher
}

func NewCmdExecute(nc *nats.Client, dispatchCmd *pb.CmdTaskDispatch) *CmdExecute {
	source := core.SourceOrchestrator
	metadata := dispatchCmd.GetMetadata().MustClone(source)
	cmd := &pb.CmdTaskExecute{
		Metadata: metadata,
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	publisher := nats.NewEventPublisher(nc)
	return &CmdExecute{CmdTaskExecute: cmd, publisher: publisher}
}

func (e *CmdExecute) Publish(ctx context.Context) error {
	return e.publisher.Publish(ctx, e)
}
