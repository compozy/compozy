package repo

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
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

	// Get the workflow execution to extract the correct workflow ID
	workflowExecution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
	require.NoError(t, err)

	taskExecID := core.MustNewID()
	taskMetadata := &pb.TaskMetadata{
		WorkflowId:     workflowExecution.WorkflowID, // Use the actual workflow ID
		WorkflowExecId: string(workflowExecID),
		TaskId:         taskID,
		TaskExecId:     string(taskExecID),
		Time:           timestamppb.Now(),
		Source:         "test",
	}
	err = taskConfig.SetCWD(tb.StateDir)
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
		loadedExecution, err := tb.TaskRepo.GetExecution(tb.Ctx, taskExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)

		// Verify loaded execution matches created execution
		assert.Equal(t, createdExecution.GetID(), loadedExecution.GetID())
		assert.Equal(t, createdExecution.GetComponentID(), loadedExecution.GetComponentID())
		assert.Equal(t, createdExecution.GetStatus(), loadedExecution.GetStatus())
		assert.Equal(t, createdExecution.GetWorkflowExecID(), loadedExecution.GetWorkflowExecID())
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentTaskExecID := core.MustNewID()
		execution, err := tb.TaskRepo.GetExecution(tb.Ctx, nonExistentTaskExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestTaskRepository_ListExecutionsByWorkflowExecID(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow execution ID", func(t *testing.T) {
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
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "task-1", taskConfig1)

		taskConfig2 := &task.Config{
			ID:     "task-2",
			Type:   "basic",
			Action: "action2",
			Env:    core.EnvMap{},
		}
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "task-2", taskConfig2)

		// List executions by workflow execution ID
		executions, err := tb.TaskRepo.ListExecutionsByWorkflowExecID(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct workflow execution ID
		for _, exec := range executions {
			assert.Equal(t, workflowExecID, exec.WorkflowExecID)
		}
	})

	t.Run("Should return empty list for workflow execution with no tasks", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()

		executions, err := tb.TaskRepo.ListExecutionsByWorkflowExecID(tb.Ctx, nonExistentWorkflowExecID)
		require.NoError(t, err)
		assert.Empty(t, executions)
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

		// List executions for first workflow only
		executions1, err := tb.TaskRepo.ListExecutionsByWorkflowExecID(tb.Ctx, workflowExecID1)
		require.NoError(t, err)
		assert.Len(t, executions1, 1)
		assert.Equal(t, taskExecID1, executions1[0].TaskExecID)

		// List executions for second workflow only
		executions2, err := tb.TaskRepo.ListExecutionsByWorkflowExecID(tb.Ctx, workflowExecID2)
		require.NoError(t, err)
		assert.Len(t, executions2, 1)
		assert.Equal(t, taskExecID2, executions2[0].TaskExecID)
	})
}

