package nats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/nats-io/nats.go/jetstream"
)

const defAckWait = 30 * time.Second

func createConsumer(
	ctx context.Context,
	js jetstream.Stream,
	name, subjects string,
) (jetstream.Consumer, error) {
	consumer, err := js.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          name,
		Durable:       name,
		FilterSubject: subjects,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       defAckWait,
		MaxDeliver:    3,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create %s consumer: %w", name, err)
	}
	return consumer, nil
}

func csNameCmd(comp ComponentType, cmd CmdType) string {
	return strings.ToUpper(fmt.Sprintf("%s_cmds_%s", comp, cmd.String()))
}

func csNameEvt(comp ComponentType, evt EvtType) string {
	return strings.ToUpper(fmt.Sprintf("%s_evts_%s", comp, evt.String()))
}

func csNameLog(comp ComponentType, lvl logger.LogLevel) string {
	return strings.ToUpper(fmt.Sprintf("%s_logs_%s", comp, lvl.String()))
}

func (c *Client) GetConsumerCmd(ctx context.Context, comp ComponentType, cmd CmdType) (jetstream.Consumer, error) {
	var stName StreamName
	switch comp {
	case ComponentWorkflow:
		stName = StreamWorkflowCmds
	case ComponentTask:
		stName = StreamTaskCmds
	case ComponentAgent:
		stName = StreamAgentCmds
	case ComponentTool:
		stName = StreamToolCmds
	case ComponentLog:
		return nil, fmt.Errorf("log component doesn't support commands")
	default:
		return nil, fmt.Errorf("unsupported component type for commands: %s", comp)
	}

	st, err := c.GetStream(ctx, stName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream: %w", err)
	}

	name := csNameCmd(comp, cmd)
	subjs := BuildCmdSubject(comp, "*", "*", cmd)
	cs, err := createConsumer(ctx, st, name, subjs)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

func (c *Client) GetConsumerEvt(ctx context.Context, comp ComponentType, evt EvtType) (jetstream.Consumer, error) {
	if comp == ComponentLog {
		return nil, fmt.Errorf("use GetConsumerLog for log events")
	}

	st, err := c.GetStream(ctx, StreamEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to get events stream: %w", err)
	}

	name := csNameEvt(comp, evt)
	subjs := BuildEvtSubject(comp, "*", "*", evt)
	cs, err := createConsumer(ctx, st, name, subjs)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

func (c *Client) GetConsumerLog(ctx context.Context, comp ComponentType, lvl logger.LogLevel) (jetstream.Consumer, error) {
	stream, err := c.GetStream(ctx, StreamLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to get log stream: %w", err)
	}

	name := csNameLog(comp, lvl)
	subjs := BuildLogSubject(comp, "*", "*", lvl)
	cs, err := createConsumer(ctx, stream, name, subjs)
	if err != nil {
		return nil, err
	}
	return cs, nil
}
