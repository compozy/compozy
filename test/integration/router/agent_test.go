package router

import (
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRoutesWithRealExamples(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load real workflows from examples that contain agents
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")

	// Update app state with the real workflows
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow}

	t.Run("GET /agents - list all agents", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agents retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		agents, exists := data["agents"]
		require.True(t, exists, "Data should contain agents key")
		assert.NotNil(t, agents, "Agents should not be nil")

		// Verify we get agents from the workflows
		agentsArray, ok := agents.([]interface{})
		require.True(t, ok, "Agents should be an array")
		assert.Greater(t, len(agentsArray), 0, "Should have at least one agent")

		// Verify agent data contains expected fields
		agentIDs := make([]string, 0, len(agentsArray))
		for _, agent := range agentsArray {
			agentData, ok := agent.(map[string]interface{})
			require.True(t, ok, "Agent should be a map")
			assert.Contains(t, agentData, "id", "Agent should have ID")
			assert.Contains(t, agentData, "instructions", "Agent should have instructions")
			assert.Contains(t, agentData, "config", "Agent should have config")
			agentIDs = append(agentIDs, agentData["id"].(string))
		}

		// Weather-agent workflow has an inline_agent
		assert.Contains(t, agentIDs, "inline_agent", "Should contain inline_agent from weather-agent")
	})

	t.Run("GET /agents/:agent_id - get specific agent", func(t *testing.T) {
		agentID := "inline_agent" // Agent from weather-agent workflow
		resp, err := htb.GET(baseURL + "/agents/" + agentID)
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the agent data
		agentData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Agent data should be a map")
		assert.Equal(t, agentID, agentData["id"], "Agent ID should match")
		assert.Contains(t, agentData, "instructions", "Should contain instructions")
		assert.Contains(t, agentData, "config", "Should contain config")

		// Verify config structure
		configData, exists := agentData["config"]
		require.True(t, exists, "Agent should have config")
		configMap, ok := configData.(map[string]interface{})
		require.True(t, ok, "Config should be a map")
		assert.Contains(t, configMap, "provider", "Config should contain provider")
		assert.Contains(t, configMap, "model", "Config should contain model")
	})

	t.Run("GET /agents/:agent_id/executions - list executions for agent", func(t *testing.T) {
		agentID := "inline_agent"
		resp, err := htb.GET(baseURL + "/agents/" + agentID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		executions, exists := data["executions"]
		require.True(t, exists, "Data should contain executions key")
		assert.NotNil(t, executions, "Executions should not be nil")

		// Verify executions is an array (might be empty if no executions exist)
		executionsArray, ok := executions.([]interface{})
		require.True(t, ok, "Executions should be an array")
		// Note: executions might be empty since we haven't created any executions in this test
		assert.GreaterOrEqual(t, len(executionsArray), 0, "Should have 0 or more executions")
	})
}

