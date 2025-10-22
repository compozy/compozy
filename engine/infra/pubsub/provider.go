package pubsub

import "context"

// Message represents a payload delivered via a pub/sub subscription.
type Message struct {
	Payload []byte
}

// Subscription exposes a stream of messages and allows callers to release
// underlying resources when the stream is no longer needed.
type Subscription interface {
	Messages() <-chan Message
	Close() error
}

// Provider describes a component capable of subscribing to named channels and
// delivering messages published to them.
type Provider interface {
	Subscribe(ctx context.Context, channel string) (Subscription, error)
}
