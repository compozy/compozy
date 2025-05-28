package repo

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWorkflowTestBed(t *testing.T) *utils.IntegrationTestBed {
	t.Helper()
	return utils.SetupIntegrationTestBed(t, utils.DefaultTestTimeout)
}

func TestWorkflowRepository_FindConfig(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should find existing workflow config", func(t *testing.T) {
		workflowConfig := utils.CreateTestWorkflowConfig(t, tb, "test-workflow", core.EnvMap{})
		workflowConfig.Description = "Test workflow for code formatting"
		workflows := []*workflow.Config{workflowConfig}

		config, err := tb.WorkflowRepo.FindConfig(workflows, "test-workflow")
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, "test-workflow", config.ID)
		assert.Equal(t, "1.0.0", config.Version)
		assert.Equal(t, "Test workflow for code formatting", config.Description)
	})

	t.Run("Should return error for non-existent workflow", func(t *testing.T) {
		workflows := []*workflow.Config{}

		config, err := tb.WorkflowRepo.FindConfig(workflows, "non-existent-workflow")
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "workflow not found")
	})
}

func TestWorkflowRepository_CreateExecution(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should create workflow execution successfully", func(t *testing.T) {
		workflowConfig := utils.CreateTestWorkflowConfig(t, tb, "test-workflow", core.EnvMap{"TEST_VAR": "test_value"})
		workflowConfig.Description = "Test workflow"
		input := &core.Input{
			"code":     "console.log('hello')",
			"language": "javascript",
		}

		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{"TEST_VAR": "test_value"}, input)
		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, execution)

		assert.Equal(t, workflowExecID, execution.GetID())
		assert.Equal(t, "test-workflow", execution.GetComponentID())
		assert.Equal(t, core.StatusPending, execution.GetStatus())
		assert.NotNil(t, execution.GetInput())
		assert.Equal(t, "console.log('hello')", execution.GetInput().Prop("code"))
		assert.Equal(t, "javascript", execution.GetInput().Prop("language"))
	})

	t.Run("Should handle execution creation with empty input", func(t *testing.T) {
		input := &core.Input{}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{}, input)

		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, execution)
		assert.Equal(t, workflowExecID, execution.GetID())
	})
}

func TestWorkflowRepository_LoadExecution(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		input := &core.Input{
			"code":     "console.log('hello')",
			"language": "javascript",
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{"TEST_VAR": "test_value"}, input)
		loadedExecution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)

		assert.Equal(t, workflowExecID, loadedExecution.GetID())
		assert.Equal(t, "test-workflow", loadedExecution.GetComponentID())
		assert.Equal(t, core.StatusPending, loadedExecution.GetStatus())
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentID := core.MustNewID()
		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, nonExistentID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestWorkflowRepository_ExecutionToMap(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should convert execution to execution map", func(t *testing.T) {
		input := &core.Input{
			"test": "data",
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{}, input)
		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		execMap, err := tb.WorkflowRepo.ExecutionToMap(tb.Ctx, execution)
		require.NoError(t, err)
		require.NotNil(t, execMap)

		assert.Equal(t, core.StatusPending, execMap.Status)
		assert.Equal(t, "test-workflow", execMap.WorkflowID)
		assert.Equal(t, workflowExecID, execMap.WorkflowExecID)
		assert.NotNil(t, execMap.Tasks)
		assert.NotNil(t, execMap.Agents)
		assert.NotNil(t, execMap.Tools)
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentID := core.MustNewID()

		_, err := tb.WorkflowRepo.GetExecution(tb.Ctx, nonExistentID)
		assert.Error(t, err)
	})
}

func TestWorkflowRepository_ListExecutions(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list all executions", func(t *testing.T) {
		input := &core.Input{"test": "data"}
		workflowExecID1 := utils.CreateTestWorkflowExecution(t, tb, "test-workflow-1", core.EnvMap{}, input)
		workflowExecID2 := utils.CreateTestWorkflowExecution(t, tb, "test-workflow-2", core.EnvMap{}, input)
		executions, err := tb.WorkflowRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		require.Len(t, executions, 2)
		executionIDs := make([]core.ID, len(executions))
		for i, exec := range executions {
			executionIDs[i] = exec.GetID()
		}
		assert.Contains(t, executionIDs, workflowExecID1)
		assert.Contains(t, executionIDs, workflowExecID2)
	})

	t.Run("Should return empty list when no executions exist", func(t *testing.T) {
		executions, err := tb.WorkflowRepo.ListExecutions(tb.Ctx)
		require.NoError(t, err)
		assert.NotNil(t, executions)
	})
}

