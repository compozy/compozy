package repo

import (
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupToolTestBed(t *testing.T) *utils.IntegrationTestBed {
	t.Helper()
	return utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout)
}

func TestToolRepository_CreateExecution(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should create tool execution successfully", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.Env = core.EnvMap{"TASK_VAR": "task_value"}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool config using helper
		toolConfig := utils.CreateTestToolConfig(t, "code-formatter", "A tool for formatting code", core.EnvMap{"TOOL_VAR": "tool_value"})
		toolConfig.InputSchema = &schema.InputSchema{
			Schema: schema.Schema{
				"type": "object",
				"properties": map[string]any{
					"code": map[string]any{
						"type":        "string",
						"description": "The code to format",
					},
					"language": map[string]any{
						"type":        "string",
						"description": "The programming language",
					},
				},
				"required": []string{"code"},
			},
		}
		toolConfig.OutputSchema = &schema.OutputSchema{
			Schema: schema.Schema{
				"type": "object",
				"properties": map[string]any{
					"formatted_code": map[string]any{
						"type":        "string",
						"description": "The formatted code",
					},
				},
				"required": []string{"formatted_code"},
			},
		}
		toolConfig.With = &core.Input{
			"indent_size": 2,
			"use_tabs":    false,
		}

		// Create tool execution using helper
		toolExecID, execution := utils.CreateTestToolExecution(t, tb, taskExecID, "code-formatter", toolConfig)

		// Verify execution properties
		assert.Equal(t, toolExecID, execution.GetID())
		assert.Equal(t, "code-formatter", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.Equal(t, workflowExecID, execution.GetWorkflowExecID())
		assert.Equal(t, taskExecID, execution.TaskExecID)
		assert.NotNil(t, execution.GetEnv())
		assert.Equal(t, "tool_value", execution.GetEnv().Prop("TOOL_VAR"))
		assert.Equal(t, "task_value", execution.GetEnv().Prop("TASK_VAR"))
		assert.Equal(t, "workflow_value", execution.GetEnv().Prop("WORKFLOW_VAR"))
		assert.NotNil(t, execution.GetInput())
		assert.Equal(t, 2, execution.GetInput().Prop("indent_size"))
		assert.Equal(t, false, execution.GetInput().Prop("use_tabs"))
	})

	t.Run("Should handle execution creation with execute script", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "run-script")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool config with execute script using helper
		toolConfig := utils.CreateTestToolConfig(t, "script-runner", "A tool that runs scripts", core.EnvMap{})
		toolConfig.Execute = "./run.ts"
		toolConfig.With = &core.Input{
			"script_path": "./scripts/process.sh",
		}

		// Create tool execution using helper
		toolExecID, execution := utils.CreateTestToolExecution(t, tb, taskExecID, "script-runner", toolConfig)

		// Verify execution properties
		assert.Equal(t, toolExecID, execution.GetID())
		assert.Equal(t, "script-runner", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.NotNil(t, execution.GetInput())
		assert.Equal(t, "./scripts/process.sh", execution.GetInput().Prop("script_path"))
	})
}

