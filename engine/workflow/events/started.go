package events

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
)

type EventStarted struct {
	*pb.EventWorkflowStarted
	publisher core.EventPublisher
}

func NewEventStarted(nc *nats.Client, metadata *pb.WorkflowMetadata) *EventStarted {
	source := core.SourceWorkflowExecute
	clonedMetadata := metadata.MustClone(source)
	event := &pb.EventWorkflowStarted{
		Metadata: clonedMetadata,
		Details: &pb.EventWorkflowStarted_Details{
			Status: pb.WorkflowStatus_WORKFLOW_STATUS_RUNNING,
		},
	}
	event.Metadata.Subject = event.ToSubject()
	publisher := nats.NewEventPublisher(nc)
	return &EventStarted{EventWorkflowStarted: event, publisher: publisher}
}

func (s *EventStarted) Publish(ctx context.Context) error {
	return s.publisher.Publish(ctx, s)
}
