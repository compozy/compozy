package nats

import (
	"github.com/compozy/compozy/pkg/pb/common"
	"google.golang.org/protobuf/types/known/structpb"
)

type Event interface {
	GetMetadata() *common.Metadata
	GetWorkflow() *common.WorkflowInfo
	GetTask() *common.TaskInfo
	GetAgent() *common.AgentInfo
	GetTool() *common.ToolInfo
	GetPayload() EventPayload
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
	case TaskStatusCancelled, WorkflowStatusCanceled:
		return StatusCanceled
	case TaskStatusTimedOut, WorkflowStatusTimedOut:
		return StatusTimedOut
	default:
		return StatusPending // Default fallback
	}
}