func TestToolRepository_LoadExecution(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool config using helper
		toolConfig := utils.CreateTestToolConfig(t, "test-tool", "Test tool", core.EnvMap{})

		// Create tool execution using helper
		toolExecID, createdExecution := utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)

		// Load the execution
		loadedExecution, err := tb.ToolRepo.GetExecution(tb.Ctx, toolExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)

		// Verify loaded execution matches created execution
		assert.Equal(t, createdExecution.GetID(), loadedExecution.GetID())
		assert.Equal(t, createdExecution.GetComponentID(), loadedExecution.GetComponentID())
		assert.Equal(t, createdExecution.GetStatus(), loadedExecution.GetStatus())
		assert.Equal(t, createdExecution.GetWorkflowExecID(), loadedExecution.GetWorkflowExecID())
		assert.Equal(t, createdExecution.TaskExecID, loadedExecution.TaskExecID)
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentToolExecID := core.MustNewID()
		execution, err := tb.ToolRepo.GetExecution(tb.Ctx, nonExistentToolExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestToolRepository_ListExecutionsByWorkflowExecID(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow execution ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple tool executions for the same workflow execution using helpers
		toolConfig1 := utils.CreateTestToolConfig(t, "tool-1", "First tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "tool-1", toolConfig1)

		toolConfig2 := utils.CreateTestToolConfig(t, "tool-2", "Second tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "tool-2", toolConfig2)

		// List executions by workflow execution ID
		executions, err := tb.ToolRepo.ListExecutionsByWorkflowExecID(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct workflow execution ID
		for _, exec := range executions {
			assert.Equal(t, workflowExecID, exec.WorkflowExecID)
		}
	})

	t.Run("Should return empty list for workflow execution with no tools", func(t *testing.T) {
		// Create workflow execution with no tools using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "empty-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		executions, err := tb.ToolRepo.ListExecutionsByWorkflowExecID(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestToolRepository_ExecutionsToMap(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should convert executions to execution maps", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool execution using helper
		toolConfig := utils.CreateTestToolConfig(t, "test-tool", "Test tool", core.EnvMap{})
		_, execution := utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)

		// Convert to execution maps
		executions := []core.Execution{execution}
		execMaps, err := tb.ToolRepo.ExecutionsToMap(tb.Ctx, executions)
		require.NoError(t, err)
		assert.Len(t, execMaps, 1)

		// Verify execution map properties
		execMap := execMaps[0]
		assert.Equal(t, core.StatusPending, execMap.Status)
		assert.Equal(t, core.ComponentTool, execMap.Component)
		assert.Equal(t, "test-workflow", execMap.WorkflowID)
		assert.Equal(t, workflowExecID, execMap.WorkflowExecID)
		assert.Equal(t, "test-task", execMap.TaskID)
		assert.Equal(t, taskExecID, execMap.TaskExecID)
		assert.NotNil(t, execMap.ToolID)
		assert.Equal(t, "test-tool", *execMap.ToolID)
		assert.NotNil(t, execMap.ToolExecID)
	})

	t.Run("Should handle empty executions list", func(t *testing.T) {
		execMaps, err := tb.ToolRepo.ExecutionsToMap(tb.Ctx, []core.Execution{})
		require.NoError(t, err)
		assert.Empty(t, execMaps)
	})
}

func TestToolRepository_CreateExecution_TemplateNormalization(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should parse templates in tool input during execution creation", func(t *testing.T) {
		// Create workflow execution with input data using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{
				"user_name": "John Doe",
				"user_id":   123,
			},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.With = &core.Input{
			"task_data": "{{ .trigger.input.user_name }}",
		}
		taskConfig.Env = core.EnvMap{"TASK_VAR": "task_value"}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create test tool config with template input using helper
		toolConfig := utils.CreateTestToolConfig(t, "template-tool", "A tool that processes templates", core.EnvMap{
			"TOOL_VAR":    "tool_value",
			"DYNAMIC_VAR": "{{ .trigger.input.user_name }}_processed",
		})
		toolConfig.With = &core.Input{
			"greeting":     "Hello, {{ .trigger.input.user_name }}!",
			"user_id":      "{{ .trigger.input.user_id }}",
			"env_message":  "Environment: {{ .env.WORKFLOW_VAR }}",
			"static_value": "no template here",
		}

		// Create execution using helper
		_, execution := utils.CreateTestToolExecution(t, tb, taskExecID, "template-tool", toolConfig)

		// Verify templates were parsed in input
		input := execution.GetInput()
		assert.Equal(t, "Hello, John Doe!", input.Prop("greeting"))
		assert.Equal(t, "123", input.Prop("user_id")) // Numbers become strings in templates
		assert.Equal(t, "Environment: workflow_value", input.Prop("env_message"))
		assert.Equal(t, "no template here", input.Prop("static_value"))

		// Verify templates were parsed in environment
		env := execution.GetEnv()
		assert.Equal(t, "tool_value", env.Prop("TOOL_VAR"))
		assert.Equal(t, "John Doe_processed", env.Prop("DYNAMIC_VAR"))
		assert.Equal(t, "workflow_value", env.Prop("WORKFLOW_VAR")) // From workflow
		assert.Equal(t, "task_value", env.Prop("TASK_VAR"))         // From task
	})

	t.Run("Should handle nested templates in tool configuration", func(t *testing.T) {
		// Create workflow execution with nested input data using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
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

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool config with nested templates using helper
		toolConfig := utils.CreateTestToolConfig(t, "nested-template-tool", "Process user {{ .trigger.input.user.profile.name }}", core.EnvMap{})
		toolConfig.With = &core.Input{
			"api_config": map[string]any{
				"endpoint": "{{ .env.API_BASE }}/users/{{ .trigger.input.user.id }}",
				"headers": map[string]any{
					"X-User-Email": "{{ .trigger.input.user.profile.email }}",
					"Content-Type": "application/json",
				},
			},
			"user_display": "{{ .trigger.input.user.profile.name }}",
			"action_type":  "{{ .trigger.input.action }}",
		}

		// Create execution using helper
		_, execution := utils.CreateTestToolExecution(t, tb, taskExecID, "nested-template-tool", toolConfig)

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
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{
				"WORKFLOW_ENV": "from_workflow",
				"SHARED_VAR":   "workflow_value",
			},
			&core.Input{
				"service": "user-service",
			},
		)

		// Create task execution with environment using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.Env = core.EnvMap{
			"TASK_ENV":   "from_task",
			"SHARED_VAR": "task_value", // Should override workflow value
		}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool config with environment merging and templates using helper
		toolConfig := utils.CreateTestToolConfig(t, "env-merge-tool", "Process service {{ .trigger.input.service }}", core.EnvMap{
			"TOOL_ENV":     "from_tool",
			"SHARED_VAR":   "tool_value", // Should override task value
			"SERVICE_URL":  "https://{{ .trigger.input.service }}.example.com",
			"COMBINED_VAR": "{{ .env.WORKFLOW_ENV }}_and_{{ .env.TASK_ENV }}_and_{{ .env.TOOL_ENV }}",
		})

		// Create execution using helper
		_, execution := utils.CreateTestToolExecution(t, tb, taskExecID, "env-merge-tool", toolConfig)

		// Verify environment variable merging and template parsing
		env := execution.GetEnv()

		// Workflow env should be present
		assert.Equal(t, "from_workflow", env.Prop("WORKFLOW_ENV"))

		// Task env should be present
		assert.Equal(t, "from_task", env.Prop("TASK_ENV"))

		// Tool env should be present
		assert.Equal(t, "from_tool", env.Prop("TOOL_ENV"))

		// Tool should override task and workflow for shared variables
		assert.Equal(t, "tool_value", env.Prop("SHARED_VAR"))

		// Templates should be parsed
		assert.Equal(t, "https://user-service.example.com", env.Prop("SERVICE_URL"))

		// Complex template combining multiple env vars should work
		combinedVar := env.Prop("COMBINED_VAR")
		assert.Contains(t, combinedVar, "from_workflow")
		assert.Contains(t, combinedVar, "from_task")
		assert.Contains(t, combinedVar, "from_tool")
	})

	t.Run("Should handle tool with package reference templates", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{
				"GITHUB_ORG":  "myorg",
				"TOOL_REPO":   "toolrepo",
				"TOOL_BRANCH": "main",
			},
			&core.Input{
				"tool_id": "data-processor",
			},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool config with package reference using helper
		toolConfig := utils.CreateTestToolConfig(t, "dynamic-tool", "A tool with dynamic package reference", core.EnvMap{})
		// In real scenarios, package references with templates are parsed before tool config creation
		// Here we're testing that the environment variables are properly available
		toolConfig.With = &core.Input{
			"org":       "{{ .env.GITHUB_ORG }}",
			"repo":      "{{ .env.TOOL_REPO }}",
			"branch":    "{{ .env.TOOL_BRANCH }}",
			"tool_name": "{{ .trigger.input.tool_id }}",
		}

		// Create execution using helper
		_, execution := utils.CreateTestToolExecution(t, tb, taskExecID, "dynamic-tool", toolConfig)

		// Verify package reference related templates were parsed
		input := execution.GetInput()
		assert.Equal(t, "myorg", input.Prop("org"))
		assert.Equal(t, "toolrepo", input.Prop("repo"))
		assert.Equal(t, "main", input.Prop("branch"))
		assert.Equal(t, "data-processor", input.Prop("tool_name"))
	})
}

