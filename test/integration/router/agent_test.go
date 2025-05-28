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

func TestAgentRoutesWithRealExamples(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	// Load real workflows from examples that contain agents
	weatherWorkflow, _ := utils.LoadExampleWorkflow(t, "weather-agent")
	quotesWorkflow, _ := utils.LoadExampleWorkflow(t, "quotes")

	// Update app state with the real workflows
	htb.AppState.Workflows = []*workflow.Config{weatherWorkflow, quotesWorkflow}

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
}
