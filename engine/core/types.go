package core

import (
	"os"
	"path/filepath"
)

func GetVersion() string {
	if version := os.Getenv("COMPOZY_VERSION"); version != "" {
		return version
	}
	return "v0"
}

func GetStoreDir(cwd string) string {
	if cwd == "" {
		return ".compozy"
	}
	return filepath.Join(cwd, ".compozy")
}

// -----------------------------------------------------------------------------
// Component Type
// -----------------------------------------------------------------------------

type ComponentType string

const (
	ComponentWorkflow ComponentType = "workflow"
	ComponentTask     ComponentType = "task"
	ComponentAgent    ComponentType = "agent"
	ComponentTool     ComponentType = "tool"
	ComponentLog      ComponentType = "log"
)

// -----------------------------------------------------------------------------
// Stream Name
// -----------------------------------------------------------------------------

type StreamName string

const (
	StreamCommands StreamName = "COMMANDS"
	StreamEvents   StreamName = "EVENTS"
	StreamLogs     StreamName = "LOGS"
)

// -----------------------------------------------------------------------------
// Commands
// -----------------------------------------------------------------------------

type CmdType string

const (
	CmdTrigger  CmdType = "trigger"
	CmdDispatch CmdType = "dispatch"
	CmdExecute  CmdType = "execute"
	CmdCancel   CmdType = "cancel"
	CmdPause    CmdType = "pause"
	CmdResume   CmdType = "resume"
	CmdAll      CmdType = "*"
)

func (c CmdType) String() string {
	return string(c)
}

// -----------------------------------------------------------------------------
// Events
// -----------------------------------------------------------------------------

type EvtType string

const (
	EvtDispatched      EvtType = "dispatched"
	EvtStarted         EvtType = "started"
	EvtPaused          EvtType = "paused"
	EvtResumed         EvtType = "resumed"
	EvtSuccess         EvtType = "success"
	EvtFailed          EvtType = "failed"
	EvtCanceled        EvtType = "canceled"
	EvtTimedOut        EvtType = "timed_out"
	EvtWaitingStarted  EvtType = "waiting_started"
	EvtWaitingEnded    EvtType = "waiting_ended"
	EvtWaitingTimedOut EvtType = "waiting_timed_out"
	EvtAll             EvtType = "*"
)

func (e EvtType) String() string {
	return string(e)
}

// -----------------------------------------------------------------------------
// Subject
// -----------------------------------------------------------------------------

type SubjectSegmentType string

const (
	SegmentCmd   SubjectSegmentType = "cmd"
	SegmentEvent SubjectSegmentType = "evt"
	SegmentLog   SubjectSegmentType = "log"
)

// -----------------------------------------------------------------------------
// Event Status
// -----------------------------------------------------------------------------

type StatusType string

const (
	StatusPending  StatusType = "PENDING"
	StatusRunning  StatusType = "RUNNING"
	StatusSuccess  StatusType = "SUCCESS"
	StatusFailed   StatusType = "FAILED"
	StatusTimedOut StatusType = "TIMED_OUT"
	StatusCanceled StatusType = "CANCELED"
	StatusWaiting  StatusType = "WAITING"
	StatusPaused   StatusType = "PAUSED"
)

type ProtoStatusType string

const (
	AgentStatusUnspecified ProtoStatusType = "AGENT_STATUS_UNSPECIFIED"
	AgentStatusRunning     ProtoStatusType = "AGENT_STATUS_RUNNING"
	AgentStatusSuccess     ProtoStatusType = "AGENT_STATUS_SUCCESS"
	AgentStatusFailed      ProtoStatusType = "AGENT_STATUS_FAILED"

	TaskStatusUnspecified ProtoStatusType = "TASK_STATUS_UNSPECIFIED"
	TaskStatusPending     ProtoStatusType = "TASK_STATUS_PENDING"
	TaskStatusRunning     ProtoStatusType = "TASK_STATUS_RUNNING"
	TaskStatusSuccess     ProtoStatusType = "TASK_STATUS_SUCCESS"
	TaskStatusFailed      ProtoStatusType = "TASK_STATUS_FAILED"
	TaskStatusWaiting     ProtoStatusType = "TASK_STATUS_WAITING"
	TaskStatusCanceled    ProtoStatusType = "TASK_STATUS_CANCELED"
	TaskStatusTimedOut    ProtoStatusType = "TASK_STATUS_TIMED_OUT"

	ToolStatusUnspecified ProtoStatusType = "TOOL_STATUS_UNSPECIFIED"
	ToolStatusRunning     ProtoStatusType = "TOOL_STATUS_RUNNING"
	ToolStatusSuccess     ProtoStatusType = "TOOL_STATUS_SUCCESS"
	ToolStatusFailed      ProtoStatusType = "TOOL_STATUS_FAILED"

	WorkflowStatusUnspecified ProtoStatusType = "WORKFLOW_STATUS_UNSPECIFIED"
	WorkflowStatusPending     ProtoStatusType = "WORKFLOW_STATUS_PENDING"
	WorkflowStatusRunning     ProtoStatusType = "WORKFLOW_STATUS_RUNNING"
	WorkflowStatusSuccess     ProtoStatusType = "WORKFLOW_STATUS_SUCCESS"
	WorkflowStatusFailed      ProtoStatusType = "WORKFLOW_STATUS_FAILED"
	WorkflowStatusPaused      ProtoStatusType = "WORKFLOW_STATUS_PAUSED"
	WorkflowStatusCanceled    ProtoStatusType = "WORKFLOW_STATUS_CANCELED"
	WorkflowStatusTimedOut    ProtoStatusType = "WORKFLOW_STATUS_TIMED_OUT"
)

func (s ProtoStatusType) String() string {
	return string(s)
}

type ProtoStatus interface {
	String() string
	Number() int32
}

func ToStatus(status string) StatusType {
	switch status {
	case AgentStatusUnspecified.String(),
		TaskStatusUnspecified.String(),
		ToolStatusUnspecified.String(),
		WorkflowStatusUnspecified.String():
		return StatusPending
	case TaskStatusPending.String(),
		WorkflowStatusPending.String():
		return StatusPending
	case AgentStatusRunning.String(),
		TaskStatusRunning.String(),
		ToolStatusRunning.String(),
		WorkflowStatusRunning.String():
		return StatusRunning
	case AgentStatusSuccess.String(),
		TaskStatusSuccess.String(),
		ToolStatusSuccess.String(),
		WorkflowStatusSuccess.String():
		return StatusSuccess
	case AgentStatusFailed.String(),
		TaskStatusFailed.String(),
		ToolStatusFailed.String(),
		WorkflowStatusFailed.String():
		return StatusFailed
	case TaskStatusWaiting.String():
		return StatusWaiting
	case WorkflowStatusPaused.String():
		return StatusPaused
	case TaskStatusCanceled.String(),
		WorkflowStatusCanceled.String():
		return StatusCanceled
	case TaskStatusTimedOut.String(),
		WorkflowStatusTimedOut.String():
		return StatusTimedOut
	default:
		return StatusPending // Default fallback
	}
}

// -----------------------------------------------------------------------------
// Sources
// -----------------------------------------------------------------------------

type SourceType string

const (
	SourceWorker          SourceType = "worker.Worker"
	SourceWorkflowExecute SourceType = "workflow.HandleExecute"
	SourceTaskExecute     SourceType = "task.HandleExecute"
)

func (s SourceType) String() string {
	return string(s)
}
