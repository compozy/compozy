package pb

import (
	"github.com/compozy/compozy/pkg/pb/common"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Subjecter interface {
	protoreflect.ProtoMessage
	ToSubject() string
}

type WithMetadata interface {
	GetMetadata() *common.Metadata
}

type WithWorkflow interface {
	GetWorkflow() *common.WorkflowInfo
}

type WithTask interface {
	GetTask() *common.TaskInfo
}

type WithAgent interface {
	GetAgent() *common.AgentInfo
}

type WithTool interface {
	GetTool() *common.ToolInfo
}

func GetCorrelationID(payload WithMetadata) string {
	corrID := "unknown_correlation_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetCorrelationId() != "" {
		corrID = meta.GetCorrelationId()
	}
	return corrID
}

func GetWorkflowID(payload WithWorkflow) string {
	workflowID := "unknown_workflow_id"
	if workflow := payload.GetWorkflow(); workflow != nil && workflow.GetId() != "" {
		workflowID = workflow.GetId()
	}
	return workflowID
}

func GetWorkflowExecID(payload WithWorkflow) string {
	wExecID := "unknown_workflow_exec_id"
	if workflow := payload.GetWorkflow(); workflow != nil && workflow.GetExecId() != "" {
		wExecID = workflow.GetExecId()
	}
	return wExecID
}

func GetTaskID(payload WithTask) string {
	taskID := "unknown_task_id"
	if task := payload.GetTask(); task != nil && task.GetId() != "" {
		taskID = task.GetId()
	}
	return taskID
}

func GetTaskExecID(payload WithTask) string {
	tExecID := "unknown_task_exec_id"
	if task := payload.GetTask(); task != nil && task.GetExecId() != "" {
		tExecID = task.GetExecId()
	}
	return tExecID
}

func GetAgentID(payload WithAgent) string {
	agentID := "unknown_agent_id"
	if agent := payload.GetAgent(); agent != nil && agent.GetId() != "" {
		agentID = agent.GetId()
	}
	return agentID
}

func GetAgentExecID(payload WithAgent) string {
	agentExecID := "unknown_agent_exec_id"
	if agent := payload.GetAgent(); agent != nil && agent.GetExecId() != "" {
		agentExecID = agent.GetExecId()
	}
	return agentExecID
}

func GetToolID(payload WithTool) string {
	toolID := "unknown_tool_id"
	if tool := payload.GetTool(); tool != nil && tool.GetId() != "" {
		toolID = tool.GetId()
	}
	return toolID
}

func GetToolExecID(payload WithTool) string {
	toolExecID := "unknown_tool_exec_id"
	if tool := payload.GetTool(); tool != nil && tool.GetExecId() != "" {
		toolExecID = tool.GetExecId()
	}
	return toolExecID
}

func GetSourceComponent(payload WithMetadata) string {
	sourceComponent := "unknown_source_component"
	if meta := payload.GetMetadata(); meta != nil && meta.GetSource() != "" {
		sourceComponent = meta.GetSource()
	}
	return sourceComponent
}
