package core

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/types/known/structpb"
)

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

type Event interface {
	Subjecter
	Publish(ctx context.Context) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event Subjecter) error
}

type EventHandler func(ctx context.Context, msg jetstream.Msg) error

type EventSubscriber interface {
	SubscribeConsumer(
		ctx context.Context,
		consumer jetstream.Consumer,
		handler EventHandler,
	) error
}

// -----------------------------------------------------------------------------
// Protobuf Event
// -----------------------------------------------------------------------------

type ErrorPayload interface {
	GetMessage() string
	GetCode() string
	GetDetails() *structpb.Struct
}

type EventDetailsError interface {
	GetError() ErrorPayload
}

type EventDetailsSuccess interface {
	GetResult() *structpb.Struct
}

type EventSuccess interface {
	GetDetails() EventDetailsSuccess
}

type EventFailed interface {
	GetDetails() EventDetailsError
}
