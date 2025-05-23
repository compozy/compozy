package test

import (
	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/engine/domain/agent"
	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/tool"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/pkg/pb"
	structpb "google.golang.org/protobuf/types/known/structpb"
	timepb "google.golang.org/protobuf/types/known/timestamppb"
)

// -----------------------------------------------------------------------------
// Common Test Data Creation
// -----------------------------------------------------------------------------

func createTaskEnv() common.EnvMap {
	return common.EnvMap{
		"TASK_KEY":     "task_val",
		"OVERRIDE_KEY": "task_override",
		"SHARED_ENV":   "from_task_env",
		"FROM_TRIGGER": "{{ .trigger.input.data.value }}",
		"FROM_INPUT":   "{{ .input.task_param }}",
		"FROM_ENV":     "{{ .env.SHARED_ENV }}",
	}
}

func createAgentEnv() common.EnvMap {
	return common.EnvMap{
		"AGENT_KEY":    "agent_val",
		"OVERRIDE_KEY": "agent_override",
		"FROM_TRIGGER": "{{ .trigger.input.data.value }}",
		"FROM_INPUT":   "{{ .input.agent_param }}",
		"FROM_ENV":     "{{ .env.SHARED_ENV }}",
	}
}

func createToolEnv() common.EnvMap {
	return common.EnvMap{
		"TOOL_KEY":     "tool_val",
		"OVERRIDE_KEY": "tool_override",
		"FROM_TRIGGER": "{{ .trigger.input.data.value }}",
		"FROM_INPUT":   "{{ .input.tool_param }}",
		"FROM_ENV":     "{{ .env.SHARED_ENV }}",
	}
}

func createWorkflowEnv() common.EnvMap {
	return common.EnvMap{
		"WORKFLOW_KEY": "workflow_val",
		"OVERRIDE_KEY": "workflow_override",
		"SHARED_ENV":   "from_workflow_env",
		"FROM_TRIGGER": "{{ .trigger.input.data.value }}",
		"FROM_INPUT":   "{{ .input.task_param }}",
		"FROM_ENV":     "{{ .env.SHARED_ENV }}",
	}
}

func createProjectEnv() common.EnvMap {
	return common.EnvMap{
		"PROJECT_ENV": "project_value",
	}
}

func createTriggerInput() common.Input {
	return common.Input{
		"data": map[string]any{
			"value": "trigger_data_value",
		},
	}
}

func createTaskInput() common.Input {
	return common.Input{
		"task_param":     "task_input_value",
		"COMMON_PARAM":   "task_common_val",
		"TEMPLATE_PARAM": "{{ .trigger.input.data.value }}",
	}
}

func createAgentInput() common.Input {
	return common.Input{
		"agent_param":          "agent_input_value",
		"COMMON_PARAM":         "agent_common_val",
		"AGENT_TEMPLATE_PARAM": "{{ .trigger.input.data.value }}",
	}
}

func createToolInput() common.Input {
	return common.Input{
		"tool_param":          "tool_input_value",
		"COMMON_PARAM":        "tool_common_val",
		"TOOL_TEMPLATE_PARAM": "{{ .trigger.input.data.value }}",
	}
}

// -----------------------------------------------------------------------------
// Context and State Creation Helpers
// -----------------------------------------------------------------------------

func CreateAgentContextAndState() (*agent.Context, *agent.State, error) {
	taskEnv := createTaskEnv()
	agentEnv := createAgentEnv()
	triggerInput := createTriggerInput()
	taskInput := createTaskInput()
	agentInput := createAgentInput()

	metadata := agent.RandomMetadata("workflow-1", "task-1", "agent-1")
	ctx, err := agent.NewContext(
		metadata,
		taskEnv,
		agentEnv,
		&triggerInput,
		&taskInput,
		&agentInput,
	)
	if err != nil {
		return nil, nil, err
	}

	state, err := agent.NewState(ctx)
	if err != nil {
		return nil, nil, err
	}

	return ctx, state, nil
}

func CreateTaskContextAndState() (*task.Context, *task.State, error) {
	taskEnv := createTaskEnv()
	workflowEnv := createWorkflowEnv()
	triggerInput := createTriggerInput()
	taskInput := createTaskInput()

	metadata := task.RandomMetadata("workflow-1", "task-1")
	ctx, err := task.NewContext(
		metadata,
		workflowEnv,
		taskEnv,
		&triggerInput,
		&taskInput,
	)
	if err != nil {
		return nil, nil, err
	}

	state, err := task.NewState(ctx)
	if err != nil {
		return nil, nil, err
	}

	return ctx, state, nil
}

func CreateToolContextAndState() (*tool.Context, *tool.State, error) {
	taskEnv := createTaskEnv()
	toolEnv := createToolEnv()
	triggerInput := createTriggerInput()
	taskInput := createTaskInput()
	toolInput := createToolInput()

	metadata := tool.RandomMetadata("workflow-1", "task-1", "tool-1")
	ctx, err := tool.NewContext(
		metadata,
		taskEnv,
		toolEnv,
		&triggerInput,
		&taskInput,
		&toolInput,
	)
	if err != nil {
		return nil, nil, err
	}

	state, err := tool.NewState(ctx)
	if err != nil {
		return nil, nil, err
	}

	return ctx, state, nil
}

