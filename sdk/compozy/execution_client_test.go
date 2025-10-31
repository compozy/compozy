package compozy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	sdkclient "github.com/compozy/compozy/sdk/v2/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineExecutionDelegation(t *testing.T) {
	t.Run("Should delegate workflow async execution to sdk client", func(t *testing.T) {
		t.Parallel()
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/workflows/workflow-123/executions",
				http.StatusAccepted,
				map[string]any{
					"exec_id":     "exec-123",
					"exec_url":    "http://testing.local/api/v0/executions/workflows/exec-123",
					"workflow_id": "workflow-123",
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"name": "demo"}, body["input"])
					assert.Equal(t, "task-xyz", body["task_id"])
				},
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		resp, err := engine.ExecuteWorkflow(ctx, "workflow-123", &ExecuteRequest{
			Input:   map[string]any{"name": "demo"},
			Options: map[string]any{"task_id": "task-xyz"},
		})
		require.NoError(t, err)
		assert.Equal(t, "exec-123", resp.ExecID)
		assert.Equal(t, "http://testing.local/api/v0/executions/workflows/exec-123", resp.ExecURL)
		transport.AssertExhausted(t)
	})

	t.Run("Should translate workflow sync request timeout to seconds", func(t *testing.T) {
		t.Parallel()
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/workflows/sample/executions/sync",
				http.StatusOK,
				map[string]any{
					"exec_id": "sync-321",
					"output":  map[string]any{"result": "ok"},
				},
				func(body map[string]any) {
					assert.Equal(t, float64(2), body["timeout"])
					assert.Equal(t, map[string]any{"input": "value"}, body["input"])
				},
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		resp, err := engine.ExecuteWorkflowSync(ctx, "sample", &ExecuteSyncRequest{
			Input:   map[string]any{"input": "value"},
			Timeout: durationPtr(2 * time.Second),
			Options: map[string]any{"task_id": "ignored"},
		})
		require.NoError(t, err)
		assert.Equal(t, "sync-321", resp.ExecID)
		assert.Equal(t, map[string]any{"result": "ok"}, resp.Output)
		transport.AssertExhausted(t)
	})

	t.Run("Should translate workflow stream execution to sdk stream session", func(t *testing.T) {
		t.Parallel()
		sseData := "event: started\nid: 1\ndata: {\"status\":\"running\"}\n\n"
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/workflows/streamer/executions",
				http.StatusAccepted,
				map[string]any{
					"exec_id":  "exec-stream",
					"exec_url": "http://testing.local/api/v0/executions/workflows/exec-stream",
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"foo": "bar"}, body["input"])
				},
			),
			streamHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodGet,
				"/api/v0/executions/workflows/exec-stream/stream",
				sseData,
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		session, err := engine.ExecuteWorkflowStream(ctx, "streamer", &ExecuteRequest{
			Input: map[string]any{"foo": "bar"},
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "exec-stream", session.ExecID)

		select {
		case evt := <-session.Events():
			assert.Equal(t, int64(1), evt.ID)
			assert.Equal(t, "started", evt.Type)
			assert.JSONEq(t, `{"status":"running"}`, string(evt.Data))
		case <-time.After(time.Second):
			t.Fatal("expected stream event")
		}

		select {
		case err := <-session.Errors():
			require.ErrorIs(t, err, io.EOF)
		case <-time.After(time.Second):
			t.Fatal("expected terminal stream error")
		}
		require.NoError(t, session.Close())
		transport.AssertExhausted(t)
	})

	t.Run("Should delegate task sync execution with derived timeout", func(t *testing.T) {
		t.Parallel()
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/tasks/task-555/executions/sync",
				http.StatusOK,
				map[string]any{
					"exec_id": "task-sync",
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"key": "value"}, body["with"])
					assert.Equal(t, float64(5), body["timeout"])
				},
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		resp, err := engine.ExecuteTaskSync(ctx, "task-555", &ExecuteSyncRequest{
			Input:   map[string]any{"key": "value"},
			Timeout: durationPtr(5 * time.Second),
		})
		require.NoError(t, err)
		assert.Equal(t, "task-sync", resp.ExecID)
		transport.AssertExhausted(t)
	})

	t.Run("Should delegate agent async execution with options mapped", func(t *testing.T) {
		t.Parallel()
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/agents/support/executions",
				http.StatusAccepted,
				map[string]any{
					"exec_id":  "agent-async",
					"exec_url": "http://testing.local/api/v0/executions/agents/agent-async",
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"topic": "billing"}, body["with"])
					assert.Equal(t, "summarize", body["action"])
					assert.Equal(t, "help the user", body["prompt"])
					assert.Equal(t, float64(7), body["timeout"])
				},
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		resp, err := engine.ExecuteAgent(ctx, "support", &ExecuteRequest{
			Input: map[string]any{"topic": "billing"},
			Options: map[string]any{
				"action":  "summarize",
				"prompt":  "help the user",
				"timeout": 7,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "agent-async", resp.ExecID)
		transport.AssertExhausted(t)
	})

	t.Run("Should delegate task async execution with timeout option parsing", func(t *testing.T) {
		t.Parallel()
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/tasks/job/executions",
				http.StatusAccepted,
				map[string]any{
					"exec_id":  "task-async",
					"exec_url": "http://testing.local/api/v0/executions/tasks/task-async",
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"payload": "data"}, body["with"])
					assert.Equal(t, float64(15), body["timeout"])
				},
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		resp, err := engine.ExecuteTask(ctx, "job", &ExecuteRequest{
			Input:   map[string]any{"payload": "data"},
			Options: map[string]any{"timeout": "15"},
		})
		require.NoError(t, err)
		assert.Equal(t, "task-async", resp.ExecID)
		transport.AssertExhausted(t)
	})

	t.Run("Should translate task stream execution", func(t *testing.T) {
		t.Parallel()
		sseData := "event: data\nid: 1\ndata: {}\n\n"
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/tasks/stream/executions",
				http.StatusAccepted,
				map[string]any{
					"exec_id":  "task-stream",
					"exec_url": "http://testing.local/api/v0/executions/tasks/task-stream",
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"foo": "bar"}, body["with"])
				},
			),
			streamHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodGet,
				"/api/v0/executions/tasks/task-stream/stream",
				sseData,
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		session, err := engine.ExecuteTaskStream(ctx, "stream", &ExecuteRequest{
			Input: map[string]any{"foo": "bar"},
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "task-stream", session.ExecID)
		select {
		case <-session.Events():
		case <-time.After(time.Second):
			t.Fatal("expected stream event")
		}
		require.NoError(t, session.Close())
		transport.AssertExhausted(t)
	})

	t.Run("Should delegate agent sync execution with prompt mapping", func(t *testing.T) {
		t.Parallel()
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/agents/assistant/executions/sync",
				http.StatusOK,
				map[string]any{
					"exec_id": "agent-sync",
					"output":  map[string]any{"value": "ok"},
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"input": "value"}, body["with"])
					assert.Equal(t, "answer", body["prompt"])
					assert.Equal(t, "summarize", body["action"])
					assert.Equal(t, float64(3), body["timeout"])
				},
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		resp, err := engine.ExecuteAgentSync(ctx, "assistant", &ExecuteSyncRequest{
			Input: map[string]any{"input": "value"},
			Options: map[string]any{
				"action":  "summarize",
				"prompt":  "answer",
				"timeout": 3,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "agent-sync", resp.ExecID)
		assert.Equal(t, map[string]any{"value": "ok"}, resp.Output)
		transport.AssertExhausted(t)
	})

	t.Run("Should translate agent stream execution", func(t *testing.T) {
		t.Parallel()
		sseData := "event: delta\nid: 5\ndata: {}\n\n"
		transport := newStubTransport(
			t,
			jsonHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodPost,
				"/api/v0/agents/agent-stream/executions",
				http.StatusAccepted,
				map[string]any{
					"exec_id":  "agent-stream-exec",
					"exec_url": "http://testing.local/api/v0/executions/agents/agent-stream-exec",
				},
				func(body map[string]any) {
					assert.Equal(t, map[string]any{"foo": "bar"}, body["with"])
				},
			),
			streamHandler( //nolint:bodyclose // response closed via test cleanup
				t,
				http.MethodGet,
				"/api/v0/executions/agents/agent-stream-exec/stream",
				sseData,
			),
		)
		engine := engineWithClient(t, transport)
		ctx := executionContext(t)
		session, err := engine.ExecuteAgentStream(ctx, "agent-stream", &ExecuteRequest{
			Input: map[string]any{"foo": "bar"},
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "agent-stream-exec", session.ExecID)
		select {
		case <-session.Events():
		case <-time.After(time.Second):
			t.Fatal("expected agent stream event")
		}
		require.NoError(t, session.Close())
		transport.AssertExhausted(t)
	})
}