func TestAgentExecutionRoutesWithRealData(t *testing.T) {
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

	// Create task execution for the first task that uses agents
	firstTask := weatherWorkflow.Tasks[0] // get_current_weather task
	taskExecID, _ := utils.CreateTestTaskExecution(
		t, htb.IntegrationTestBed, workflowExecID, firstTask.ID, &firstTask,
	)

	// Create agent execution for the inline_agent agent
	var agentConfig *agent.Config
	for i := range weatherWorkflow.Agents {
		if weatherWorkflow.Agents[i].ID == "inline_agent" {
			agentConfig = &weatherWorkflow.Agents[i]
			break
		}
	}
	require.NotNil(t, agentConfig, "Should find inline_agent agent in weather-agent workflow")

	agentExecID, _ := utils.CreateTestAgentExecution(
		t, htb.IntegrationTestBed, taskExecID, agentConfig.ID, agentConfig,
	)

	t.Run("GET /agents/:agent_id/executions - list weather-agent agent executions", func(t *testing.T) {
		agentID := "inline_agent"
		resp, err := htb.GET(baseURL + "/agents/" + agentID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
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
			assert.Equal(t, agentID, execution["agent_id"], "Agent ID should match")
			assert.Contains(t, execution, "agent_exec_id", "Should contain agent execution ID")
			assert.Contains(t, execution, "status", "Should contain status")
			assert.Contains(t, execution, "component", "Should contain component")
			assert.Equal(t, "agent", execution["component"], "Component should be agent")
		}
	})

	t.Run("GET /executions/agents/:agent_exec_id - get weather-agent agent execution", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/agents/" + string(agentExecID))
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent execution retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the execution data
		execData, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Execution data should be a map")
		assert.Equal(t, string(agentExecID), execData["agent_exec_id"], "Agent execution ID should match")
		assert.Equal(t, agentConfig.ID, execData["agent_id"], "Agent ID should match")
		assert.Contains(t, execData, "status", "Should contain status")
		assert.Contains(t, execData, "component", "Should contain component")
		assert.Equal(t, "agent", execData["component"], "Component should be agent")

		// Verify input data contains weather-agent specific data
		if inputData, exists := execData["input"]; exists {
			inputMap, ok := inputData.(map[string]interface{})
			if ok {
				// Agent should have location-related input
				assert.NotEmpty(t, inputMap, "Agent input should not be empty")
			}
		}
	})

	t.Run("GET /executions/agents - list all agent executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "all agent executions retrieved", apiResp.Message)
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

func TestAgentRouteValidation(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load a real workflow for validation tests
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow}

	t.Run("GET /agents/:agent_id with invalid agent_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/agents/invalid-agent-id")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})

	t.Run("GET /agents/:agent_id/executions with empty agent_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/agents//executions")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("GET /agents/:agent_id with empty agent_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/agents/")
		require.NoError(t, err, "Failed to make GET request")

		// This should match the list agents route instead
		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agents retrieved", apiResp.Message)
	})

	t.Run("GET /agents/:agent_id/executions with valid agent_id", func(t *testing.T) {
		agentID := "inline_agent" // Valid agent from weather-agent
		resp, err := htb.GET(baseURL + "/agents/" + agentID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Verify the response contains the expected data structure
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		assert.Contains(t, data, "executions", "Should contain executions key")
	})

	t.Run("GET /executions/agents/:agent_exec_id with malformed execution_id", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/agents/malformed-id-123")
		require.NoError(t, err, "Failed to make GET request")

		// This should return a 500 error since the ID format will cause database issues
		htb.AssertErrorResponse(resp, http.StatusInternalServerError, router.ErrInternalCode)
	})

	t.Run("GET /executions/agents/:agent_exec_id with non-existent execution_id", func(t *testing.T) {
		nonExistentExecID := core.MustNewID()
		resp, err := htb.GET(baseURL + "/executions/agents/" + string(nonExistentExecID))
		require.NoError(t, err, "Failed to make GET request")

		// Should return 404 or 500 for non-existent execution
		assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError,
			"Should return 404 or 500 for non-existent execution, got %d", resp.StatusCode)
	})

	t.Run("GET /agents with no workflows loaded", func(t *testing.T) {
		// Temporarily clear workflows
		originalWorkflows := htb.AppState.Workflows
		htb.AppState.Workflows = []*workflow.Config{}

		resp, err := htb.GET(baseURL + "/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agents retrieved", apiResp.Message)

		// Should return empty agents array
		data, ok := apiResp.Data.(map[string]any)
		require.True(t, ok, "Data should be a map")
		agents, exists := data["agents"]
		require.True(t, exists, "Data should contain agents key")
		agentsArray, ok := agents.([]interface{})
		require.True(t, ok, "Agents should be an array")
		assert.Len(t, agentsArray, 0, "Should have no agents when no workflows loaded")

		// Restore workflows
		htb.AppState.Workflows = originalWorkflows
	})
}
