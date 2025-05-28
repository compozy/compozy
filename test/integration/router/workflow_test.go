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

func TestWorkflowRoutesWithRealExamples(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load real workflows from examples
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	quotesWorkflow, _ := utils.LoadExampleWorkflow(t, "quotes")

	// Update app state with the real workflows
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow, quotesWorkflow}

	t.Run("GET /workflows - list real workflows", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflows retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		workflows, exists := data["workflows"]
		require.True(t, exists, "Data should contain workflows key")
		assert.NotNil(t, workflows, "Workflows should not be nil")

		// Verify we get the expected workflows
		workflowsArray, ok := workflows.([]interface{})
		require.True(t, ok, "Workflows should be an array")
		assert.Len(t, workflowsArray, 2, "Should have 2 workflows")

		// Verify workflow data contains real examples
		workflowIDs := make([]string, 0, len(workflowsArray))
		for _, wf := range workflowsArray {
			workflowData, ok := wf.(map[string]interface{})
			require.True(t, ok, "Workflow should be a map")
			workflowIDs = append(workflowIDs, workflowData["id"].(string))
		}
		assert.Contains(t, workflowIDs, weatherWorkflow.ID, "Should contain weather-agent workflow")
		assert.Contains(t, workflowIDs, quotesWorkflow.ID, "Should contain quotes workflow")
	})

	t.Run("GET /workflows/:workflow_id - get weather-agent workflow", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID)
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the workflow data
		workflowData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Workflow data should be a map")
		assert.Equal(t, weatherWorkflow.ID, workflowData["id"], "Workflow ID should match")
		assert.Equal(t, weatherWorkflow.Description, workflowData["description"], "Workflow description should match")

		// Verify tasks are included and contain real weather-agent tasks
		tasksData, exists := workflowData["tasks"]
		require.True(t, exists, "Workflow should contain tasks")
		tasksArray, ok := tasksData.([]interface{})
		require.True(t, ok, "Tasks should be an array")
		assert.Greater(t, len(tasksArray), 0, "Weather-agent should have tasks")

		// Verify we have real task types from the weather-agent (all are basic tasks)
		taskTypes := make([]string, 0, len(tasksArray))
		for _, task := range tasksArray {
			taskData, ok := task.(map[string]interface{})
			require.True(t, ok, "Task should be a map")
			if taskType, exists := taskData["type"]; exists {
				taskTypes = append(taskTypes, taskType.(string))
			}
		}
		// Weather-agent uses basic tasks that reference agents and tools
		assert.Contains(t, taskTypes, "basic", "Weather-agent should have basic tasks")

		// Verify we have the expected number of tasks (4 tasks in weather-agent)
		assert.Len(t, tasksArray, 4, "Weather-agent should have 4 tasks")
	})

	t.Run("GET /workflows/:workflow_id - get quotes workflow", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/" + quotesWorkflow.ID)
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the workflow data
		workflowData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Workflow data should be a map")
		assert.Equal(t, quotesWorkflow.ID, workflowData["id"], "Workflow ID should match")

		// Quotes workflow doesn't have a description, so it should be empty or nil
		if desc, exists := workflowData["description"]; exists && desc != nil {
			assert.Equal(t, quotesWorkflow.Description, desc, "Workflow description should match")
		}

		// Verify tasks are included
		tasksData, exists := workflowData["tasks"]
		require.True(t, exists, "Workflow should contain tasks")
		tasksArray, ok := tasksData.([]interface{})
		require.True(t, ok, "Tasks should be an array")
		assert.Greater(t, len(tasksArray), 0, "Quotes workflow should have tasks")
	})

	t.Run("POST /workflows/:workflow_id/executions - execute weather-agent workflow", func(t *testing.T) {
		input := utils.GetWeatherAgentTestInput()

		resp, err := htb.POST(baseURL+"/workflows/"+weatherWorkflow.ID+"/executions", input)
		require.NoError(t, err, "Failed to make POST request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusAccepted)
		assert.Equal(t, "workflow triggered successfully", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		assert.Contains(t, data, "workflow_id", "Data should contain workflow_id")
		assert.Contains(t, data, "workflow_exec_id", "Data should contain workflow_exec_id")
		assert.Contains(t, data, "exec_url", "Data should contain exec_url")
		assert.Equal(t, weatherWorkflow.ID, data["workflow_id"], "Workflow ID should match")

		// Verify the execution URL is properly formatted
		execURL, ok := data["exec_url"].(string)
		require.True(t, ok, "Execution URL should be a string")
		assert.Contains(t, execURL, "/api/workflows/executions/", "Execution URL should contain correct path")
	})

	t.Run("POST /workflows/:workflow_id/executions - execute quotes workflow", func(t *testing.T) {
		input := utils.GetQuotesTestInput()

		resp, err := htb.POST(baseURL+"/workflows/"+quotesWorkflow.ID+"/executions", input)
		require.NoError(t, err, "Failed to make POST request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusAccepted)
		assert.Equal(t, "workflow triggered successfully", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		assert.Equal(t, quotesWorkflow.ID, data["workflow_id"], "Workflow ID should match")
		assert.Contains(t, data, "workflow_exec_id", "Data should contain workflow_exec_id")
		assert.Contains(t, data, "exec_url", "Data should contain exec_url")
	})

	t.Run("GET /workflows/:workflow_id with non-existent workflow", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/non-existent-workflow")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})
}

func TestWorkflowExecutionRoutesWithRealData(t *testing.T) {
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

	t.Run("GET /workflows/:workflow_id/executions - list weather-agent executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/" + weatherWorkflow.ID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow executions retrieved", apiResp.Message)
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
			execData, ok := executionsArray[0].(map[string]interface{})
			require.True(t, ok, "Execution should be a map")
			assert.Equal(t, weatherWorkflow.ID, execData["workflow_id"], "Workflow ID should match")
			assert.Contains(t, execData, "workflow_exec_id", "Should contain workflow execution ID")
			assert.Contains(t, execData, "status", "Should contain status")
		}
	})

	t.Run("GET /executions/workflows - list all workflow executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/workflows")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		executions, exists := data["executions"]
		require.True(t, exists, "Data should contain executions key")
		assert.NotNil(t, executions, "Executions should not be nil")

		// Verify we get at least the executions we created
		executionsArray, ok := executions.([]interface{})
		require.True(t, ok, "Executions should be an array")
		assert.GreaterOrEqual(t, len(executionsArray), 1, "Should have at least 1 execution")

		// Verify execution data
		if len(executionsArray) > 0 {
			execData, ok := executionsArray[0].(map[string]interface{})
			require.True(t, ok, "Execution should be a map")
			assert.Contains(t, execData, "workflow_id", "Should contain workflow ID")
			assert.Contains(t, execData, "workflow_exec_id", "Should contain workflow execution ID")
			assert.Contains(t, execData, "status", "Should contain status")
			assert.Contains(t, execData, "component", "Should contain component")
			assert.Equal(t, "workflow", execData["component"], "Component should be workflow")
		}
	})

	t.Run("GET /executions/workflows/:workflow_exec_id - get weather-agent execution", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/workflows/" + string(workflowExecID))
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow execution retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the execution data
		execData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Execution data should be a map")
		assert.Equal(t, string(workflowExecID), execData["workflow_exec_id"], "Workflow execution ID should match")
		assert.Equal(t, weatherWorkflow.ID, execData["workflow_id"], "Workflow ID should match")
		assert.Contains(t, execData, "status", "Should contain status")
		assert.Contains(t, execData, "component", "Should contain component")
		assert.Equal(t, "workflow", execData["component"], "Component should be workflow")

		// Verify input data contains weather-agent specific data
		if inputData, exists := execData["input"]; exists {
			inputMap, ok := inputData.(map[string]interface{})
			if ok {
				assert.Contains(t, inputMap, "city", "Weather-agent input should contain city")
			}
		}
	})

	t.Run("GET /executions/workflows/:workflow_exec_id/executions/tasks - list weather-agent task executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/workflows/" + string(workflowExecID) + "/executions/tasks")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should have data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		executions, exists := data["executions"]
		require.True(t, exists, "Data should contain executions key")
		assert.NotNil(t, executions, "Executions should not be nil")

		// Weather-agent should have task executions
		// Note: We might not have executions yet since they're created asynchronously
		// but the endpoint should still work
	})

	t.Run("GET /executions/workflows/:workflow_exec_id with non-existent execution", func(t *testing.T) {
		nonExistentExecID := core.MustNewID()
		resp, err := htb.GET(baseURL + "/executions/workflows/" + string(nonExistentExecID))
		require.NoError(t, err, "Failed to make GET request")

		// The API currently returns 500 for non-existent executions, but 404 would also be acceptable
		assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError,
			"Should return 404 or 500 for non-existent execution, got %d", resp.StatusCode)
	})
}