func TestWorkflowRepository_ExecutionsToMap(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should convert executions to execution maps", func(t *testing.T) {
		input := &core.Input{"test": "data"}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{}, input)
		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)

		executions := []workflow.Execution{*execution}
		executionMaps, err := tb.WorkflowRepo.ExecutionsToMap(tb.Ctx, executions)
		require.NoError(t, err)
		require.Len(t, executionMaps, 1)

		executionMap := executionMaps[0]
		assert.Equal(t, workflowExecID, executionMap.WorkflowExecID)
		assert.Equal(t, "test-workflow", executionMap.WorkflowID)
		assert.Equal(t, core.StatusPending, executionMap.Status)
		assert.NotNil(t, executionMap.Tasks)
		assert.NotNil(t, executionMap.Agents)
		assert.NotNil(t, executionMap.Tools)
	})

	t.Run("Should handle empty executions list", func(t *testing.T) {
		executionMaps, err := tb.WorkflowRepo.ExecutionsToMap(tb.Ctx, []workflow.Execution{})
		require.NoError(t, err)
		assert.Empty(t, executionMaps)
	})
}

func TestWorkflowRepository_CreateExecution_TemplateNormalization(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should parse templates in workflow input during execution creation", func(t *testing.T) {
		workflowInput := &core.Input{
			"service":     "user-service",
			"environment": "production",
			"config": map[string]any{
				"api_url":     "https://{{ .trigger.input.service }}.example.com",
				"environment": "{{ .trigger.input.environment }}",
				"project":     "{{ .env.PROJECT_NAME }}",
			},
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{
			"PROJECT_NAME": "compozy",
			"DYNAMIC_VAR":  "{{ .trigger.input.service }}_processed",
			"COMBINED_VAR": "{{ .env.PROJECT_NAME }}_{{ .trigger.input.environment }}",
		}, workflowInput)

		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, execution)
		input := execution.GetInput()
		assert.Equal(t, "user-service", input.Prop("service"))
		assert.Equal(t, "production", input.Prop("environment"))
		config, ok := input.Prop("config").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "https://user-service.example.com", config["api_url"])
		assert.Equal(t, "production", config["environment"])
		assert.Equal(t, "compozy", config["project"])
		env := execution.GetEnv()
		assert.Equal(t, "compozy", env.Prop("PROJECT_NAME"))
		assert.Equal(t, "user-service_processed", env.Prop("DYNAMIC_VAR"))
		assert.Equal(t, "compozy_production", env.Prop("COMBINED_VAR"))
	})

	t.Run("Should handle complex nested templates in workflow", func(t *testing.T) {
		workflowInput := &core.Input{
			"request": map[string]any{
				"user": map[string]any{
					"id":    "user123",
					"email": "user@example.com",
				},
				"action": "create",
			},
			"api_config": map[string]any{
				"endpoint": "{{ .env.API_BASE }}/{{ .env.API_VERSION }}/users/{{ .trigger.input.request.user.id }}",
				"headers": map[string]any{
					"Authorization": "Bearer token",
					"X-User-Email":  "{{ .trigger.input.request.user.email }}",
					"X-Action":      "{{ .trigger.input.request.action }}",
				},
				"metadata": []any{
					"{{ .trigger.input.request.action }}",
					"user_{{ .trigger.input.request.user.id }}",
					"static_value",
				},
			},
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "complex-workflow", core.EnvMap{
			"API_BASE":    "https://api.example.com",
			"API_VERSION": "v1",
		}, workflowInput)
		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, execution)
		input := execution.GetInput()

		request, ok := input.Prop("request").(map[string]any)
		require.True(t, ok)
		user, ok := request["user"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "user123", user["id"])
		assert.Equal(t, "user@example.com", user["email"])

		apiConfig, ok := input.Prop("api_config").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "https://api.example.com/v1/users/user123", apiConfig["endpoint"])

		headers, ok := apiConfig["headers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Bearer token", headers["Authorization"])
		assert.Equal(t, "user@example.com", headers["X-User-Email"])
		assert.Equal(t, "create", headers["X-Action"])

		metadata, ok := apiConfig["metadata"].([]any)
		require.True(t, ok)
		assert.Equal(t, "create", metadata[0])
		assert.Equal(t, "user_user123", metadata[1])
		assert.Equal(t, "static_value", metadata[2])
	})

	t.Run("Should handle templates with sprig functions", func(t *testing.T) {
		workflowInput := &core.Input{
			"user_name": "john doe",
			"email":     "JOHN.DOE@EXAMPLE.COM",
			"age":       25,
			"processing": map[string]any{
				"formatted_name":  "{{ title .trigger.input.user_name }}",
				"lowercase_email": "{{ lower .trigger.input.email }}",
				"age_plus_ten":    "{{ add .trigger.input.age 10 }}",
				"contains_check":  "{{ contains \"doe\" .trigger.input.user_name }}",
			},
		}

		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "sprig-workflow", core.EnvMap{}, workflowInput)
		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, execution)
		input := execution.GetInput()

		processing, ok := input.Prop("processing").(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "John Doe", processing["formatted_name"])
		assert.Equal(t, "john.doe@example.com", processing["lowercase_email"])
		assert.Equal(t, "35", processing["age_plus_ten"])
		assert.Equal(t, "true", processing["contains_check"])
	})
}

