package repo

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupTaskRepoTestBed(t *testing.T) *utils.IntegrationTestBed {
	t.Helper()
	componentsToWatch := []core.ComponentType{
		core.ComponentWorkflow,
		core.ComponentTask,
	}
	return utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout, componentsToWatch)
}

func createTestWorkflowExecution(
	t *testing.T,
	tb *utils.IntegrationTestBed,
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
	workflowConfig := &workflow.Config{
		ID:      workflowID,
		Version: "1.0.0",
		Env:     env,
	}
	err := workflowConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)
	_, err = tb.WorkflowRepo.CreateExecution(tb.Ctx, workflowMetadata, workflowConfig, input)
	require.NoError(t, err)
	return workflowExecID
}

func createTestTaskExecution(
	t *testing.T,
	tb *utils.IntegrationTestBed,
	workflowExecID core.ID,
	taskID string,
	taskConfig *task.Config,
) (core.ID, *task.Execution) {
	t.Helper()
	taskExecID := core.MustNewID()
	taskMetadata := &pb.TaskMetadata{
		WorkflowId:     "test-workflow",
		WorkflowExecId: string(workflowExecID),
		TaskId:         taskID,
		TaskExecId:     string(taskExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}
	err := taskConfig.SetCWD(tb.StateDir)
	require.NoError(t, err)
	execution, err := tb.TaskRepo.CreateExecution(tb.Ctx, taskMetadata, taskConfig)
	require.NoError(t, err)
	require.NotNil(t, execution)
	return taskExecID, execution
}

func TestTaskRepository_CreateExecution(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should create task execution successfully", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task config
		taskConfig := &task.Config{
			ID:     "format-code",
			Type:   "basic",
			Action: "format",
			Env:    core.EnvMap{"TEST_VAR": "test_value"},
		}

		// Create task execution
		taskExecID, execution := createTestTaskExecution(t, tb, workflowExecID, "format-code", taskConfig)

		// Verify execution properties
		assert.Equal(t, taskExecID, execution.GetID())
		assert.Equal(t, "format-code", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.Equal(t, workflowExecID, execution.GetWorkflowExecID())
		assert.NotNil(t, execution.GetEnv())
		assert.Equal(t, "test_value", execution.GetEnv().Prop("TEST_VAR"))
		assert.Equal(t, "workflow_value", execution.GetEnv().Prop("WORKFLOW_VAR"))
	})

	t.Run("Should handle execution creation with empty env", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task config with empty env
		taskConfig := &task.Config{
			ID:     "simple-task",
			Type:   "basic",
			Action: "simple",
			Env:    core.EnvMap{},
		}

		// Create task execution
		taskExecID, execution := createTestTaskExecution(t, tb, workflowExecID, "simple-task", taskConfig)

		// Verify execution properties
		assert.Equal(t, taskExecID, execution.GetID())
		assert.Equal(t, "simple-task", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
	})
}