type stubTransport struct {
	t        *testing.T
	handlers []func(*http.Request) (*http.Response, error)
	mu       sync.Mutex
	index    int
}

func newStubTransport(
	t *testing.T,
	handlers ...func(*http.Request) (*http.Response, error),
) *stubTransport {
	return &stubTransport{t: t, handlers: handlers}
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index >= len(s.handlers) {
		s.t.Fatalf("unexpected request %s %s", req.Method, req.URL)
	}
	handler := s.handlers[s.index]
	s.index++
	return handler(req)
}

func (s *stubTransport) AssertExhausted(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != len(s.handlers) {
		t.Fatalf("expected %d requests, got %d", len(s.handlers), s.index)
	}
}

func jsonHandler(
	t *testing.T,
	expectedMethod string,
	expectedPath string,
	status int,
	data map[string]any,
	validate func(body map[string]any),
) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		require.Equal(t, expectedMethod, req.Method)
		require.Equal(t, expectedPath, req.URL.Path)
		defer req.Body.Close()
		payload, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		if len(payload) > 0 {
			var body map[string]any
			require.NoError(t, json.Unmarshal(payload, &body))
			if validate != nil {
				validate(body)
			}
		} else if validate != nil {
			validate(map[string]any{})
		}
		bodyData, err := json.Marshal(map[string]any{
			"status":  status,
			"message": http.StatusText(status),
			"data":    data,
		})
		require.NoError(t, err)
		resp := &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(bodyData)),
			Request:    req,
		}
		t.Cleanup(func() {
			require.NoError(t, resp.Body.Close())
		})
		return resp, nil
	}
}

func streamHandler(
	t *testing.T,
	expectedMethod string,
	expectedPath string,
	payload string,
) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		require.Equal(t, expectedMethod, req.Method)
		require.Equal(t, expectedPath, req.URL.Path)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(payload)),
			Request:    req,
		}
		t.Cleanup(func() {
			require.NoError(t, resp.Body.Close())
		})
		return resp, nil
	}
}

func engineWithClient(t *testing.T, transport http.RoundTripper) *Engine {
	t.Helper()
	httpClient := &http.Client{Transport: transport}
	ctx := executionContext(t)
	client, err := sdkclient.New(ctx, "http://testing.local", sdkclient.WithHTTPClient(httpClient))
	require.NoError(t, err)
	return &Engine{ctx: ctx, client: client}
}

func executionContext(t *testing.T) context.Context {
	t.Helper()
	return logger.ContextWithLogger(t.Context(), logger.NewForTests())
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}
