package events

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
)

type CmdExecute struct {
	*pb.CmdWorkflowExecute
	publisher core.EventPublisher
}

func NewCmdExecute(nc *nats.Client, triggerCmd *pb.CmdWorkflowTrigger) *CmdExecute {
	source := core.SourceOrchestrator
	metadata := triggerCmd.GetMetadata().MustClone(source)
	cmd := &pb.CmdWorkflowExecute{
		Metadata: metadata,
		Details: &pb.CmdWorkflowExecute_Details{
			TriggerInput: triggerCmd.GetDetails().GetTriggerInput(),
		},
	}
	cmd.Metadata.Subject = cmd.ToSubject()
	publisher := nats.NewEventPublisher(nc)
	return &CmdExecute{CmdWorkflowExecute: cmd, publisher: publisher}
}

func (e *CmdExecute) Publish(ctx context.Context) error {
	return e.publisher.Publish(ctx, e)
}
