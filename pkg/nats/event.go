package nats

import (
	"fmt"

	pbagent "github.com/compozy/compozy/pkg/pb/agent"
	"github.com/compozy/compozy/pkg/pb/common"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
	pbtool "github.com/compozy/compozy/pkg/pb/tool"
	pbworkflow "github.com/compozy/compozy/pkg/pb/workflow"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

type Event interface {
	GetMetadata() *common.Metadata
	GetWorkflow() *common.WorkflowInfo
	GetTask() *common.TaskInfo
	GetAgent() *common.AgentInfo
	GetTool() *common.ToolInfo
	GetDetails() EventPayload
}

type EventPayload interface {
	GetContext() *structpb.Struct
}

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
		event := &pbworkflow.EventWorkflowStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow started event: %w", err)
		}
		return event, nil
	case EvtTypePaused:
		event := &pbworkflow.EventWorkflowPaused{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow paused event: %w", err)
		}
		return event, nil
	case EvtTypeResumed:
		event := &pbworkflow.EventWorkflowResumed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow resumed event: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pbworkflow.EventWorkflowSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow success event: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pbworkflow.EventWorkflowFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow failed event: %w", err)
		}
		return event, nil
	case EvtTypeCanceled:
		event := &pbworkflow.EventWorkflowCanceled{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow canceled event: %w", err)
		}
		return event, nil
	case EvtTypeTimedOut:
		event := &pbworkflow.EventWorkflowTimedOut{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal workflow timed out event: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported workflow event type: %s", evtType)
	}
}

func parseAgentEvent(evtType EvtType, data []byte) (any, error) {
	switch evtType {
	case EvtTypeStarted:
		event := &pbagent.EventAgentStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent started event: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pbagent.EventAgentSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent success event: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pbagent.EventAgentFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal agent failed event: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported agent event type: %s", evtType)
	}
}

func parseTaskEvent(evtType EvtType, data []byte) (any, error) {
	switch evtType {
	case EvtTypeDispatched:
		event := &pbtask.EventTaskDispatched{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task dispatched event: %w", err)
		}
		return event, nil
	case EvtTypeStarted:
		event := &pbtask.EventTaskStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task started event: %w", err)
		}
		return event, nil
	case EvtTypeWaitingStarted:
		event := &pbtask.EventTaskWaiting{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task waiting started event: %w", err)
		}
		return event, nil
	case EvtTypeWaitingEnded:
		event := &pbtask.EventTaskWaitingEnded{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task waiting ended event: %w", err)
		}
		return event, nil
	case EvtTypeWaitingTimedOut:
		event := &pbtask.EventTaskWaitingTimedOut{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task waiting timed out event: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pbtask.EventTaskSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task success event: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pbtask.EventTaskFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal task failed event: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported task event type: %s", evtType)
	}
}

func parseToolEvent(evtType EvtType, data []byte) (any, error) {
	switch evtType {
	case EvtTypeStarted:
		event := &pbtool.EventToolStarted{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool started event: %w", err)
		}
		return event, nil
	case EvtTypeSuccess:
		event := &pbtool.EventToolSuccess{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool success event: %w", err)
		}
		return event, nil
	case EvtTypeFailed:
		event := &pbtool.EventToolFailed{}
		if err := proto.Unmarshal(data, event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool failed event: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unsupported tool event type: %s", evtType)
	}
}
