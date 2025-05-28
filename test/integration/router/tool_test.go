package router

import (
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolRoutesWithRealExamples(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load real workflows from examples that contain tools
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	quotesWorkflow, _ := utils.LoadExampleWorkflow(t, "quotes")

	// Update app state with the real workflows
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow, quotesWorkflow}

	t.Run("GET /tools - list all tools", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/tools")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tools retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		tools, exists := data["tools"]
		require.True(t, exists, "Data should contain tools key")
		assert.NotNil(t, tools, "Tools should not be nil")

		// Verify we get tools from the workflows
		toolsArray, ok := tools.([]interface{})
		require.True(t, ok, "Tools should be an array")
		assert.Greater(t, len(toolsArray), 0, "Should have at least one tool")

		// Verify tool data contains expected fields
		toolIDs := make([]string, 0, len(toolsArray))
		for _, tool := range toolsArray {
			toolData, ok := tool.(map[string]interface{})
			require.True(t, ok, "Tool should be a map")
			assert.Contains(t, toolData, "id", "Tool should have ID")
			assert.Contains(t, toolData, "description", "Tool should have description")
			toolIDs = append(toolIDs, toolData["id"].(string))
		}

		// Weather-agent workflow has specific tools
		assert.Contains(t, toolIDs, "weather_tool", "Should contain weather_tool tool from weather-agent")
	})

	t.Run("GET /tools/:tool_id - get specific tool", func(t *testing.T) {
		toolID := "weather_tool" // Tool from weather-agent workflow
		resp, err := htb.GET(baseURL + "/tools/" + toolID)
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the tool data
		toolData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Tool data should be a map")
		assert.Equal(t, toolID, toolData["id"], "Tool ID should match")
		assert.Contains(t, toolData, "description", "Should contain description")
		assert.Contains(t, toolData, "execute", "Should contain execute field")

		// Verify the tool has proper structure
		if executeField, exists := toolData["execute"]; exists {
			executeStr, ok := executeField.(string)
			require.True(t, ok, "Execute field should be a string")
			assert.NotEmpty(t, executeStr, "Execute field should not be empty")
		}
	})

	t.Run("GET /tools/:tool_id/executions - list tool executions", func(t *testing.T) {
		toolID := "weather_tool" // Tool from weather-agent workflow
		resp, err := htb.GET(baseURL + "/tools/" + toolID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		executions, exists := data["executions"]
		require.True(t, exists, "Data should contain executions key")
		assert.NotNil(t, executions, "Executions should not be nil")

		// Verify executions array structure (might be empty if no executions yet)
		executionsArray, ok := executions.([]interface{})
		require.True(t, ok, "Executions should be an array")
		// Note: Array might be empty since we haven't created executions yet
		assert.GreaterOrEqual(t, len(executionsArray), 0, "Should have 0 or more executions")
	})
}

