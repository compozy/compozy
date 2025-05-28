package router

import (
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRoutesWithRealExamples(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load weather-agent workflow from examples
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")

	// Update app state with the real workflow
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow}

	t.Run("GET /workflows/:workflow_id/tasks - list weather-agent tasks", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tasks retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		tasks, exists := data["tasks"]
		require.True(t, exists, "Data should contain tasks key")
		assert.NotNil(t, tasks, "Tasks should not be nil")

		// Verify we get the expected tasks from weather-agent
		tasksArray, ok := tasks.([]interface{})
		require.True(t, ok, "Tasks should be an array")
		assert.Len(t, tasksArray, 4, "Weather-agent should have 4 tasks")

		// Verify task data contains real weather-agent task IDs
		taskIDs := make([]string, 0, len(tasksArray))
		for _, task := range tasksArray {
			taskData, ok := task.(map[string]interface{})
			require.True(t, ok, "Task should be a map")
			taskIDs = append(taskIDs, taskData["id"].(string))
		}

		// Weather-agent has these specific tasks
		expectedTaskIDs := []string{"get_current_weather", "suggest_activities", "suggest_clothing", "save_weather_data"}
		for _, expectedID := range expectedTaskIDs {
			assert.Contains(t, taskIDs, expectedID, "Should contain weather-agent task: %s", expectedID)
		}
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id - get specific weather-agent task", func(t *testing.T) {
		taskID := "get_current_weather" // First task in weather-agent workflow
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks/" + taskID)
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the task data
		taskData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Task data should be a map")
		assert.Equal(t, taskID, taskData["id"], "Task ID should match")
		assert.Equal(t, "basic", taskData["type"], "Weather-agent tasks are basic type")
		assert.Contains(t, taskData, "use", "Should contain use field")
		assert.Contains(t, taskData, "action", "Should contain action field")
		assert.Contains(t, taskData, "with", "Should contain with field for input templates")

		// Verify the task uses an agent
		useField, exists := taskData["use"]
		require.True(t, exists, "Task should have use field")
		useStr, ok := useField.(string)
		require.True(t, ok, "Use field should be a string")
		assert.Contains(t, useStr, "agent(id=inline_agent)", "Should reference the inline_agent")
	})
}

func TestTaskExecutionRoutesWithRealData(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load weather-agent workflow and create execution
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow}

	// Create a real workflow execution with weather-agent
	weatherInput := utils.GetWeatherAgentTestInput()
	weatherWorkflow, workflowExecID := utils.CreateTestWorkflowFromExample(
		t, htb.IntegrationTestBed, "weather-agent", weatherInput,
	)

	// Create task executions for the first task (get_current_weather)
	firstTask := weatherWorkflow.Tasks[0] // get_current_weather
	taskExecID, _ := utils.CreateTestTaskExecution(
		t, htb.IntegrationTestBed, workflowExecID, firstTask.ID, &firstTask,
	)

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions - list weather-agent task executions", func(t *testing.T) {
		taskID := firstTask.ID
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks/" + taskID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		executions, exists := data["executions"]
		require.True(t, exists, "Data should contain executions key")
		assert.NotNil(t, executions, "Executions should not be nil")

		// Verify we get at least one execution
		executionsArray, ok := executions.([]interface{})
		require.True(t, ok, "Executions should be an array")
		assert.GreaterOrEqual(t, len(executionsArray), 1, "Should have at least 1 execution")

		// Verify execution data structure
		if len(executionsArray) > 0 {
			execution := executionsArray[0].(map[string]interface{})
			assert.Equal(t, taskID, execution["task_id"], "Task ID should match")
			assert.Contains(t, execution, "task_exec_id", "Should contain task execution ID")
			assert.Contains(t, execution, "status", "Should contain status")
			assert.Contains(t, execution, "component", "Should contain component")
			assert.Equal(t, "task", execution["component"], "Component should be task")
		}
	})

	t.Run("GET /executions/tasks/:task_exec_id - get weather-agent task execution", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks/" + string(taskExecID))
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task execution retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the execution data
		execData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Execution data should be a map")
		assert.Equal(t, string(taskExecID), execData["task_exec_id"], "Task execution ID should match")
		assert.Equal(t, firstTask.ID, execData["task_id"], "Task ID should match")
		assert.Contains(t, execData, "status", "Should contain status")
		assert.Contains(t, execData, "component", "Should contain component")
		assert.Equal(t, "task", execData["component"], "Component should be task")

		// Verify input data contains weather-agent specific data
		if inputData, exists := execData["input"]; exists {
			inputMap, ok := inputData.(map[string]interface{})
			if ok {
				assert.Contains(t, inputMap, "city", "Weather-agent task input should contain city")
			}
		}
	})

	t.Run("GET /executions/tasks - list all task executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "all task executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		executions, exists := data["executions"]
		require.True(t, exists, "Data should contain executions key")
		assert.NotNil(t, executions, "Executions should not be nil")

		// Verify we get executions (might be empty due to async nature)
		executionsArray, ok := executions.([]interface{})
		require.True(t, ok, "Executions should be an array")
		assert.GreaterOrEqual(t, len(executionsArray), 1, "Should have at least 1 execution")
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions/children - list children executions", func(t *testing.T) {
		taskID := firstTask.ID
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks/" + taskID + "/executions/children")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task children executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions/agents - list agent executions", func(t *testing.T) {
		taskID := firstTask.ID // This task uses an agent
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks/" + taskID + "/executions/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions/tools - list tool executions", func(t *testing.T) {
		taskID := firstTask.ID // This task uses a tool
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks/" + taskID + "/executions/tools")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/tasks/:task_exec_id/executions - list children executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks/" + string(taskExecID) + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task children executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/tasks/:task_exec_id/executions/agents - list agent executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks/" + string(taskExecID) + "/executions/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/tasks/:task_exec_id/executions/tools - list tool executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks/" + string(taskExecID) + "/executions/tools")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})
}

func TestTaskRouteValidation(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load a real workflow for validation tests
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow}

	t.Run("GET /workflows/:workflow_id/tasks/:task_id with invalid task_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks/invalid-task-id")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions with empty task_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/tasks//executions")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("GET /workflows/:workflow_id/tasks with invalid workflow_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/invalid-workflow-id/tasks")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})

	t.Run("GET /workflows/:workflow_id/tasks with empty workflow_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows//tasks")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("GET /executions/tasks/:task_exec_id with malformed execution_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks/malformed-id-123")
		require.NoError(t, err, "Failed to make GET request")

		// This should return a 500 error since the ID format will cause database issues
		htb.AssertErrorResponse(resp, http.StatusInternalServerError, router.ErrInternalCode)
	})
}
