package pb

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Subjecter interface {
	protoreflect.ProtoMessage
	ToSubject() string
}

// -----------------------------------------------------------------------------
// Metadata Interfaces
// -----------------------------------------------------------------------------

type WithWorkflowMetadata interface {
	GetMetadata() *WorkflowMetadata
}

type WithTaskMetadata interface {
	GetMetadata() *TaskMetadata
}

type WithAgentMetadata interface {
	GetMetadata() *AgentMetadata
}

type WithToolMetadata interface {
	GetMetadata() *ToolMetadata
}

type WithLogMetadata interface {
	GetMetadata() *LogMetadata
}

// -----------------------------------------------------------------------------
// Correlation ID Helpers
// -----------------------------------------------------------------------------

func GetCorrelationID(payload any) string {
	corrID := "unknown_correlation_id"

	switch p := payload.(type) {
	case WithWorkflowMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetCorrelationId() != "" {
			corrID = meta.GetCorrelationId()
		}
	case WithTaskMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetCorrelationId() != "" {
			corrID = meta.GetCorrelationId()
		}
	case WithAgentMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetCorrelationId() != "" {
			corrID = meta.GetCorrelationId()
		}
	case WithToolMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetCorrelationId() != "" {
			corrID = meta.GetCorrelationId()
		}
	case WithLogMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetCorrelationId() != "" {
			corrID = meta.GetCorrelationId()
		}
	}

	return corrID
}

// -----------------------------------------------------------------------------
// Workflow Helpers
// -----------------------------------------------------------------------------

func GetWorkflowID(payload WithWorkflowMetadata) string {
	workflowID := "unknown_workflow_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetWorkflowId() != "" {
		workflowID = meta.GetWorkflowId()
	}
	return workflowID
}

func GetWorkflowExecID(payload WithWorkflowMetadata) string {
	wExecID := "unknown_workflow_exec_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetWorkflowExecId() != "" {
		wExecID = meta.GetWorkflowExecId()
	}
	return wExecID
}

// -----------------------------------------------------------------------------
// Task Helpers
// -----------------------------------------------------------------------------

func GetTaskID(payload WithTaskMetadata) string {
	taskID := "unknown_task_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetTaskId() != "" {
		taskID = meta.GetTaskId()
	}
	return taskID
}

func GetTaskExecID(payload WithTaskMetadata) string {
	tExecID := "unknown_task_exec_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetTaskExecId() != "" {
		tExecID = meta.GetTaskExecId()
	}
	return tExecID
}

// -----------------------------------------------------------------------------
// Agent Helpers
// -----------------------------------------------------------------------------

func GetAgentID(payload WithAgentMetadata) string {
	agentID := "unknown_agent_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetAgentId() != "" {
		agentID = meta.GetAgentId()
	}
	return agentID
}

func GetAgentExecID(payload WithAgentMetadata) string {
	agentExecID := "unknown_agent_exec_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetAgentExecId() != "" {
		agentExecID = meta.GetAgentExecId()
	}
	return agentExecID
}

// -----------------------------------------------------------------------------
// Tool Helpers
// -----------------------------------------------------------------------------

func GetToolID(payload WithToolMetadata) string {
	toolID := "unknown_tool_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetToolId() != "" {
		toolID = meta.GetToolId()
	}
	return toolID
}

func GetToolExecID(payload WithToolMetadata) string {
	toolExecID := "unknown_tool_exec_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetToolExecId() != "" {
		toolExecID = meta.GetToolExecId()
	}
	return toolExecID
}

// -----------------------------------------------------------------------------
// Source Helpers
// -----------------------------------------------------------------------------

func GetSourceComponent(payload any) string {
	sourceComponent := "unknown_source_component"

	switch p := payload.(type) {
	case WithWorkflowMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetSource() != "" {
			sourceComponent = meta.GetSource()
		}
	case WithTaskMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetSource() != "" {
			sourceComponent = meta.GetSource()
		}
	case WithAgentMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetSource() != "" {
			sourceComponent = meta.GetSource()
		}
	case WithToolMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetSource() != "" {
			sourceComponent = meta.GetSource()
		}
	case WithLogMetadata:
		if meta := p.GetMetadata(); meta != nil && meta.GetSource() != "" {
			sourceComponent = meta.GetSource()
		}
	}

	return sourceComponent
}
