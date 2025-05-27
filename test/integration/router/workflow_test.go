package router

import (
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRoutes(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("GET /workflows - list workflows", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflows retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		workflows, exists := data["workflows"]
		require.True(t, exists, "Data should contain workflows key")
		assert.NotNil(t, workflows, "Workflows should not be nil")
	})

	t.Run("GET /workflows/:workflow_id - get workflow by ID", func(t *testing.T) {
		workflowID := "test-workflow"
		resp, err := htb.GET(baseURL + "/workflows/" + workflowID)
		require.NoError(t, err, "Failed to make GET request")

		// This should return 404 since no workflows are configured in test
		htb.AssertErrorResponse(resp, http.StatusNotFound, router.ErrNotFoundCode)
	})

	t.Run("GET /workflows/:workflow_id/executions - list workflow executions", func(t *testing.T) {
		workflowID := "test-workflow"
		resp, err := htb.GET(baseURL + "/workflows/" + workflowID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("POST /workflows/:workflow_id/executions - execute workflow", func(t *testing.T) {
		workflowID := "test-workflow"
		input := map[string]interface{}{
			"test": "data",
		}

		resp, err := htb.POST(baseURL+"/workflows/"+workflowID+"/executions", input)
		require.NoError(t, err, "Failed to make POST request")

		// This should return 202 (accepted) since the workflow trigger works correctly
		// even though the workflow doesn't exist in the config
		apiResp := htb.AssertSuccessResponse(resp, http.StatusAccepted)
		assert.Equal(t, "workflow triggered successfully", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")

		// Check data structure
		data, ok := apiResp.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")
		assert.Contains(t, data, "workflow_id", "Data should contain workflow_id")
		assert.Contains(t, data, "workflow_exec_id", "Data should contain workflow_exec_id")
		assert.Contains(t, data, "exec_url", "Data should contain exec_url")
		assert.Equal(t, workflowID, data["workflow_id"], "Workflow ID should match")
	})

	t.Run("GET /executions/workflows - list all workflow executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/workflows")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/workflows/:workflow_exec_id - get workflow execution", func(t *testing.T) {
		workflowExecID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/workflows/" + workflowExecID)
		require.NoError(t, err, "Failed to make GET request")

		// This should return an error since the execution doesn't exist
		htb.AssertErrorResponse(resp, http.StatusInternalServerError, router.ErrInternalCode)
	})
}

func TestWorkflowExecutionRoutes(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("GET /executions/workflows/:workflow_exec_id/executions - list children executions", func(t *testing.T) {
		execID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/workflows/" + execID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		// This should return success even if execution doesn't exist (empty list)
		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should have data")
	})

	t.Run("GET /executions/workflows/:workflow_exec_id/executions/tasks - list task executions", func(t *testing.T) {
		execID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/workflows/" + execID + "/executions/tasks")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should have data")
	})

	t.Run("GET /executions/workflows/:workflow_exec_id/executions/agents - list agent executions", func(t *testing.T) {
		execID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/workflows/" + execID + "/executions/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should have data")
	})

	t.Run("GET /executions/workflows/:workflow_exec_id/executions/tools - list tool executions", func(t *testing.T) {
		execID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/workflows/" + execID + "/executions/tools")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should have data")
	})
}

func TestWorkflowRouteValidation(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("GET /workflows with missing workflow_id parameter", func(t *testing.T) {
		// This should work - it's the list all workflows endpoint
		resp, err := htb.GET(baseURL + "/workflows")
		require.NoError(t, err, "Failed to make GET request")

		htb.AssertSuccessResponse(resp, http.StatusOK)
	})

	t.Run("POST /workflows/:workflow_id/executions with invalid JSON", func(t *testing.T) {
		workflowID := "test-workflow"

		// Make request with invalid JSON by sending raw string
		req := utils.HTTPRequest{
			Method: http.MethodPost,
			Path:   baseURL + "/workflows/" + workflowID + "/executions",
			Body:   "invalid json",
		}

		resp, err := htb.MakeRequest(req)
		require.NoError(t, err, "Failed to make POST request")

		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("GET with empty workflow_id parameter", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/workflows/")
		require.NoError(t, err, "Failed to make GET request")

		// This should hit the list workflows endpoint, not the get by ID endpoint
		htb.AssertSuccessResponse(resp, http.StatusOK)
	})
}
