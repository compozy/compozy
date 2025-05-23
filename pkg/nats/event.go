package nats

import (
	"fmt"

	"github.com/compozy/compozy/pkg/pb"
	"google.golang.org/protobuf/proto"
)

type EventStatus interface {
	String() string
	Number() int32
}

func ToStatus(eventStatus EventStatus) EvStatusType {
	switch eventStatus.String() {
	case AgentStatusUnspecified, TaskStatusUnspecified, ToolStatusUnspecified, WorkflowStatusUnspecified:
		return StatusPending
	case TaskStatusPending, WorkflowStatusPending:
		return StatusPending
	case AgentStatusRunning, TaskStatusRunning, ToolStatusRunning, WorkflowStatusRunning:
		return StatusRunning
	case AgentStatusSuccess, TaskStatusSuccess, ToolStatusSuccess, WorkflowStatusSuccess:
		return StatusSuccess
	case AgentStatusFailed, TaskStatusFailed, ToolStatusFailed, WorkflowStatusFailed:
		return StatusFailed
	case TaskStatusWaiting:
		return StatusWaiting
	case WorkflowStatusPaused:
		return StatusPaused
	case TaskStatusCanceled, WorkflowStatusCanceled:
		return StatusCanceled
	case TaskStatusTimedOut, WorkflowStatusTimedOut:
		return StatusTimedOut
	default:
		return StatusPending // Default fallback
	}
}

// -----------------------------------------------------------------------------
// Parser
// -----------------------------------------------------------------------------

func ParseEvent(compType ComponentType, evtType EvtType, data []byte) (any, error) {
	switch compType {
	case ComponentWorkflow:
		return parseWorkflowEvent(evtType, data)
	case ComponentTask:
		return parseTaskEvent(evtType, data)
	case ComponentAgent:
		return parseAgentEvent(evtType, data)
	case ComponentTool:
		return parseToolEvent(evtType, data)
	default:
		return nil, fmt.Errorf("unsupported component type: %s", compType)
	}
}

func parseWorkflowEvent(evtType EvtType, data []byte) (any, error) {
	switch evtType {
	case EvtTypeStarted:
		event := &pb.EventWorkflowStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowStarted: %w", err)
		}
		return event, nil
	case EvtTypePaused:
		event := &pb.EventWorkflowPaused{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowPaused: %w", err)
		}
		return event, nil
	case EvtTypeResumed:
		event := &pb.EventWorkflowResumed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowResumed: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pb.EventWorkflowSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowSuccess: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pb.EventWorkflowFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowFailed: %w", err)
		}
		return event, nil
	case EvtTypeCanceled:
		event := &pb.EventWorkflowCanceled{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowCanceled: %w", err)
		}
		return event, nil
	case EvtTypeTimedOut:
		event := &pb.EventWorkflowTimedOut{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventWorkflowTimedOut: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported workflow event type: %s", evtType)
	}
}

func parseAgentEvent(evtType EvtType, data []byte) (any, error) {
	switch evtType {
	case EvtTypeStarted:
		event := &pb.EventAgentStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventAgentStarted: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pb.EventAgentSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventAgentSuccess: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pb.EventAgentFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventAgentFailed: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported agent event type: %s", evtType)
	}
}

func parseTaskEvent(evtType EvtType, data []byte) (any, error) {
	switch evtType {
	case EvtTypeDispatched:
		event := &pb.EventTaskDispatched{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskDispatched: %w", err)
		}
		return event, nil
	case EvtTypeStarted:
		event := &pb.EventTaskStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskStarted: %w", err)
		}
		return event, nil
	case EvtTypeWaitingStarted:
		event := &pb.EventTaskWaiting{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskWaiting: %w", err)
		}
		return event, nil
	case EvtTypeWaitingEnded:
		event := &pb.EventTaskWaitingEnded{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskWaitingEnded: %w", err)
		}
		return event, nil
	case EvtTypeWaitingTimedOut:
		event := &pb.EventTaskWaitingTimedOut{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskWaitingTimedOut: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pb.EventTaskSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskSuccess: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pb.EventTaskFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventTaskFailed: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported task event type: %s", evtType)
	}
}

func parseToolEvent(evtType EvtType, data []byte) (any, error) {
	switch evtType {
	case EvtTypeStarted:
		event := &pb.EventToolStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventToolStarted: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pb.EventToolSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventToolSuccess: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pb.EventToolFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal EventToolFailed: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported tool event type: %s", evtType)
	}
}
