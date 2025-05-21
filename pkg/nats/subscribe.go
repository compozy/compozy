package nats

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/nats-io/nats.go/jetstream"
)

type MessageHandler func(subject string, data []byte, msg jetstream.Msg) error

type SubscribeOpts struct {
	consumer     jetstream.Consumer
	batchSize    int
	fetchTimeout time.Duration
	maxRetries   int
}

func DefaultSubscribeOpts(consumer jetstream.Consumer) SubscribeOpts {
	return SubscribeOpts{
		consumer:     consumer,
		batchSize:    100,
		fetchTimeout: time.Second * 5,
		maxRetries:   3,
	}
}

// SubscribeConsumer starts a goroutine to consume messages from a NATS consumer
// and returns a channel that will receive errors if they occur.
// The goroutine will continue running until the context is canceled.
func SubscribeConsumer(ctx context.Context, handler MessageHandler, opts SubscribeOpts) error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		for {
			select {
			case <-ctx.Done():
				logger.Warn("Context canceled, stopping consumption")
				errCh <- ctx.Err()
				return
			default:
				// Fetch messages with timeout
				msgs, err := opts.consumer.Fetch(opts.batchSize, jetstream.FetchMaxWait(opts.fetchTimeout))
				if err != nil {
					if errors.Is(err, jetstream.ErrConsumerNotFound) {
						logger.Error("Consumer not found, exiting")
						errCh <- err
						return
					}
					if errors.Is(err, context.DeadlineExceeded) {
						logger.Warn("No messages available, retrying...")
						continue
					}
					logger.Warn("Fetch error", "error", err)
					time.Sleep(time.Second) // Simple backoff
					continue
				}

				// Process messages
				count := 0
				for msg := range msgs.Messages() {
					count++
					subject, data := msg.Subject(), msg.Data()
					err = handler(subject, data, msg)
					if err != nil {
						logger.Error("Error processing message", "subject", subject, "error", err)
						if err := msg.Nak(); err != nil {
							logger.Error("Failed to Nak message", "error", err)
						}
						continue
					}
					if err := msg.Ack(); err != nil {
						logger.Error("Failed to ack message", "subject", subject, "error", err)
					} else {
						logger.Info("Processed message", "subject", subject)
					}
				}

				if err := msgs.Error(); err != nil {
					if errors.Is(err, jetstream.ErrMsgNotFound) {
						logger.Error("No more messages to fetch, waiting...")
						time.Sleep(time.Second)
						continue
					}
					logger.Error("Batch fetch error", "error", err)
					time.Sleep(time.Second)
					continue
				}
			}
		}
	}()

	return nil
}
