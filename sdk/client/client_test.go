package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/test/helpers"
)

func TestDeployProjectHandlesConcurrency(t *testing.T) {
	ctx := helpers.NewTestContext(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, routes.Base()+"/project", r.URL.Path)
			require.Equal(t, "demo", r.URL.Query().Get("project"))
			require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusPreconditionRequired)
			writeJSON(t, w, map[string]any{"message": "precondition required"})
		case 2:
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, routes.Base()+"/project", r.URL.Path)
			w.Header().Set("ETag", "\"current-etag\"")
			writeJSON(t, w, map[string]any{"message": "ok"})
		case 3:
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "\"current-etag\"", r.Header.Get("If-Match"))
			writeJSON(t, w, map[string]any{"message": "updated"})
		default:
			t.Fatalf("unexpected call %d", calls.Load())
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL).
		WithAPIKey("token").
		Build(ctx)
	require.NoError(t, err)

	err = client.DeployProject(ctx, &project.Config{Name: "demo"})
	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load())
}

func TestExecuteWorkflow(t *testing.T) {
	ctx := helpers.NewTestContext(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, routes.Base()+"/workflows/sample/executions", r.URL.Path)
		writeJSON(t, w, map[string]any{
			"data": map[string]any{
				"exec_id":     "exec123",
				"workflow_id": "sample",
				"exec_url":    "/executions/workflows/exec123",
			},
		})
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL).Build(ctx)
	require.NoError(t, err)

	result, err := client.ExecuteWorkflow(ctx, "sample", map[string]any{"foo": "bar"})
	require.NoError(t, err)
	require.Equal(t, "exec123", result.ExecutionID)
	require.Equal(t, "sample", result.WorkflowID)
}

func TestGetWorkflowStatus(t *testing.T) {
	ctx := helpers.NewTestContext(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, routes.Base()+"/executions/workflows/exec123", r.URL.Path)
		writeJSON(t, w, map[string]any{
			"data": map[string]any{
				"workflow_id":      "sample",
				"workflow_exec_id": "exec123",
				"status":           core.StatusSuccess,
				"output":           map[string]any{"result": "ok"},
				"error":            nil,
			},
		})
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL).Build(ctx)
	require.NoError(t, err)

	status, err := client.GetWorkflowStatus(ctx, "exec123")
	require.NoError(t, err)
	require.Equal(t, "sample", status.WorkflowID)
	require.Equal(t, core.StatusSuccess, status.Status)
	require.NotNil(t, status.Output)
	require.Nil(t, status.Error)
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}
