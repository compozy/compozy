package pb

import "github.com/compozy/compozy/pkg/pb/common"

type Subjecter interface {
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

func GetCorrelationId(payload WithMetadata) string {
	correlationID := "unknown_correlation_id"
	if meta := payload.GetMetadata(); meta != nil && meta.GetCorrelationId() != "" {
		correlationID = meta.GetCorrelationId()
	}
	return correlationID
}

func GetWorkflowId(payload WithWorkflow) string {
	workflowID := "unknown_workflow_id"
	if workflow := payload.GetWorkflow(); workflow != nil && workflow.GetId() != "" {
		workflowID = workflow.GetId()
	}
	return workflowID
}

func GetWorkflowExecId(payload WithWorkflow) string {
	workflowExecID := "unknown_workflow_exec_id"
	if workflow := payload.GetWorkflow(); workflow != nil && workflow.GetExecId() != "" {
		workflowExecID = workflow.GetExecId()
	}
	return workflowExecID
}

func GetTaskId(payload WithTask) string {
	taskID := "unknown_task_id"
	if task := payload.GetTask(); task != nil && task.GetId() != "" {
		taskID = task.GetId()
	}
	return taskID
}

func GetTaskExecId(payload WithTask) string {
	taskExecID := "unknown_task_exec_id"
	if task := payload.GetTask(); task != nil && task.GetExecId() != "" {
		taskExecID = task.GetExecId()
	}
	return taskExecID
}

func GetAgentId(payload WithAgent) string {
	agentID := "unknown_agent_id"
	if agent := payload.GetAgent(); agent != nil && agent.GetId() != "" {
		agentID = agent.GetId()
	}
	return agentID
}

func GetAgentExecId(payload WithAgent) string {
	agentExecID := "unknown_agent_exec_id"
	if agent := payload.GetAgent(); agent != nil && agent.GetExecId() != "" {
		agentExecID = agent.GetExecId()
	}
	return agentExecID
}

func GetToolId(payload WithTool) string {
	toolID := "unknown_tool_id"
	if tool := payload.GetTool(); tool != nil && tool.GetId() != "" {
		toolID = tool.GetId()
	}
	return toolID
}

func GetToolExecId(payload WithTool) string {
	toolExecID := "unknown_tool_exec_id"
	if tool := payload.GetTool(); tool != nil && tool.GetExecId() != "" {
		toolExecID = tool.GetExecId()
	}
	return toolExecID
}

func GetSourceComponent(payload WithMetadata) string {
	sourceComponent := "unknown_source_component"
	if meta := payload.GetMetadata(); meta != nil && meta.GetSourceComponent() != "" {
		sourceComponent = meta.GetSourceComponent()
	}
	return sourceComponent
}