package pubsub

import "context"

// Message represents a payload delivered via a pub/sub subscription.
type Message struct {
	Payload []byte
}

// Subscription exposes a stream of messages and allows callers to observe
// termination state. Close must be safe to call multiple times.
type Subscription interface {
	Messages() <-chan Message
	Done() <-chan struct{}
	Err() error
	Close() error
}

// Provider describes a component capable of subscribing to named channels and
// delivering messages published to them.
type Provider interface {
	Subscribe(ctx context.Context, channel string) (Subscription, error)
	Publish(ctx context.Context, channel string, payload []byte) error
}
