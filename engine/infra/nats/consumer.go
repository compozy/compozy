package nats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/nats-io/nats.go/jetstream"
)

const defAckWait = 30 * time.Second

func createConsumer(
	ctx context.Context,
	stream jetstream.Stream,
	name string,
	subjects []string,
) (jetstream.Consumer, error) {
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:           name,
		Durable:        name,
		FilterSubjects: subjects,
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        defAckWait,
		MaxDeliver:     3,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create %s consumer: %w", name, err)
	}
	return consumer, nil
}

func CmdConsumerName(comp core.ComponentType, cmd core.CmdType) string {
	if cmd == core.CmdAll {
		return strings.ToUpper(fmt.Sprintf("%s_cmds", comp))
	}
	return strings.ToUpper(fmt.Sprintf("%s_cmds_%s", comp, cmd.String()))
}

func EventConsumerName(comp core.ComponentType, evt core.EvtType) string {
	if evt == core.EvtAll {
		return strings.ToUpper(fmt.Sprintf("%s_evts", comp))
	}
	return strings.ToUpper(fmt.Sprintf("%s_evts_%s", comp, evt.String()))
}

func LogConsumerName(comp core.ComponentType, lvl logger.LogLevel) string {
	if lvl == logger.NoLevel {
		return strings.ToUpper(fmt.Sprintf("%s_logs", comp))
	}
	return strings.ToUpper(fmt.Sprintf("%s_logs_%s", comp, lvl.String()))
}

func (c *Client) GetCmdConsumer(
	ctx context.Context,
	name string,
	subjects []string,
) (jetstream.Consumer, error) {
	stream, err := c.GetStream(ctx, core.StreamCommands)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream: %w", err)
	}
	return createConsumer(ctx, stream, name, subjects)
}

func (c *Client) GetEvtConsumer(
	ctx context.Context,
	name string,
	subjects []string,
) (jetstream.Consumer, error) {
	stream, err := c.GetStream(ctx, core.StreamEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to get events stream: %w", err)
	}
	return createConsumer(ctx, stream, name, subjects)
}

func (c *Client) GetLogConsumer(
	ctx context.Context,
	name string,
	subjects []string,
) (jetstream.Consumer, error) {
	stream, err := c.GetStream(ctx, core.StreamLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to get log stream: %w", err)
	}

	return createConsumer(ctx, stream, name, subjects)
}
