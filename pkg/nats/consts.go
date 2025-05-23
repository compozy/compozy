package nats

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
// Event Status
// -----------------------------------------------------------------------------

type EvStatusType string

const (
	StatusPending  EvStatusType = "PENDING"
	StatusRunning  EvStatusType = "RUNNING"
	StatusSuccess  EvStatusType = "SUCCESS"
	StatusFailed   EvStatusType = "FAILED"
	StatusTimedOut EvStatusType = "TIMED_OUT"
	StatusCanceled EvStatusType = "CANCELED"
	StatusWaiting  EvStatusType = "WAITING"
	StatusPaused   EvStatusType = "PAUSED"
)

const (
	AgentStatusUnspecified = "AGENT_STATUS_UNSPECIFIED"
	AgentStatusRunning     = "AGENT_STATUS_RUNNING"
	AgentStatusSuccess     = "AGENT_STATUS_SUCCESS"
	AgentStatusFailed      = "AGENT_STATUS_FAILED"

	TaskStatusUnspecified = "TASK_STATUS_UNSPECIFIED"
	TaskStatusPending     = "TASK_STATUS_PENDING"
	TaskStatusRunning     = "TASK_STATUS_RUNNING"
	TaskStatusSuccess     = "TASK_STATUS_SUCCESS"
	TaskStatusFailed      = "TASK_STATUS_FAILED"
	TaskStatusWaiting     = "TASK_STATUS_WAITING"
	TaskStatusCanceled    = "TASK_STATUS_CANCELED"
	TaskStatusTimedOut    = "TASK_STATUS_TIMED_OUT"

	ToolStatusUnspecified = "TOOL_STATUS_UNSPECIFIED"
	ToolStatusRunning     = "TOOL_STATUS_RUNNING"
	ToolStatusSuccess     = "TOOL_STATUS_SUCCESS"
	ToolStatusFailed      = "TOOL_STATUS_FAILED"

	WorkflowStatusUnspecified = "WORKFLOW_STATUS_UNSPECIFIED"
	WorkflowStatusPending     = "WORKFLOW_STATUS_PENDING"
	WorkflowStatusRunning     = "WORKFLOW_STATUS_RUNNING"
	WorkflowStatusSuccess     = "WORKFLOW_STATUS_SUCCESS"
	WorkflowStatusFailed      = "WORKFLOW_STATUS_FAILED"
	WorkflowStatusPaused      = "WORKFLOW_STATUS_PAUSED"
	WorkflowStatusCanceled    = "WORKFLOW_STATUS_CANCELED"
	WorkflowStatusTimedOut    = "WORKFLOW_STATUS_TIMED_OUT"
)

// -----------------------------------------------------------------------------
// Stream Name
// -----------------------------------------------------------------------------

type StreamName string

const (
	StreamWorkflowCmds StreamName = "WORKFLOW_COMMANDS"
	StreamTaskCmds     StreamName = "TASK_COMMANDS"
	StreamAgentCmds    StreamName = "AGENT_COMMANDS"
	StreamToolCmds     StreamName = "TOOL_COMMANDS"
	StreamEvents       StreamName = "EVENTS"
	StreamLogs         StreamName = "LOGS"
)

// -----------------------------------------------------------------------------
// Commands
// -----------------------------------------------------------------------------

type CmdType string

const (
	CmdTypeTrigger  CmdType = "trigger"
	CmdTypeDispatch CmdType = "dispatch"
	CmdTypeExecute  CmdType = "execute"
	CmdTypeCancel   CmdType = "cancel"
	CmdTypePause    CmdType = "pause"
	CmdTypeResume   CmdType = "resume"
	CmdTypeAll      CmdType = "*"
)

func (c CmdType) String() string {
	return string(c)
}

// -----------------------------------------------------------------------------
// Events
// -----------------------------------------------------------------------------

type EvtType string

const (
	EvtTypeDispatched      EvtType = "dispatched"
	EvtTypeStarted         EvtType = "started"
	EvtTypePaused          EvtType = "paused"
	EvtTypeResumed         EvtType = "resumed"
	EvtTypeSuccess         EvtType = "success"
	EvtTypeFailed          EvtType = "failed"
	EvtTypeCanceled        EvtType = "canceled"
	EvtTypeTimedOut        EvtType = "timed_out"
	EvtTypeWaitingStarted  EvtType = "waiting_started"
	EvtTypeWaitingEnded    EvtType = "waiting_ended"
	EvtTypeWaitingTimedOut EvtType = "waiting_timed_out"
	EvtTypeAll             EvtType = "*"
)

func (e EvtType) String() string {
	return string(e)
}