func TestTaskRepository_ExecutionsToMap(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should convert executions to execution maps", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		_, execution := createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Convert to execution maps
		executions := []core.Execution{execution}
		execMaps, err := tb.TaskRepo.ExecutionsToMap(tb.Ctx, executions)
		require.NoError(t, err)
		assert.Len(t, execMaps, 1)

		// Verify execution map properties
		execMap := execMaps[0]
		assert.Equal(t, core.StatusPending, execMap.Status)
		assert.Equal(t, core.ComponentTask, execMap.Component)
		assert.Equal(t, "test-workflow", execMap.WorkflowID)
		assert.Equal(t, workflowExecID, execMap.WorkflowExecID)
		assert.Equal(t, "test-task", execMap.TaskID)
	})

	t.Run("Should handle empty executions list", func(t *testing.T) {
		execMaps, err := tb.TaskRepo.ExecutionsToMap(tb.Ctx, []core.Execution{})
		require.NoError(t, err)
		assert.Empty(t, execMaps)
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

func TestTaskRepository_GetExecution(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		// Create workflow execution first
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{"TASK_VAR": "task_value"},
		}
		taskExecID, createdExecution := createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Load the execution
		loadedExecution, err := tb.TaskRepo.GetExecution(tb.Ctx, taskExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)

		// Verify loaded execution matches created execution
		assert.Equal(t, createdExecution.GetID(), loadedExecution.GetID())
		assert.Equal(t, createdExecution.GetComponentID(), loadedExecution.GetComponentID())
		assert.Equal(t, createdExecution.GetStatus(), loadedExecution.GetStatus())
		assert.Equal(t, createdExecution.GetWorkflowExecID(), loadedExecution.GetWorkflowExecID())
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentTaskExecID := core.MustNewID()
		execution, err := tb.TaskRepo.GetExecution(tb.Ctx, nonExistentTaskExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestTaskRepository_ListExecutions(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list all task executions", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create multiple task executions
		taskConfig1 := &task.Config{
			ID:     "task-1",
			Type:   "basic",
			Action: "action1",
			Env:    core.EnvMap{},
		}
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "task-1", taskConfig1)

		taskConfig2 := &task.Config{
			ID:     "task-2",
			Type:   "basic",
			Action: "action2",
			Env:    core.EnvMap{},
		}
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "task-2", taskConfig2)

		// List all executions
		executions, err := tb.TaskRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 2)
	})

	t.Run("Should return empty list when no executions exist", func(t *testing.T) {
		// Create a fresh test bed with empty database
		dbFilePath := filepath.Join(tb.StateDir, "empty_task_test.db")
		emptyStore, err := store.NewStore(dbFilePath)
		require.NoError(t, err)
		defer emptyStore.Close()

		err = emptyStore.Setup()
		require.NoError(t, err)

		// Create repositories with the empty store
		projectConfig := &project.Config{}
		err = projectConfig.SetCWD(tb.StateDir)
		require.NoError(t, err)

		workflows := []*workflow.Config{}
		workflowRepo := emptyStore.NewWorkflowRepository(projectConfig, workflows)
		taskRepo := emptyStore.NewTaskRepository(workflowRepo)

		executions, err := taskRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestTaskRepository_ListExecutionsByStatus(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by status", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// List executions by status
		executions, err := tb.TaskRepo.ListExecutionsByStatus(tb.Ctx, core.StatusPending)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct status
		for _, exec := range executions {
			assert.Equal(t, core.StatusPending, exec.Status)
		}
	})

	t.Run("Should return empty list for status with no executions", func(t *testing.T) {
		executions, err := tb.TaskRepo.ListExecutionsByStatus(tb.Ctx, core.StatusSuccess)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestTaskRepository_ListExecutionsByWorkflowID(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow ID", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// List executions by workflow ID
		executions, err := tb.TaskRepo.ListExecutionsByWorkflowID(tb.Ctx, "test-workflow")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct workflow ID
		for _, exec := range executions {
			assert.Equal(t, "test-workflow", exec.WorkflowID)
		}
	})

	t.Run("Should return empty list for non-existent workflow ID", func(t *testing.T) {
		executions, err := tb.TaskRepo.ListExecutionsByWorkflowID(tb.Ctx, "non-existent-workflow")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestTaskRepository_ListExecutionsByTaskID(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by task ID", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create multiple executions for the same task ID
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// List executions by task ID
		executions, err := tb.TaskRepo.ListExecutionsByTaskID(tb.Ctx, "test-task")
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct task ID
		for _, exec := range executions {
			assert.Equal(t, "test-task", exec.TaskID)
		}
	})

	t.Run("Should return empty list for non-existent task ID", func(t *testing.T) {
		executions, err := tb.TaskRepo.ListExecutionsByTaskID(tb.Ctx, "non-existent-task")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestTaskRepository_ListExecutionsByWorkflowAndTaskID(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow and task ID", func(t *testing.T) {
		// Use unique workflow IDs to avoid contamination
		uniqueWorkflowID1 := fmt.Sprintf("test-workflow-%d", time.Now().UnixNano())
		uniqueWorkflowID2 := fmt.Sprintf("other-workflow-%d", time.Now().UnixNano())

		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, uniqueWorkflowID1,
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		_, _ = createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create another workflow with different task
		workflowExecID2 := createTestWorkflowExecution(
			t, tb, uniqueWorkflowID2,
			core.EnvMap{},
			&core.Input{},
		)
		_, _ = createTestTaskExecution(t, tb, workflowExecID2, "test-task", taskConfig)

		// List executions by workflow and task ID
		executions, err := tb.TaskRepo.ListExecutionsByWorkflowAndTaskID(tb.Ctx, uniqueWorkflowID1, "test-task")
		require.NoError(t, err)
		assert.Len(t, executions, 1)

		// Verify returned execution has correct workflow and task ID
		exec := executions[0]
		assert.Equal(t, uniqueWorkflowID1, exec.WorkflowID)
		assert.Equal(t, "test-task", exec.TaskID)
	})

	t.Run("Should return empty list for non-existent combination", func(t *testing.T) {
		executions, err := tb.TaskRepo.ListExecutionsByWorkflowAndTaskID(tb.Ctx, "non-existent-workflow", "non-existent-task")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestTaskRepository_ListChildrenExecutions(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list children executions by task execution ID", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "test-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		taskExecID, _ := createTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create agent execution as child
		agentConfig := &agent.Config{
			ID:           "test-agent",
			Instructions: "Test agent",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			Env: core.EnvMap{},
		}
		err := agentConfig.SetCWD(tb.StateDir)
		require.NoError(t, err)

		agentExecID := core.MustNewID()
		agentMetadata := &pb.AgentMetadata{
			WorkflowId:     "test-workflow",
			WorkflowExecId: string(workflowExecID),
			TaskId:         "test-task",
			TaskExecId:     string(taskExecID),
			AgentId:        "test-agent",
			AgentExecId:    string(agentExecID),
			Time:           timestamppb.Now(),
			Source:         "test",
		}
		_, err = tb.AgentRepo.CreateExecution(tb.Ctx, agentMetadata, agentConfig)
		require.NoError(t, err)

		// List children executions
		children, err := tb.TaskRepo.ListChildrenExecutions(tb.Ctx, taskExecID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(children), 1)

		// Verify children are agents or tools
		for _, child := range children {
			component := child.GetComponent()
			assert.True(t, component == core.ComponentAgent || component == core.ComponentTool)
		}
	})

	t.Run("Should return empty list for task with no children", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution without children
		taskConfig := &task.Config{
			ID:     "lonely-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		taskExecID, _ := createTestTaskExecution(t, tb, workflowExecID, "lonely-task", taskConfig)

		// List children executions
		children, err := tb.TaskRepo.ListChildrenExecutions(tb.Ctx, taskExecID)
		require.NoError(t, err)
		assert.Empty(t, children)
	})
}

func TestTaskRepository_ListChildrenExecutionsByTaskID(t *testing.T) {
	tb := setupTaskRepoTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list children executions by task ID", func(t *testing.T) {
		// Create workflow execution
		workflowExecID := createTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution
		taskConfig := &task.Config{
			ID:     "parent-task",
			Type:   "basic",
			Action: "process",
			Env:    core.EnvMap{},
		}
		taskExecID, _ := createTestTaskExecution(t, tb, workflowExecID, "parent-task", taskConfig)

		// Create agent execution as child
		agentConfig := &agent.Config{
			ID:           "child-agent",
			Instructions: "Child agent",
			Config: agent.ProviderConfig{
				Provider: agent.ProviderAnthropic,
				Model:    agent.ModelClaude3Opus,
			},
			Env: core.EnvMap{},
		}
		err := agentConfig.SetCWD(tb.StateDir)
		require.NoError(t, err)

		agentExecID := core.MustNewID()
		agentMetadata := &pb.AgentMetadata{
			WorkflowId:     "test-workflow",
			WorkflowExecId: string(workflowExecID),
			TaskId:         "parent-task",
			TaskExecId:     string(taskExecID),
			AgentId:        "child-agent",
			AgentExecId:    string(agentExecID),
			Time:           timestamppb.Now(),
			Source:         "test",
		}
		_, err = tb.AgentRepo.CreateExecution(tb.Ctx, agentMetadata, agentConfig)
		require.NoError(t, err)

		// List children executions by task ID
		children, err := tb.TaskRepo.ListChildrenExecutionsByTaskID(tb.Ctx, "parent-task")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(children), 1)

		// Verify children are agents or tools
		for _, child := range children {
			component := child.GetComponent()
			assert.True(t, component == core.ComponentAgent || component == core.ComponentTool)
		}
	})

	t.Run("Should return empty list for task ID with no children", func(t *testing.T) {
		children, err := tb.TaskRepo.ListChildrenExecutionsByTaskID(tb.Ctx, "non-existent-task")
		require.NoError(t, err)
		assert.Empty(t, children)
	})
}
