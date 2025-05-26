package events

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
)

type EventStarted struct {
	*pb.EventTaskStarted
	publisher core.EventPublisher
}

func NewEventStarted(nc *nats.Client, metadata *pb.TaskMetadata) *EventStarted {
	source := core.SourceTaskExecutor
	clonedMetadata := metadata.MustClone(source)
	event := &pb.EventTaskStarted{
		Metadata: clonedMetadata,
		Details: &pb.EventTaskStarted_Details{
			Status: pb.TaskStatus_TASK_STATUS_RUNNING,
		},
	}
	event.Metadata.Subject = event.ToSubject()
	publisher := nats.NewEventPublisher(nc)
	return &EventStarted{EventTaskStarted: event, publisher: publisher}
}

func (s *EventStarted) Publish(ctx context.Context) error {
	return s.publisher.Publish(ctx, s)
}
