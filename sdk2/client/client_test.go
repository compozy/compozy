package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/v2/client"
)

func TestExecuteWorkflow(t *testing.T) {
	t.Parallel()
	var called atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v0/workflows/sample/executions":
			called.Add(1)
			require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			body := make(map[string]any)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "subtask", body["task_id"])
			require.Contains(t, body, "input")
			require.Nil(t, body["input"])
			writeJSON(t, w, http.StatusAccepted, map[string]any{
				"exec_id":     "exec-123",
				"exec_url":    "/api/v0/executions/workflows/exec-123",
				"workflow_id": "sample",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	ctx := withTestLogger(t.Context())
	cl, err := client.New(ctx, server.URL, client.WithAPIKey("test-key"))
	require.NoError(t, err)
	resp, err := cl.ExecuteWorkflow(ctx, "sample", &client.WorkflowExecuteRequest{TaskID: "subtask"})
	require.NoError(t, err)
	require.Equal(t, "exec-123", resp.ExecID)
	require.Equal(t, "/api/v0/executions/workflows/exec-123", resp.ExecURL)
	require.Equal(t, int32(1), called.Load())
}

func TestExecuteTaskSync(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v0/tasks/build/executions/sync" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"exec_id": "task-exec",
			"output":  map[string]any{"artifact": "binary"},
		})
	}))
	t.Cleanup(server.Close)
	ctx := withTestLogger(t.Context())
	cl, err := client.New(ctx, server.URL)
	require.NoError(t, err)
	resp, err := cl.ExecuteTaskSync(ctx, "build", &client.TaskExecuteRequest{})
	require.NoError(t, err)
	require.Equal(t, "task-exec", resp.ExecID)
	require.Equal(t, "binary", (*resp.Output)["artifact"])
}

func TestExecuteWorkflowStream(t *testing.T) {
	t.Parallel()
	execPath := "/api/v0/executions/workflows/exec-456/stream"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v0/workflows/streamed/executions":
			writeJSON(t, w, http.StatusAccepted, map[string]any{
				"exec_id":     "exec-456",
				"exec_url":    execPath,
				"workflow_id": "streamed",
			})
		case r.Method == http.MethodGet && r.URL.Path == execPath:
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			require.True(t, ok)
			fmt.Fprintf(w, "id: 1\nevent: workflow_status\ndata: {\"status\":\"running\"}\n\n")
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
			fmt.Fprintf(w, "id: 2\nevent: complete\ndata: {\"status\":\"success\"}\n\n")
			flusher.Flush()
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	ctx := withTestLogger(t.Context())
	cl, err := client.New(ctx, server.URL)
	require.NoError(t, err)
	session, err := cl.ExecuteWorkflowStream(ctx, "streamed", &client.WorkflowExecuteRequest{}, nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, session.Close())
	}()
	events := collectEvents(t, session)
	require.Len(t, events, 2)
	require.Equal(t, int64(1), events[0].ID)
	require.Equal(t, "workflow_status", events[0].Type)
	require.JSONEq(t, `{"status":"running"}`, string(events[0].Data))
	require.Equal(t, "complete", events[1].Type)
	errVal := <-session.Errors()
	require.True(t, errors.Is(errVal, io.EOF) || errVal == nil)
}

func TestExecuteAgentError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, map[string]any{
			"error": map[string]any{
				"code":    "AGENT_NOT_FOUND",
				"message": "agent missing",
			},
		})
	}))
	t.Cleanup(server.Close)
	ctx := withTestLogger(t.Context())
	cl, err := client.New(ctx, server.URL)
	require.NoError(t, err)
	_, execErr := cl.ExecuteAgent(ctx, "missing", &client.AgentExecuteRequest{Prompt: "hi"})
	var apiErr *client.APIError
	require.ErrorAs(t, execErr, &apiErr)
	require.Equal(t, "AGENT_NOT_FOUND", apiErr.Code)
	require.Equal(t, http.StatusNotFound, apiErr.Status)
}

func collectEvents(t *testing.T, session *client.StreamSession) []client.StreamEvent {
	t.Helper()
	result := make([]client.StreamEvent, 0, 2)
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case evt, ok := <-session.Events():
			if !ok {
				return result
			}
			result = append(result, evt)
		case <-timeout.C:
			t.Fatal("timed out waiting for stream events")
		}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, data map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := map[string]any{
		"status":  status,
		"message": http.StatusText(status),
	}
	if data != nil {
		if raw, ok := data["error"]; ok {
			payload["error"] = raw
		} else {
			payload["data"] = data
		}
	}
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

func withTestLogger(parent context.Context) context.Context {
	return logger.ContextWithLogger(parent, logger.NewForTests())
}