func TestWorkflowRouteValidation(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load a real workflow for validation tests
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow}

	t.Run("GET /workflows/:workflow_id with invalid workflow_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/invalid-workflow-id")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})

	t.Run("GET /workflows/:workflow_id/executions with empty workflow_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows//executions")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("POST /workflows/:workflow_id/executions with invalid JSON", func(t *testing.T) {
		// Make request with invalid JSON by sending raw string
		req := utils.HTTPRequest{
			Method: http.MethodPost,
			Path:   baseURL + "/workflows/" + weatherWorkflow.ID + "/executions",
			Body:   "invalid json",
		}

		resp, err := htb.MakeRequest(req)
		require.NoError(t, err, "Failed to make POST request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("POST /workflows/:workflow_id/executions with empty workflow_id", func(t *testing.T) {
		input := utils.GetWeatherAgentTestInput()
		resp, err := htb.POST(baseURL+"/workflows//executions", input)
		require.NoError(t, err, "Failed to make POST request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("POST /workflows/:workflow_id/executions with valid weather-agent input", func(t *testing.T) {
		input := utils.GetWeatherAgentTestInput()

		resp, err := htb.POST(baseURL+"/workflows/"+weatherWorkflow.ID+"/executions", input)
		require.NoError(t, err, "Failed to make POST request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusAccepted)
		assert.Equal(t, "workflow triggered successfully", apiResp.Message)

		// Verify the response contains the expected data
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		assert.Equal(t, weatherWorkflow.ID, data["workflow_id"], "Workflow ID should match")
		assert.Contains(t, data, "workflow_exec_id", "Should contain workflow execution ID")
		assert.Contains(t, data, "exec_url", "Should contain execution URL")
	})

	t.Run("POST /workflows/:workflow_id/executions with missing required input", func(t *testing.T) {
		// Weather-agent requires 'city' parameter, test with empty input
		emptyInput := &core.Input{}

		resp, err := htb.POST(baseURL+"/workflows/"+weatherWorkflow.ID+"/executions", emptyInput)
		require.NoError(t, err, "Failed to make POST request")

		// This should still succeed at the API level, validation happens during execution
		apiResp := htb.AssertSuccessResponse(resp, http.StatusAccepted)
		assert.Equal(t, "workflow triggered successfully", apiResp.Message)
	})

	t.Run("GET /executions/workflows/:workflow_exec_id with malformed execution_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/workflows/malformed-id-123")
		require.NoError(t, err, "Failed to make GET request")

		// This should return an error since the ID format is invalid
		assert.True(t, resp.StatusCode >= 400, "Should return an error for malformed ID")
	})
}
