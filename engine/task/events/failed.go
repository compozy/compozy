package events

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/structpb"
)

type EventFailed struct {
	*pb.EventTaskFailed
	publisher core.EventPublisher
}

func NewEventFailed(nc *nats.Client, metadata *pb.TaskMetadata, errData error) *EventFailed {
	code := "TASK_EXECUTION_FAILED"
	source := core.SourceTaskExecutor
	clonedMetadata := metadata.MustClone(source)
	event := &pb.EventTaskFailed{
		Metadata: clonedMetadata,
		Details: &pb.EventTaskFailed_Details{
			Status: pb.TaskStatus_TASK_STATUS_FAILED,
			Error: &pb.ErrorResult{
				Code:    &code,
				Message: errData.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	event.Metadata.Subject = event.ToSubject()
	publisher := nats.NewEventPublisher(nc)
	return &EventFailed{EventTaskFailed: event, publisher: publisher}
}

func (f *EventFailed) Publish(ctx context.Context) error {
	return f.publisher.Publish(ctx, f)
}
