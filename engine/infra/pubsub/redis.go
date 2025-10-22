package pubsub

import (
	"context"
	"errors"
	"sync"

	"github.com/redis/go-redis/v9"
)

// RedisProvider implements the Provider interface using Redis Pub/Sub.
type RedisProvider struct {
	client *redis.Client
}

// NewRedisProvider constructs a Provider backed by a Redis client.
func NewRedisProvider(client *redis.Client) (*RedisProvider, error) {
	if client == nil {
		return nil, errors.New("pubsub: redis client is nil")
	}
	return &RedisProvider{client: client}, nil
}

func (p *RedisProvider) Subscribe(ctx context.Context, channel string) (Subscription, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	pubsub := p.client.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, err
	}
	subCtx, cancel := context.WithCancel(ctx)
	out := make(chan Message, 64)
	go func(messages <-chan *redis.Message) {
		defer close(out)
		for {
			select {
			case <-subCtx.Done():
				return
			case msg, ok := <-messages:
				if !ok {
					return
				}
				if msg == nil {
					continue
				}
				copied := make([]byte, len(msg.Payload))
				copy(copied, msg.Payload)
				select {
				case out <- Message{Payload: copied}:
				case <-subCtx.Done():
					return
				}
			}
		}
	}(pubsub.Channel())

	return &redisSubscription{pubsub: pubsub, cancel: cancel, messages: out}, nil
}

type redisSubscription struct {
	pubsub   *redis.PubSub
	cancel   context.CancelFunc
	messages <-chan Message
	once     sync.Once
}

func (s *redisSubscription) Messages() <-chan Message {
	return s.messages
}

func (s *redisSubscription) Close() error {
	var err error
	s.once.Do(func() {
		s.cancel()
		err = s.pubsub.Close()
	})
	return err
}
