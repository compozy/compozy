package nats

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// Parser
// -----------------------------------------------------------------------------

func ParseEvent(compType core.ComponentType, evtType core.EvtType, data []byte) (any, error) {
	switch compType {
	case core.ComponentWorkflow:
		return parseWorkflowEvent(evtType, data)
	case core.ComponentTask:
		return parseTaskEvent(evtType, data)
	case core.ComponentAgent:
		return parseAgentEvent(evtType, data)
	case core.ComponentTool:
		return parseToolEvent(evtType, data)
	default:
		return nil, fmt.Errorf("unsupported component type: %s", compType)
	}
}

func parseWorkflowEvent(evtType core.EvtType, data []byte) (any, error) {
	switch evtType {
	case core.EvtStarted:
		event := &pb.EventWorkflowStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowStarted: %w", err)
		}
		return event, nil
	case core.EvtPaused:
		event := &pb.EventWorkflowPaused{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowPaused: %w", err)
		}
		return event, nil
	case core.EvtResumed:
		event := &pb.EventWorkflowResumed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowResumed: %w", err)
		}
		return event, nil
	case core.EvtSuccess:
		event := &pb.EventWorkflowSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowSuccess: %w", err)
		}
		return event, nil
	case core.EvtFailed:
		event := &pb.EventWorkflowFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowFailed: %w", err)
		}
		return event, nil
	case core.EvtCanceled:
		event := &pb.EventWorkflowCanceled{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowCanceled: %w", err)
		}
		return event, nil
	case core.EvtTimedOut:
		event := &pb.EventWorkflowTimedOut{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowTimedOut: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported workflow event type: %s", evtType)
	}
}

func parseAgentEvent(evtType core.EvtType, data []byte) (any, error) {
	switch evtType {
	case core.EvtStarted:
		event := &pb.EventAgentStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventAgentStarted: %w", err)
		}
		return event, nil
	case core.EvtSuccess:
		event := &pb.EventAgentSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventAgentSuccess: %w", err)
		}
		return event, nil
	case core.EvtFailed:
		event := &pb.EventAgentFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventAgentFailed: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported agent event type: %s", evtType)
	}
}

func parseTaskEvent(evtType core.EvtType, data []byte) (any, error) {
	switch evtType {
	case core.EvtDispatched:
		event := &pb.EventTaskDispatched{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskDispatched: %w", err)
		}
		return event, nil
	case core.EvtStarted:
		event := &pb.EventTaskStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskStarted: %w", err)
		}
		return event, nil
	case core.EvtWaitingStarted:
		event := &pb.EventTaskWaiting{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskWaiting: %w", err)
		}
		return event, nil
	case core.EvtWaitingEnded:
		event := &pb.EventTaskWaitingEnded{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskWaitingEnded: %w", err)
		}
		return event, nil
	case core.EvtWaitingTimedOut:
		event := &pb.EventTaskWaitingTimedOut{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskWaitingTimedOut: %w", err)
		}
		return event, nil
	case core.EvtSuccess:
		event := &pb.EventTaskSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskSuccess: %w", err)
		}
		return event, nil
	case core.EvtFailed:
		event := &pb.EventTaskFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskFailed: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported task event type: %s", evtType)
	}
}

func parseToolEvent(evtType core.EvtType, data []byte) (any, error) {
	switch evtType {
	case core.EvtStarted:
		event := &pb.EventToolStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventToolStarted: %w", err)
		}
		return event, nil
	case core.EvtSuccess:
		event := &pb.EventToolSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventToolSuccess: %w", err)
		}
		return event, nil
	case core.EvtFailed:
		event := &pb.EventToolFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventToolFailed: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported tool event type: %s", evtType)
	}
}

// -----------------------------------------------------------------------------
// Event Publisher Implementation
// -----------------------------------------------------------------------------

type EventPublisher struct {
	client *Client
}

func NewEventPublisher(client *Client) *EventPublisher {
	return &EventPublisher{
		client: client,
	}
}

func (p *EventPublisher) Publish(ctx context.Context, event core.Subjecter) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	_, err = p.client.js.Publish(ctx, event.ToSubject(), data)
	if err != nil {
		return fmt.Errorf("failed to publish event to JetStream: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Event Subscriber Implementation
// -----------------------------------------------------------------------------

type EventSubscriber struct {
	client *Client
}

func NewEventSubscriber(client *Client) *EventSubscriber {
	return &EventSubscriber{
		client: client,
	}
}

// SubscribeConsumer implements JetStream consumer subscription using any provided consumer
func (s *EventSubscriber) SubscribeConsumer(
	ctx context.Context,
	consumer jetstream.Consumer,
	handler core.EventHandler,
) error {
	subOpts := DefaultSubscribeOpts(consumer)
	errCh := SubscribeConsumer(ctx, handler, subOpts)
	go func() {
		for err := range errCh {
			if err != nil {
				logger.Error("Error in JetStream consumer subscription", "error", err)
			}
		}
	}()
	return nil
}
