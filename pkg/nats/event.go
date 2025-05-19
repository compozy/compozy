package nats

import (
	"github.com/compozy/compozy/pkg/pb/common"
	"google.golang.org/protobuf/types/known/structpb"
)

type ComponentType string

const (
	ComponentWorkflow ComponentType = "workflow"
	ComponentTask     ComponentType = "task"
	ComponentAgent    ComponentType = "agent"
	ComponentTool     ComponentType = "tool"
)

type EvStatusType string

const (
	StatusPending        EvStatusType = "PENDING"
	StatusScheduled      EvStatusType = "SCHEDULED"
	StatusRunning        EvStatusType = "RUNNING"
	StatusSuccess        EvStatusType = "SUCCESS"
	StatusFailed         EvStatusType = "FAILED"
	StatusTimedOut       EvStatusType = "TIMED_OUT"
	StatusCancelled      EvStatusType = "CANCELED"
	StatusWaiting        EvStatusType = "WAITING"
	StatusRetryScheduled EvStatusType = "RETRY_SCHEDULED"
)

const (
	AgentStatusUnspecified = "AGENT_STATUS_UNSPECIFIED"
	AgentStatusRunning     = "AGENT_STATUS_RUNNING"
	AgentStatusSuccess     = "AGENT_STATUS_SUCCESS"
	AgentStatusFailed      = "AGENT_STATUS_FAILED"

	TaskStatusUnspecified    = "TASK_STATUS_UNSPECIFIED"
	TaskStatusPending        = "TASK_STATUS_PENDING"
	TaskStatusScheduled      = "TASK_STATUS_SCHEDULED"
	TaskStatusRunning        = "TASK_STATUS_RUNNING"
	TaskStatusSuccess        = "TASK_STATUS_SUCCESS"
	TaskStatusFailed         = "TASK_STATUS_FAILED"
	TaskStatusWaiting        = "TASK_STATUS_WAITING"
	TaskStatusRetryScheduled = "TASK_STATUS_RETRY_SCHEDULED"
	TaskStatusCancelled      = "TASK_STATUS_CANCELLED"
	TaskStatusTimedOut       = "TASK_STATUS_TIMED_OUT"

	ToolStatusUnspecified = "TOOL_STATUS_UNSPECIFIED"
	ToolStatusRunning     = "TOOL_STATUS_RUNNING"
	ToolStatusSuccess     = "TOOL_STATUS_SUCCESS"
	ToolStatusFailed      = "TOOL_STATUS_FAILED"

	WorkflowStatusUnspecified = "WORKFLOW_STATUS_UNSPECIFIED"
	WorkflowStatusPending     = "WORKFLOW_STATUS_PENDING"
	WorkflowStatusRunning     = "WORKFLOW_STATUS_RUNNING"
	WorkflowStatusSuccess     = "WORKFLOW_STATUS_SUCCESS"
	WorkflowStatusFailed      = "WORKFLOW_STATUS_FAILED"
	WorkflowStatusWaiting     = "WORKFLOW_STATUS_WAITING"
	WorkflowStatusCanceled    = "WORKFLOW_STATUS_CANCELED"
	WorkflowStatusTimedOut    = "WORKFLOW_STATUS_TIMED_OUT"
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
	// Handle unspecified statuses
	case AgentStatusUnspecified, TaskStatusUnspecified, ToolStatusUnspecified, WorkflowStatusUnspecified:
		return StatusPending

	// Handle pending statuses
	case TaskStatusPending, WorkflowStatusPending:
		return StatusPending

	// Handle scheduled statuses
	case TaskStatusScheduled:
		return StatusScheduled

	// Handle running statuses
	case AgentStatusRunning, TaskStatusRunning, ToolStatusRunning, WorkflowStatusRunning:
		return StatusRunning

	// Handle success statuses
	case AgentStatusSuccess, TaskStatusSuccess, ToolStatusSuccess, WorkflowStatusSuccess:
		return StatusSuccess

	// Handle failed statuses
	case AgentStatusFailed, TaskStatusFailed, ToolStatusFailed, WorkflowStatusFailed:
		return StatusFailed

	// Handle waiting statuses
	case TaskStatusWaiting, WorkflowStatusWaiting:
		return StatusWaiting

	// Handle retry scheduled statuses
	case TaskStatusRetryScheduled:
		return StatusRetryScheduled

	// Handle cancelled statuses
	case TaskStatusCancelled, WorkflowStatusCanceled:
		return StatusCancelled

	// Handle timed out statuses
	case TaskStatusTimedOut, WorkflowStatusTimedOut:
		return StatusTimedOut

	default:
		return StatusPending // Default fallback
	}
}