func TestTaskRepository_LoadExecution(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task config
		taskConfig := &task.Config{
			ID:     "format-code",
			Type:   "basic",
			Action: "format",
			Env:    core.EnvMap{"TEST_VAR": "test_value"},
		}

		// Create task execution
		taskExecID, createdExecution := createTestTaskExecution(t, tb, workflowExecID, "format-code", taskConfig)

		// Load the execution
		loadedExecution, err := tb.TaskRepo.LoadExecution(tb.Ctx, workflowExecID, taskExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)

		// Verify loaded execution matches created execution
		assert.Equal(t, createdExecution.GetID(), loadedExecution.GetID())
		assert.Equal(t, createdExecution.GetComponentID(), loadedExecution.GetComponentID())
		assert.Equal(t, createdExecution.GetStatus(), loadedExecution.GetStatus())
		assert.Equal(t, createdExecution.GetWorkflowExecID(), loadedExecution.GetWorkflowExecID())
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()
		nonExistentTaskExecID := core.MustNewID()

		execution, err := tb.TaskRepo.LoadExecution(tb.Ctx, nonExistentWorkflowExecID, nonExistentTaskExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestTaskRepository_LoadExecutionsJSON(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load executions JSON for existing workflow execution", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create multiple task executions for the same workflow execution
		taskConfig1 := &task.Config{
			ID:     "task-1",
			Type:   "basic",
			Action: "action1",
			Env:    core.EnvMap{},
		}
		taskExecID1, _ := createTestTaskExecution(t, tb, workflowExecID, "task-1", taskConfig1)

		taskConfig2 := &task.Config{
			ID:     "task-2",
			Type:   "basic",
			Action: "action2",
			Env:    core.EnvMap{},
		}
		taskExecID2, _ := createTestTaskExecution(t, tb, workflowExecID, "task-2", taskConfig2)

		// Load executions JSON
		executionsJSON, err := tb.TaskRepo.LoadExecutionsJSON(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, executionsJSON)

		// Verify we have both executions
		assert.Len(t, executionsJSON, 2)

		// Verify execution data
		exec1, exists := executionsJSON[taskExecID1]
		assert.True(t, exists)
		assert.Equal(t, "task-1", exec1.ComponentID)
		assert.Equal(t, taskExecID1, exec1.ExecID)
		assert.Equal(t, string(core.StatusPending), exec1.Status)

		exec2, exists := executionsJSON[taskExecID2]
		assert.True(t, exists)
		assert.Equal(t, "task-2", exec2.ComponentID)
		assert.Equal(t, taskExecID2, exec2.ExecID)
		assert.Equal(t, string(core.StatusPending), exec2.Status)
	})

	t.Run("Should return empty map for workflow execution with no tasks", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()

		executionsJSON, err := tb.TaskRepo.LoadExecutionsJSON(tb.Ctx, nonExistentWorkflowExecID)
		require.NoError(t, err)
		assert.Empty(t, executionsJSON)
	})

	t.Run("Should handle workflow execution with tasks from different workflows", func(t *testing.T) {
		// Create first workflow execution
		workflowExecID1 := createTestWorkflowExecution(
			t, tb, "test-workflow-1",
			core.EnvMap{},
			&core.Input{},
		)

		// Create second workflow execution
		workflowExecID2 := createTestWorkflowExecution(
			t, tb, "test-workflow-2",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task for first workflow
		taskConfig1 := &task.Config{
			ID:     "task-1",
			Type:   "basic",
			Action: "action1",
			Env:    core.EnvMap{},
		}
		taskExecID1, _ := createTestTaskExecution(t, tb, workflowExecID1, "task-1", taskConfig1)

		// Create task for second workflow
		taskConfig2 := &task.Config{
			ID:     "task-2",
			Type:   "basic",
			Action: "action2",
			Env:    core.EnvMap{},
		}
		taskExecID2, _ := createTestTaskExecution(t, tb, workflowExecID2, "task-2", taskConfig2)

		// Load executions for first workflow only
		executionsJSON1, err := tb.TaskRepo.LoadExecutionsJSON(tb.Ctx, workflowExecID1)
		require.NoError(t, err)
		assert.Len(t, executionsJSON1, 1)
		assert.Contains(t, executionsJSON1, taskExecID1)
		assert.NotContains(t, executionsJSON1, taskExecID2)

		// Load executions for second workflow only
		executionsJSON2, err := tb.TaskRepo.LoadExecutionsJSON(tb.Ctx, workflowExecID2)
		require.NoError(t, err)
		assert.Len(t, executionsJSON2, 1)
		assert.Contains(t, executionsJSON2, taskExecID2)
		assert.NotContains(t, executionsJSON2, taskExecID1)
	})
}

