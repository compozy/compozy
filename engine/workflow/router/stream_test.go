package wfrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	"github.com/compozy/compozy/engine/core"
	appstate "github.com/compozy/compozy/engine/infra/server/appstate"
	routerpkg "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/worker"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
)

type stubWorkflowQueryClient struct {
	responses []*wf.StreamState
	mu        sync.Mutex
	calls     int
}

func (s *stubWorkflowQueryClient) QueryWorkflow(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ ...any,
) (converter.EncodedValue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.calls
	s.calls++
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	state := s.responses[idx]
	payloads, err := converter.GetDefaultDataConverter().ToPayloads(state)
	if err != nil {
		return nil, err
	}
	return client.NewValue(payloads), nil
}

func newWorkflowStreamTestRouter(t *testing.T, client workflowQueryClient) *gin.Engine {
	t.Helper()
	ginmode.EnsureGinTestMode()
	proj := &project.Config{Name: "demo"}
	require.NoError(t, proj.SetCWD(t.TempDir()))
	deps := appstate.NewBaseDeps(proj, nil, nil, nil)
	state, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	state.Worker = &worker.Worker{}
	state.SetWorkflowQueryClient(client)
	cfgManager := config.NewManager(t.Context(), config.NewService())
	_, err = cfgManager.Load(t.Context(), config.NewDefaultProvider())
	require.NoError(t, err)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := logger.ContextWithLogger(c.Request.Context(), logger.NewForTests())
		ctx = config.ContextWithManager(ctx, cfgManager)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(appstate.StateMiddleware(state))
	r.Use(routerpkg.ErrorHandler())
	api := r.Group("/api/v0")
	Register(api)
	return r
}

func buildStreamState(t *testing.T, status core.StatusType, events ...struct {
	Type string
	Data any
}) *wf.StreamState {
	t.Helper()
	state := wf.NewStreamState(status)
	for _, evt := range events {
		require.NoError(t, state.Append(evt.Type, time.Unix(0, 0), evt.Data))
	}
	state.SetStatus(status)
	return state
}

func TestStreamWorkflow_InvalidLastEventID(t *testing.T) {
	client := &stubWorkflowQueryClient{responses: []*wf.StreamState{
		buildStreamState(t, core.StatusRunning, struct {
			Type string
			Data any
		}{Type: wf.StreamEventWorkflowStatus, Data: map[string]any{"status": core.StatusRunning}}),
	}}
	r := newWorkflowStreamTestRouter(t, client)
	execID := core.MustNewID()
	req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/workflows/"+execID.String()+"/stream", http.NoBody)
	req.Header.Set("Last-Event-ID", "not-a-number")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "invalid Last-Event-ID")
}

func TestStreamWorkflow_InvalidPollInterval(t *testing.T) {
	client := &stubWorkflowQueryClient{responses: []*wf.StreamState{
		buildStreamState(t, core.StatusRunning, struct {
			Type string
			Data any
		}{Type: wf.StreamEventWorkflowStatus, Data: map[string]any{"status": core.StatusRunning}}),
	}}
	r := newWorkflowStreamTestRouter(t, client)
	execID := core.MustNewID()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v0/executions/workflows/"+execID.String()+"/stream?poll_ms=42",
		http.NoBody,
	)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "poll_ms")
}

func TestStreamWorkflow_EmitsOnlyNewEventsAndCompletes(t *testing.T) {
	stateRunning := buildStreamState(t, core.StatusRunning,
		struct {
			Type string
			Data any
		}{Type: wf.StreamEventWorkflowStatus, Data: map[string]any{"status": core.StatusRunning}},
		struct {
			Type string
			Data any
		}{Type: wf.StreamEventWorkflowStatus, Data: map[string]any{"status": core.StatusRunning}},
	)
	stateSuccess := buildStreamState(t, core.StatusSuccess,
		struct {
			Type string
			Data any
		}{Type: wf.StreamEventWorkflowStatus, Data: map[string]any{"status": core.StatusRunning}},
		struct {
			Type string
			Data any
		}{Type: wf.StreamEventWorkflowStatus, Data: map[string]any{"status": core.StatusRunning}},
		struct {
			Type string
			Data any
		}{Type: wf.StreamEventComplete, Data: map[string]any{"status": core.StatusSuccess}},
	)
	client := &stubWorkflowQueryClient{responses: []*wf.StreamState{stateRunning, stateSuccess}}
	r := newWorkflowStreamTestRouter(t, client)
	execID := core.MustNewID()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v0/executions/workflows/"+execID.String()+"/stream?poll_ms=250",
		http.NoBody,
	)
	req.Header.Set("Last-Event-ID", "1")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	expected := "id: 2\nevent: workflow_status\ndata: {\"status\":\"RUNNING\"}\n\nid: 3\nevent: complete\ndata: {\"status\":\"SUCCESS\"}\n\n"
	require.Equal(t, expected, res.Body.String())
}