func TestToolRepository_GetExecution(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{"WORKFLOW_VAR": "workflow_value"},
			&core.Input{"workflow_input": "test_data"},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskConfig.Env = core.EnvMap{"TASK_VAR": "task_value"}
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool config using helper
		toolConfig := utils.CreateTestToolConfig(t, "test-tool", "Test tool", core.EnvMap{"TOOL_VAR": "tool_value"})

		// Create tool execution using helper
		toolExecID, createdExecution := utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)

		// Load the execution
		loadedExecution, err := tb.ToolRepo.GetExecution(tb.Ctx, toolExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)

		// Verify loaded execution matches created execution
		assert.Equal(t, createdExecution.GetID(), loadedExecution.GetID())
		assert.Equal(t, createdExecution.GetComponentID(), loadedExecution.GetComponentID())
		assert.Equal(t, createdExecution.GetStatus(), loadedExecution.GetStatus())
		assert.Equal(t, createdExecution.GetWorkflowExecID(), loadedExecution.GetWorkflowExecID())
		assert.Equal(t, createdExecution.TaskExecID, loadedExecution.TaskExecID)
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentToolExecID := core.MustNewID()
		execution, err := tb.ToolRepo.GetExecution(tb.Ctx, nonExistentToolExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestToolRepository_ListExecutions(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list all tool executions", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple tool executions using helpers
		toolConfig1 := utils.CreateTestToolConfig(t, "tool-1", "First tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "tool-1", toolConfig1)

		toolConfig2 := utils.CreateTestToolConfig(t, "tool-2", "Second tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "tool-2", toolConfig2)

		// List all executions
		executions, err := tb.ToolRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 2)
	})

	t.Run("Should return empty list when no executions exist", func(t *testing.T) {
		// Clear any existing data by creating a new database connection
		// but reuse the same NATS server to avoid conflicts
		dbFilePath := filepath.Join(tb.StateDir, "empty_test.db")
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
		toolRepo := emptyStore.NewToolRepository(workflowRepo, taskRepo)

		executions, err := toolRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestToolRepository_ListExecutionsByStatus(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by status", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool execution using helper
		toolConfig := utils.CreateTestToolConfig(t, "test-tool", "Test tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)

		// List executions by status
		executions, err := tb.ToolRepo.ListExecutionsByStatus(tb.Ctx, core.StatusPending)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct status
		for _, exec := range executions {
			assert.Equal(t, core.StatusPending, exec.Status)
		}
	})

	t.Run("Should return empty list for status with no executions", func(t *testing.T) {
		executions, err := tb.ToolRepo.ListExecutionsByStatus(tb.Ctx, core.StatusSuccess)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestToolRepository_ListExecutionsByWorkflowID(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool execution using helper
		toolConfig := utils.CreateTestToolConfig(t, "test-tool", "Test tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)

		// List executions by workflow ID
		executions, err := tb.ToolRepo.ListExecutionsByWorkflowID(tb.Ctx, "test-workflow")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct workflow ID
		for _, exec := range executions {
			assert.Equal(t, "test-workflow", exec.WorkflowID)
		}
	})

	t.Run("Should return empty list for non-existent workflow ID", func(t *testing.T) {
		executions, err := tb.ToolRepo.ListExecutionsByWorkflowID(tb.Ctx, "non-existent-workflow")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestToolRepository_ListExecutionsByTaskID(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by task ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create tool execution using helper
		toolConfig := utils.CreateTestToolConfig(t, "test-tool", "Test tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)

		// List executions by task ID
		executions, err := tb.ToolRepo.ListExecutionsByTaskID(tb.Ctx, "test-task")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		// Verify all returned executions have the correct task ID
		for _, exec := range executions {
			assert.Equal(t, "test-task", exec.TaskID)
		}
	})

	t.Run("Should return empty list for non-existent task ID", func(t *testing.T) {
		executions, err := tb.ToolRepo.ListExecutionsByTaskID(tb.Ctx, "non-existent-task")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestToolRepository_ListExecutionsByTaskExecID(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by task execution ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple tool executions for the same task execution using helpers
		toolConfig1 := utils.CreateTestToolConfig(t, "tool-1", "First tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "tool-1", toolConfig1)

		toolConfig2 := utils.CreateTestToolConfig(t, "tool-2", "Second tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "tool-2", toolConfig2)

		// List executions by task execution ID
		executions, err := tb.ToolRepo.ListExecutionsByTaskExecID(tb.Ctx, taskExecID)
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct task execution ID
		for _, exec := range executions {
			assert.Equal(t, taskExecID, exec.TaskExecID)
		}
	})

	t.Run("Should return empty list for non-existent task execution ID", func(t *testing.T) {
		nonExistentTaskExecID := core.MustNewID()
		executions, err := tb.ToolRepo.ListExecutionsByTaskExecID(tb.Ctx, nonExistentTaskExecID)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestToolRepository_ListExecutionsByToolID(t *testing.T) {
	tb := setupToolTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by tool ID", func(t *testing.T) {
		// Create workflow execution using helper
		workflowExecID := utils.CreateTestWorkflowExecution(
			t, tb, "test-workflow",
			core.EnvMap{},
			&core.Input{},
		)

		// Create task execution using helper
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		taskExecID, _ := utils.CreateTestTaskExecution(t, tb, workflowExecID, "test-task", taskConfig)

		// Create multiple executions for the same tool ID using helper
		toolConfig := utils.CreateTestToolConfig(t, "test-tool", "Test tool", core.EnvMap{})
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)
		_, _ = utils.CreateTestToolExecution(t, tb, taskExecID, "test-tool", toolConfig)

		// List executions by tool ID
		executions, err := tb.ToolRepo.ListExecutionsByToolID(tb.Ctx, "test-tool")
		require.NoError(t, err)
		assert.Len(t, executions, 2)

		// Verify all returned executions have the correct tool ID
		for _, exec := range executions {
			assert.Equal(t, "test-tool", exec.ToolID)
		}
	})

	t.Run("Should return empty list for non-existent tool ID", func(t *testing.T) {
		executions, err := tb.ToolRepo.ListExecutionsByToolID(tb.Ctx, "non-existent-tool")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}