func TestWorkflowRepository_GetExecution(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should load existing execution", func(t *testing.T) {
		input := &core.Input{
			"test": "data",
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{"WORKFLOW_VAR": "workflow_value"}, input)
		loadedExecution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		require.NotNil(t, loadedExecution)
		assert.Equal(t, workflowExecID, loadedExecution.GetID())
		assert.Equal(t, "test-workflow", loadedExecution.GetComponentID())
		assert.Equal(t, core.StatusPending, loadedExecution.GetStatus())
		assert.Equal(t, workflowExecID, loadedExecution.GetWorkflowExecID())
	})

	t.Run("Should return error for non-existent execution", func(t *testing.T) {
		nonExistentWorkflowExecID := core.MustNewID()
		execution, err := tb.WorkflowRepo.GetExecution(tb.Ctx, nonExistentWorkflowExecID)
		assert.Error(t, err)
		assert.Nil(t, execution)
	})
}

func TestWorkflowRepository_ListExecutionsByStatus(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by status", func(t *testing.T) {
		input := &core.Input{
			"test": "data",
		}
		utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{}, input)
		executions, err := tb.WorkflowRepo.ListExecutionsByStatus(tb.Ctx, core.StatusPending)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		for _, exec := range executions {
			assert.Equal(t, core.StatusPending, exec.Status)
		}
	})

	t.Run("Should return empty list for status with no executions", func(t *testing.T) {
		executions, err := tb.WorkflowRepo.ListExecutionsByStatus(tb.Ctx, core.StatusSuccess)
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestWorkflowRepository_ListExecutionsByWorkflowID(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list executions by workflow ID", func(t *testing.T) {
		input := &core.Input{
			"test": "data",
		}
		utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{}, input)
		executions, err := tb.WorkflowRepo.ListExecutionsByWorkflowID(tb.Ctx, "test-workflow")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(executions), 1)

		for _, exec := range executions {
			assert.Equal(t, "test-workflow", exec.WorkflowID)
		}
	})

	t.Run("Should return empty list for non-existent workflow ID", func(t *testing.T) {
		executions, err := tb.WorkflowRepo.ListExecutionsByWorkflowID(tb.Ctx, "non-existent-workflow")
		require.NoError(t, err)
		assert.Empty(t, executions)
	})
}

func TestWorkflowRepository_ListChildrenExecutions(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list children executions by workflow execution ID", func(t *testing.T) {
		input := &core.Input{
			"test": "data",
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "test-workflow", core.EnvMap{}, input)
		taskConfig := utils.CreateTestBasicTaskConfig(t, "test-task", "process")
		_, _ = utils.CreateTestTaskExecution(t, tb, workflowExecID, taskConfig.ID, taskConfig)
		children, err := tb.WorkflowRepo.ListChildrenExecutions(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(children), 1)

		for _, child := range children {
			component := child.GetComponent()
			assert.True(t, component == core.ComponentTask || component == core.ComponentAgent || component == core.ComponentTool)
		}
	})

	t.Run("Should return empty list for workflow with no children", func(t *testing.T) {
		input := &core.Input{
			"test": "data",
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "lonely-workflow", core.EnvMap{}, input)
		children, err := tb.WorkflowRepo.ListChildrenExecutions(tb.Ctx, workflowExecID)
		require.NoError(t, err)
		assert.Empty(t, children)
	})
}

func TestWorkflowRepository_ListChildrenExecutionsByWorkflowID(t *testing.T) {
	tb := setupWorkflowTestBed(t)
	defer tb.Cleanup()

	t.Run("Should list children executions by workflow ID", func(t *testing.T) {
		input := &core.Input{
			"test": "data",
		}
		workflowExecID := utils.CreateTestWorkflowExecution(t, tb, "parent-workflow", core.EnvMap{}, input)
		taskConfig := utils.CreateTestBasicTaskConfig(t, "child-task", "process")
		_, _ = utils.CreateTestTaskExecution(t, tb, workflowExecID, taskConfig.ID, taskConfig)
		children, err := tb.WorkflowRepo.ListChildrenExecutionsByWorkflowID(tb.Ctx, "parent-workflow")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(children), 1)

		for _, child := range children {
			component := child.GetComponent()
			assert.True(t, component == core.ComponentTask || component == core.ComponentAgent || component == core.ComponentTool)
		}
	})

	t.Run("Should return empty list for workflow ID with no children", func(t *testing.T) {
		children, err := tb.WorkflowRepo.ListChildrenExecutionsByWorkflowID(tb.Ctx, "non-existent-workflow")
		require.NoError(t, err)
		assert.Empty(t, children)
	})
}