func TestToolExecutionRoutesWithRealData(t *testing.T) {
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

	// Create task execution for the first task that uses tools
	firstTask := weatherWorkflow.Tasks[0] // get_current_weather task
	taskExecID, _ := utils.CreateTestTaskExecution(
		t, htb.IntegrationTestBed, workflowExecID, firstTask.ID, &firstTask,
	)

	// Create tool execution for the weather_tool tool
	var toolConfig *tool.Config
	for i := range weatherWorkflow.Tools {
		if weatherWorkflow.Tools[i].ID == "weather_tool" {
			toolConfig = &weatherWorkflow.Tools[i]
			break
		}
	}
	require.NotNil(t, toolConfig, "Should find weather_tool tool in weather-agent workflow")

	toolExecID, _ := utils.CreateTestToolExecution(
		t, htb.IntegrationTestBed, taskExecID, toolConfig.ID, toolConfig,
	)

	t.Run("GET /tools/:tool_id/executions - list weather-agent tool executions", func(t *testing.T) {
		toolID := "weather_tool"
		resp, err := htb.GET(baseURL + "/tools/" + toolID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool executions retrieved", apiResp.Message)
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
			assert.Equal(t, toolID, execution["tool_id"], "Tool ID should match")
			assert.Contains(t, execution, "tool_exec_id", "Should contain tool execution ID")
			assert.Contains(t, execution, "status", "Should contain status")
			assert.Contains(t, execution, "component", "Should contain component")
			assert.Equal(t, "tool", execution["component"], "Component should be tool")
		}
	})

	t.Run("GET /executions/tools/:tool_exec_id - get weather-agent tool execution", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tools/" + string(toolExecID))
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool execution retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the execution data
		execData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Execution data should be a map")
		assert.Equal(t, string(toolExecID), execData["tool_exec_id"], "Tool execution ID should match")
		assert.Equal(t, toolConfig.ID, execData["tool_id"], "Tool ID should match")
		assert.Contains(t, execData, "status", "Should contain status")
		assert.Contains(t, execData, "component", "Should contain component")
		assert.Equal(t, "tool", execData["component"], "Component should be tool")

		// Verify input data contains weather-agent specific data
		if inputData, exists := execData["input"]; exists {
			inputMap, ok := inputData.(map[string]interface{})
			if ok {
				// Weather tool should have location-related input
				assert.NotEmpty(t, inputMap, "Tool input should not be empty")
			}
		}
	})

	t.Run("GET /executions/tools - list all tool executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tools")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "all tool executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		executions, exists := data["executions"]
		require.True(t, exists, "Data should contain executions key")
		assert.NotNil(t, executions, "Executions should not be nil")

		// Verify we get executions
		executionsArray, ok := executions.([]interface{})
		require.True(t, ok, "Executions should be an array")
		assert.GreaterOrEqual(t, len(executionsArray), 1, "Should have at least 1 execution")
	})
}

func TestToolRouteValidation(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load a real workflow for validation tests
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow}

	t.Run("GET /tools/:tool_id with invalid tool_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/tools/invalid-tool-id")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})

	t.Run("GET /tools/:tool_id/executions with empty tool_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/tools//executions")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("GET /tools/:tool_id with empty tool_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/tools/")
		require.NoError(t, err, "Failed to make GET request")

		// This should match the list tools route instead
		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tools retrieved", apiResp.Message)
	})

	t.Run("GET /tools/:tool_id/executions with valid tool_id", func(t *testing.T) {
		toolID := "weather_tool" // Valid tool from weather-agent
		resp, err := htb.GET(baseURL + "/tools/" + toolID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the response contains the expected data structure
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		assert.Contains(t, data, "executions", "Should contain executions key")
	})

	t.Run("GET /executions/tools/:tool_exec_id with malformed execution_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tools/malformed-id-123")
		require.NoError(t, err, "Failed to make GET request")

		// This should return a 500 error since the ID format will cause database issues
		htb.AssertErrorResponse(resp, http.StatusInternalServerError, router.ErrInternalCode)
	})

	t.Run("GET /executions/tools/:tool_exec_id with non-existent execution_id", func(t *testing.T) {
		nonExistentExecID := core.MustNewID()
		resp, err := htb.GET(baseURL + "/executions/tools/" + string(nonExistentExecID))
		require.NoError(t, err, "Failed to make GET request")

		// Should return 404 or 500 for non-existent execution
		assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError,
			"Should return 404 or 500 for non-existent execution, got %d", resp.StatusCode)
	})

	t.Run("GET /tools with no workflows loaded", func(t *testing.T) {
		// Temporarily clear workflows
		originalWorkflows := htb.AppState.Workflows
		htb.AppState.Workflows = []*workflow.Config{}

		resp, err := htb.GET(baseURL + "/tools")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tools retrieved", apiResp.Message)

		// Should return empty tools array
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		tools, exists := data["tools"]
		require.True(t, exists, "Data should contain tools key")
		toolsArray, ok := tools.([]interface{})
		require.True(t, ok, "Tools should be an array")
		assert.Len(t, toolsArray, 0, "Should have no tools when no workflows loaded")

		// Restore workflows
		htb.AppState.Workflows = originalWorkflows
	})
}
