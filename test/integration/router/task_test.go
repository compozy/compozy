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

func TestTaskRoutes(t *testing.T) {
	htb := utils.SetupHTTPTestBed(t, 10*time.Second)
	defer htb.Cleanup()

	version := core.GetVersion()
	baseURL := "/api/" + version

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions - list task executions", func(t *testing.T) {
		workflowID := "test-workflow"
		taskID := "test-task"
		resp, err := htb.GET(baseURL + "/workflows/" + workflowID + "/tasks/" + taskID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "task executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions/children - list children executions", func(t *testing.T) {
		workflowID := "test-workflow"
		taskID := "test-task"
		resp, err := htb.GET(baseURL + "/workflows/" + workflowID + "/tasks/" + taskID + "/executions/children")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions/agents - list agent executions", func(t *testing.T) {
		workflowID := "test-workflow"
		taskID := "test-task"
		resp, err := htb.GET(baseURL + "/workflows/" + workflowID + "/tasks/" + taskID + "/executions/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /workflows/:workflow_id/tasks/:task_id/executions/tools - list tool executions", func(t *testing.T) {
		workflowID := "test-workflow"
		taskID := "test-task"
		resp, err := htb.GET(baseURL + "/workflows/" + workflowID + "/tasks/" + taskID + "/executions/tools")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "tool executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/tasks - list all task executions", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "all task executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/tasks/:task_exec_id - get task execution", func(t *testing.T) {
		taskExecID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/tasks/" + taskExecID)
		require.NoError(t, err, "Failed to make GET request")

		// This should return an error since the execution doesn't exist
		htb.AssertErrorResponse(resp, http.StatusInternalServerError, router.ErrInternalCode)
	})

	t.Run("GET /executions/tasks/:task_exec_id/executions - list children executions", func(t *testing.T) {
		taskExecID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/tasks/" + taskExecID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "workflow executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/tasks/:task_exec_id/executions/agents - list agent executions", func(t *testing.T) {
		taskExecID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/tasks/" + taskExecID + "/executions/agents")
		require.NoError(t, err, "Failed to make GET request")

		apiResp := htb.AssertSuccessResponse(resp, http.StatusOK)
		assert.Equal(t, "agent executions retrieved", apiResp.Message)
		assert.NotNil(t, apiResp.Data, "Response should contain data")
	})

	t.Run("GET /executions/tasks/:task_exec_id/executions/tools - list tool executions", func(t *testing.T) {
		taskExecID := "test-exec-id"
		resp, err := htb.GET(baseURL + "/executions/tasks/" + taskExecID + "/executions/tools")
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

	t.Run("GET with missing workflow_id parameter", func(t *testing.T) {
		taskID := "test-task"
		resp, err := htb.GET(baseURL + "/workflows//tasks/" + taskID + "/executions")
		require.NoError(t, err, "Failed to make GET request")

		// This should return bad request due to empty workflow_id
		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("GET with missing task_id parameter", func(t *testing.T) {
		workflowID := "test-workflow"
		resp, err := htb.GET(baseURL + "/workflows/" + workflowID + "/tasks//executions")
		require.NoError(t, err, "Failed to make GET request")

		// This should return bad request due to empty task_id
		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})

	t.Run("GET with missing task_exec_id parameter", func(t *testing.T) {
		resp, err := htb.GET(baseURL + "/executions/tasks//executions")
		require.NoError(t, err, "Failed to make GET request")

		// This should return bad request due to empty task_exec_id
		htb.AssertErrorResponse(resp, http.StatusBadRequest, router.ErrBadRequestCode)
	})
}
