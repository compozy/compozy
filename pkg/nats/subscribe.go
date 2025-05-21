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

func SubscribeConsumer(ctx context.Context, handler MessageHandler, opts SubscribeOpts) error {
	for {
		select {
		case <-ctx.Done():
			logger.Warn("Context canceled, stopping consumption")
			return ctx.Err()
		default:
			// Fetch messages with timeout
			msgs, err := opts.consumer.Fetch(opts.batchSize, jetstream.FetchMaxWait(opts.fetchTimeout))
			if err != nil {
				if errors.Is(err, jetstream.ErrConsumerNotFound) {
					logger.Error("Consumer not found, exiting")
					return err
				}
				if errors.Is(err, context.DeadlineExceeded) {
					logger.Warn("No messages available, retrying...")
					continue
				}
				logger.Warn("Fetch error: %v, retrying...", err)
				time.Sleep(time.Second) // Simple backoff
				continue
			}

			// Process messages
			count := 0
			for msg := range msgs.Messages() {
				count++
				subject, data := msg.Subject(), msg.Data()

				// Process message using provided handler
				err = handler(subject, data, msg)
				if err != nil {
					logger.Error("Error processing message %s: %v\n", subject, err)
					if err := msg.Nak(); err != nil {
						logger.Error("Failed to Nak message: %v\n", err)
					}
					continue
				}

				// Acknowledge message
				if err := msg.Ack(); err != nil {
					logger.Error("Failed to ack message %s: %v\n", subject, err)
				} else {
					logger.Info("Processed message %s: %s\n", subject, data)
				}
			}

			if err := msgs.Error(); err != nil {
				if errors.Is(err, jetstream.ErrMsgNotFound) {
					logger.Error("No more messages to fetch, waiting...")
					time.Sleep(time.Second)
					continue
				}
				logger.Error("Batch fetch error: %v, retrying...\n", err)
				time.Sleep(time.Second)
				continue
			}

			logger.Info("Processed %d messages\n", count)
		}
	}
}
