package events

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/types/known/structpb"
)

type EventFailed struct {
	*pb.EventWorkflowFailed
	publisher core.EventPublisher
}

func NewEventFailed(nc *nats.Client, metadata *pb.WorkflowMetadata, errData error) *EventFailed {
	code := "WORKFLOW_EXECUTION_FAILED"
	source := core.SourceWorkflowExecute
	clonedMetadata := metadata.MustClone(source)
	event := &pb.EventWorkflowFailed{
		Metadata: clonedMetadata,
		Details: &pb.EventWorkflowFailed_Details{
			Status: pb.WorkflowStatus_WORKFLOW_STATUS_FAILED,
			Error: &pb.ErrorResult{
				Code:    &code,
				Message: errData.Error(),
				Details: &structpb.Struct{},
			},
		},
	}
	event.Metadata.Subject = event.ToSubject()
	publisher := nats.NewEventPublisher(nc)
	return &EventFailed{EventWorkflowFailed: event, publisher: publisher}
}

func (f *EventFailed) Publish(ctx context.Context) error {
	return f.publisher.Publish(ctx, f)
}
