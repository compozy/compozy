package utils

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// -----------------------------------------------------------------------------
// Workflow Helpers
// -----------------------------------------------------------------------------

// CreateTestWorkflowConfig creates a test workflow configuration
func CreateTestWorkflowConfig(
	t *testing.T,
	tb *IntegrationTestBed,
	workflowID string,
	env core.EnvMap,
) *workflow.Config {
	t.Helper()
	workflowConfig := &workflow.Config{
		ID:      workflowID,
		Version: "1.0.0",
		Opts: workflow.Opts{
			Env: env,
		},
	}
	err := workflowConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)
	return workflowConfig
}

// CreateTestWorkflowWithTasks creates a workflow config with tasks
func CreateTestWorkflowWithTasks(
	t *testing.T,
	tb *IntegrationTestBed,
	workflowID string,
	tasks []task.Config,
) *workflow.Config {
	t.Helper()
	workflowConfig := CreateTestWorkflowConfig(t, tb, workflowID, core.EnvMap{})
	workflowConfig.Tasks = tasks
	return workflowConfig
}

// CreateTestWorkflowExecution creates a workflow execution and returns its ID
func CreateTestWorkflowExecution(
	t *testing.T,
	tb *IntegrationTestBed,
	workflowID string,
	env core.EnvMap,
	input *core.Input,
) core.ID {
	t.Helper()
	workflowExecID := core.MustNewID()
	workflowMetadata := &pb.WorkflowMetadata{
		WorkflowId:     workflowID,
		WorkflowExecId: string(workflowExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}
	workflowConfig := CreateTestWorkflowConfig(t, tb, workflowID, env)
	_, err := tb.WorkflowRepo.CreateExecution(
		tb.Ctx,
		workflowMetadata,
		workflowConfig,
		input,
	)
	require.NoError(t, err)
	return workflowExecID
}

// -----------------------------------------------------------------------------
// Task Helpers
// -----------------------------------------------------------------------------

// CreateTestTaskConfig creates a test task configuration
func CreateTestTaskConfig(
	t *testing.T,
	taskID string,
	taskType task.Type,
	action string,
	env core.EnvMap,
) *task.Config {
	t.Helper()
	return &task.Config{
		ID:     taskID,
		Type:   taskType,
		Action: action,
		Env:    env,
	}
}

// CreateTestBasicTaskConfig creates a basic task configuration
func CreateTestBasicTaskConfig(t *testing.T, taskID string, action string) *task.Config {
	t.Helper()
	return CreateTestTaskConfig(t, taskID, task.TaskTypeBasic, action, core.EnvMap{})
}

// CreateTestDecisionTaskConfig creates a decision task configuration
func CreateTestDecisionTaskConfig(
	t *testing.T,
	taskID string,
	condition string,
	routes map[string]string,
) *task.Config {
	t.Helper()
	return &task.Config{
		ID:        taskID,
		Type:      task.TaskTypeDecision,
		Condition: condition,
		Routes:    routes,
		Env:       core.EnvMap{},
	}
}

// CreateTestTaskExecution creates a task execution and returns its ID and execution
func CreateTestTaskExecution(
	t *testing.T,
	tb *IntegrationTestBed,
	workflowExecID core.ID,
	taskID string,
	taskConfig *task.Config,
) (core.ID, *task.Execution) {
	t.Helper()

	// Get the workflow execution to extract the correct workflow ID
	workflowExecution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
	require.NoError(t, err)

	taskExecID := core.MustNewID()
	taskMetadata := &pb.TaskMetadata{
		WorkflowId:     workflowExecution.WorkflowID,
		WorkflowExecId: string(workflowExecID),
		TaskId:         taskID,
		TaskExecId:     string(taskExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}
	err = taskConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)
	execution, err := tb.TaskRepo.CreateExecution(
		tb.Ctx,
		taskMetadata,
		taskConfig,
	)
	require.NoError(t, err)
	require.NotNil(t, execution)
	return taskExecID, execution
}

// -----------------------------------------------------------------------------
// Agent Helpers
// -----------------------------------------------------------------------------

// CreateTestAgentConfig creates a test agent configuration
func CreateTestAgentConfig(t *testing.T, agentID string, instructions string, env core.EnvMap) *agent.Config {
	t.Helper()
	return &agent.Config{
		ID:           agentID,
		Config:       agent.ProviderConfig{},
		Instructions: instructions,
		Env:          env,
	}
}

// CreateTestAgentExecution creates an agent execution and returns its ID and execution
func CreateTestAgentExecution(
	t *testing.T,
	tb *IntegrationTestBed,
	taskExecID core.ID,
	agentID string,
	agentConfig *agent.Config,
) (core.ID, *agent.Execution) {
	t.Helper()

	// Get the task execution to extract the correct workflow and task IDs
	taskExecution, err := tb.TaskRepo.GetExecution(tb.Ctx, taskExecID)
	require.NoError(t, err)

	agentExecID := core.MustNewID()
	agentMetadata := &pb.AgentMetadata{
		WorkflowId:     taskExecution.WorkflowID,
		WorkflowExecId: string(taskExecution.WorkflowExecID),
		TaskId:         taskExecution.TaskID,
		TaskExecId:     string(taskExecID),
		AgentId:        agentID,
		AgentExecId:    string(agentExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}
	err = agentConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)
	execution, err := tb.AgentRepo.CreateExecution(
		tb.Ctx,
		agentMetadata,
		agentConfig,
	)
	require.NoError(t, err)
	require.NotNil(t, execution)
	return agentExecID, execution
}

// -----------------------------------------------------------------------------
// Tool Helpers
// -----------------------------------------------------------------------------

// CreateTestToolConfig creates a test tool configuration
func CreateTestToolConfig(t *testing.T, toolID string, execute string, env core.EnvMap) *tool.Config {
	t.Helper()
	return &tool.Config{
		ID:      toolID,
		Execute: execute,
		Env:     env,
	}
}

// CreateTestToolExecution creates a tool execution and returns its ID and execution
func CreateTestToolExecution(
	t *testing.T,
	tb *IntegrationTestBed,
	taskExecID core.ID,
	toolID string,
	toolConfig *tool.Config,
) (core.ID, *tool.Execution) {
	t.Helper()

	// Get the task execution to extract the correct workflow and task IDs
	taskExecution, err := tb.TaskRepo.GetExecution(tb.Ctx, taskExecID)
	require.NoError(t, err)

	toolExecID := core.MustNewID()
	toolMetadata := &pb.ToolMetadata{
		WorkflowId:     taskExecution.WorkflowID,
		WorkflowExecId: string(taskExecution.WorkflowExecID),
		TaskId:         taskExecution.TaskID,
		TaskExecId:     string(taskExecID),
		ToolId:         toolID,
		ToolExecId:     string(toolExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}
	err = toolConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)
	execution, err := tb.ToolRepo.CreateExecution(
		tb.Ctx,
		toolMetadata,
		toolConfig,
	)
	require.NoError(t, err)
	require.NotNil(t, execution)
	return toolExecID, execution
}

// -----------------------------------------------------------------------------
// Composite Helpers
// -----------------------------------------------------------------------------

// CreateTestWorkflowWithTasksAndExecutions creates a complete test scenario with workflow, tasks, and executions
func CreateTestWorkflowWithTasksAndExecutions(
	t *testing.T,
	tb *IntegrationTestBed,
	workflowID string,
) (*workflow.Config, core.ID, []task.Config, []core.ID) {
	t.Helper()

	// Create task configs
	tasks := []task.Config{
		*CreateTestBasicTaskConfig(t, "format-code", "format"),
		*CreateTestBasicTaskConfig(t, "lint-code", "lint"),
		*CreateTestDecisionTaskConfig(t, "check-quality", "quality > 80", map[string]string{
			"true":  "deploy",
			"false": "fix-issues",
		}),
	}

	// Create workflow config with tasks
	workflowConfig := CreateTestWorkflowWithTasks(t, tb, workflowID, tasks)

	// Create workflow execution
	workflowExecID := CreateTestWorkflowExecution(
		t, tb, workflowID,
		core.EnvMap{"ENV": "test"},
		&core.Input{"code": "console.log('hello')", "language": "javascript"},
	)

	// Create task executions
	var taskExecIDs []core.ID
	for i := range tasks {
		taskExecID, _ := CreateTestTaskExecution(
			t,
			tb,
			workflowExecID,
			tasks[i].ID,
			&tasks[i],
		)
		taskExecIDs = append(taskExecIDs, taskExecID)
	}

	return workflowConfig, workflowExecID, tasks, taskExecIDs
}