func CreateWorkflowContextAndState() (*workflow.Context, *workflow.State, error) {
	projectEnv := createProjectEnv()
	workflowEnv := createWorkflowEnv()
	triggerInput := createTriggerInput()

	metadata := workflow.RandomMetadata("workflow-1")
	ctx, err := workflow.NewContext(
		metadata,
		&triggerInput,
		projectEnv,
		workflowEnv,
	)
	if err != nil {
		return nil, nil, err
	}

	state, err := workflow.NewState(ctx)
	if err != nil {
		return nil, nil, err
	}

	return ctx, state, nil
}

// -----------------------------------------------------------------------------
// Event Metadata Creation Helpers
// -----------------------------------------------------------------------------

// CreateAgentEventMetadata creates agent metadata for events with all required fields
func CreateAgentEventMetadata(
	corrID, workflowExecID, taskExecID, agentExecID, workflowStateID, taskStateID, agentStateID string,
) *pb.AgentMetadata {
	return &pb.AgentMetadata{
		Source:          "test",
		CorrelationId:   corrID,
		WorkflowId:      "workflow-id",
		WorkflowExecId:  workflowExecID,
		WorkflowStateId: workflowStateID,
		TaskId:          "task-id",
		TaskExecId:      taskExecID,
		TaskStateId:     taskStateID,
		AgentId:         "agent-id",
		AgentExecId:     agentExecID,
		AgentStateId:    agentStateID,
		Time:            timepb.Now(),
		Subject:         "",
	}
}

func CreateTaskEventMetadata(
	corrID, workflowExecID, taskExecID, workflowStateID, taskStateID string,
) *pb.TaskMetadata {
	return &pb.TaskMetadata{
		Source:          "test",
		CorrelationId:   corrID,
		WorkflowId:      "workflow-id",
		WorkflowExecId:  workflowExecID,
		WorkflowStateId: workflowStateID,
		TaskId:          "task-id",
		TaskExecId:      taskExecID,
		TaskStateId:     taskStateID,
		Time:            timepb.Now(),
		Subject:         "",
	}
}

func CreateWorkflowEventMetadata(corrID, workflowExecID, workflowStateID string) *pb.WorkflowMetadata {
	return &pb.WorkflowMetadata{
		Source:          "test",
		CorrelationId:   corrID,
		WorkflowId:      "workflow-id",
		WorkflowExecId:  workflowExecID,
		WorkflowStateId: workflowStateID,
		Time:            timepb.Now(),
		Subject:         "",
	}
}

// -----------------------------------------------------------------------------
// Event Creation Helpers
// -----------------------------------------------------------------------------

func CreateAgentStartedEvent(metadata *pb.AgentMetadata) *pb.EventAgentStarted {
	return &pb.EventAgentStarted{
		Metadata: metadata,
	}
}

func CreateAgentSuccessEvent(metadata *pb.AgentMetadata, result *structpb.Struct) *pb.EventAgentSuccess {
	return &pb.EventAgentSuccess{
		Metadata: metadata,
		Details: &pb.EventAgentSuccess_Details{
			Result: result,
		},
	}
}

func CreateAgentFailedEvent(metadata *pb.AgentMetadata, errorResult *pb.ErrorResult) *pb.EventAgentFailed {
	return &pb.EventAgentFailed{
		Metadata: metadata,
		Details: &pb.EventAgentFailed_Details{
			Error: errorResult,
		},
	}
}

func CreateTaskStartedEvent(metadata *pb.TaskMetadata) *pb.EventTaskStarted {
	return &pb.EventTaskStarted{
		Metadata: metadata,
	}
}

func CreateTaskSuccessEvent(metadata *pb.TaskMetadata, result *structpb.Struct) *pb.EventTaskSuccess {
	return &pb.EventTaskSuccess{
		Metadata: metadata,
		Details: &pb.EventTaskSuccess_Details{
			Result: result,
		},
	}
}

func CreateTaskFailedEvent(metadata *pb.TaskMetadata, errorResult *pb.ErrorResult) *pb.EventTaskFailed {
	return &pb.EventTaskFailed{
		Metadata: metadata,
		Details: &pb.EventTaskFailed_Details{
			Error: errorResult,
		},
	}
}

// -----------------------------------------------------------------------------
// Result Data Creation Helpers
// -----------------------------------------------------------------------------

func CreateSuccessResult(message string, count int, additionalData map[string]any) (*structpb.Struct, error) {
	data := map[string]interface{}{
		"message": message,
		"count":   count,
	}
	for k, v := range additionalData {
		data[k] = v
	}
	return structpb.NewStruct(data)
}

func CreateErrorResult(message, code string, details map[string]any) (*pb.ErrorResult, error) {
	errorDetails, err := structpb.NewStruct(details)
	if err != nil {
		return nil, err
	}
	return &pb.ErrorResult{
		Message: message,
		Code:    &code,
		Details: errorDetails,
	}, nil
}