func TestTaskRepository_CreateExecution_TemplateNormalization(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should parse templates in task input during execution creation", func(t *testing.T) {
		// Create workflow execution with input data
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{
				"user_name": "John Doe",
				"user_id":   123,
			},
		)

		// Create test task config with template input
		taskConfig := &task.Config{
			ID:     "template-task",
			Type:   "basic",
			Action: "process",
			With: &core.Input{
				"greeting":     "Hello, {{ .trigger.input.user_name }}!",
				"user_id":      "{{ .trigger.input.user_id }}",
				"env_message":  "Environment: {{ .env.WORKFLOW_VAR }}",
				"static_value": "no template here",
			},
			Env: core.EnvMap{
				"TASK_VAR":    "task_value",
				"DYNAMIC_VAR": "{{ .trigger.input.user_name }}_processed",
			},
		}

		// Create execution
		_, execution := createTestTaskExecution(t, tb, workflowExecID, "template-task", taskConfig)

		// Verify templates were parsed in input
		input := execution.GetInput()
		assert.Equal(t, "Hello, John Doe!", input.Prop("greeting"))
		assert.Equal(t, "123", input.Prop("user_id")) // Numbers become strings in templates
		assert.Equal(t, "Environment: workflow_value", input.Prop("env_message"))
		assert.Equal(t, "no template here", input.Prop("static_value"))

		// Verify templates were parsed in environment
		env := execution.GetEnv()
		assert.Equal(t, "task_value", env.Prop("TASK_VAR"))
		assert.Equal(t, "John Doe_processed", env.Prop("DYNAMIC_VAR"))
		assert.Equal(t, "workflow_value", env.Prop("WORKFLOW_VAR")) // From workflow
	})

	t.Run("Should handle nested templates in task input", func(t *testing.T) {
		// Create workflow execution with nested input data
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"API_BASE": "https://api.example.com"},
			&core.Input{
				"user": map[string]any{
					"profile": map[string]any{
						"name":  "Jane Smith",
						"email": "jane@example.com",
					},
					"id": "user123",
				},
				"action": "create",
			},
		)

		// Create task config with nested templates
		taskConfig := &task.Config{
			ID:     "nested-template-task",
			Type:   "basic",
			Action: "process",
			With: &core.Input{
				"api_config": map[string]any{
					"endpoint": "{{ .env.API_BASE }}/users/{{ .trigger.input.user.id }}",
					"headers": map[string]any{
						"X-User-Email": "{{ .trigger.input.user.profile.email }}",
						"Content-Type": "application/json",
					},
				},
				"user_display": "{{ .trigger.input.user.profile.name }}",
				"action_type":  "{{ .trigger.input.action }}",
			},
			Env: core.EnvMap{},
		}

		// Create execution
		_, execution := createTestTaskExecution(t, tb, workflowExecID, "nested-template-task", taskConfig)

		// Verify nested templates were parsed
		input := execution.GetInput()

		apiConfig, ok := input.Prop("api_config").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "https://api.example.com/users/user123", apiConfig["endpoint"])

		headers, ok := apiConfig["headers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "jane@example.com", headers["X-User-Email"])
		assert.Equal(t, "application/json", headers["Content-Type"])

		assert.Equal(t, "Jane Smith", input.Prop("user_display"))
		assert.Equal(t, "create", input.Prop("action_type"))
	})

	t.Run("Should handle environment variable merging with templates", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{
				"WORKFLOW_ENV": "from_workflow",
				"SHARED_VAR":   "workflow_value",
			},
			&core.Input{
				"service": "user-service",
			},
		)

		// Create task config with environment merging and templates
		taskConfig := &task.Config{
			ID:     "env-merge-task",
			Type:   "basic",
			Action: "process",
			With:   &core.Input{},
			Env: core.EnvMap{
				"TASK_ENV":     "from_task",
				"SHARED_VAR":   "task_value", // Should override workflow value
				"SERVICE_URL":  "https://{{ .trigger.input.service }}.example.com",
				"COMBINED_VAR": "{{ .env.WORKFLOW_ENV }}_and_{{ .env.TASK_ENV }}",
			},
		}

		// Create execution
		_, execution := createTestTaskExecution(t, tb, workflowExecID, "env-merge-task", taskConfig)

		// Verify environment variable merging and template parsing
		env := execution.GetEnv()

		// Workflow env should be present
		assert.Equal(t, "from_workflow", env.Prop("WORKFLOW_ENV"))

		// Task env should be present
		assert.Equal(t, "from_task", env.Prop("TASK_ENV"))

		// Task should override workflow for shared variables
		assert.Equal(t, "task_value", env.Prop("SHARED_VAR"))

		// Templates should be parsed
		assert.Equal(t, "https://user-service.example.com", env.Prop("SERVICE_URL"))

		// Complex template combining multiple env vars should work
		combinedVar := env.Prop("COMBINED_VAR")
		assert.Contains(t, combinedVar, "from_workflow")
		assert.Contains(t, combinedVar, "from_task")
	})
}
