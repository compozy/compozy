package pubsub

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

const defaultMsgBuffer = 64

// RedisProvider implements the Provider interface using Redis Pub/Sub.
type RedisProvider struct {
	client redis.UniversalClient
}

// NewRedisProvider constructs a Provider backed by a Redis client.
func NewRedisProvider(client redis.UniversalClient) (*RedisProvider, error) {
	if client == nil {
		return nil, errors.New("pubsub: redis client is nil")
	}
	return &RedisProvider{client: client}, nil
}

func (p *RedisProvider) Subscribe(ctx context.Context, channel string) (Subscription, error) {
	if ctx == nil {
		return nil, errors.New("pubsub: nil context")
	}
	if strings.TrimSpace(channel) == "" {
		return nil, errors.New("pubsub: empty channel")
	}
	pubsub := p.client.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, err
	}
	subCtx, cancel := context.WithCancel(ctx)
	out := make(chan Message, defaultMsgBuffer)
	sub := &redisSubscription{
		pubsub:   pubsub,
		cancel:   cancel,
		messages: out,
		done:     make(chan struct{}),
	}
	go func(messages <-chan *redis.Message) {
		defer close(out)
		for {
			select {
			case <-subCtx.Done():
				sub.closeWith(subCtx.Err())
				return
			case msg, ok := <-messages:
				if !ok {
					sub.closeWith(errors.New("pubsub: remote subscription closed"))
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
					sub.closeWith(subCtx.Err())
					return
				}
			}
		}
	}(pubsub.Channel())

	return sub, nil
}

func (p *RedisProvider) Publish(ctx context.Context, channel string, payload []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return p.client.Publish(ctx, channel, payload).Err()
}

type redisSubscription struct {
	pubsub    *redis.PubSub
	cancel    context.CancelFunc
	messages  <-chan Message
	done      chan struct{}
	closeOnce sync.Once
	errMu     sync.Mutex
	err       error
}

func (s *redisSubscription) Messages() <-chan Message {
	return s.messages
}

func (s *redisSubscription) Done() <-chan struct{} {
	return s.done
}

func (s *redisSubscription) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

func (s *redisSubscription) Close() error {
	s.closeWith(nil)
	return s.Err()
}

func (s *redisSubscription) closeWith(err error) {
	s.closeOnce.Do(func() {
		s.setErr(err)
		if closeErr := s.pubsub.Close(); closeErr != nil {
			s.setErr(closeErr)
		}
		s.cancel()
		close(s.done)
	})
}

func (s *redisSubscription) setErr(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if s.err == nil {
		s.err = err
	}
}
