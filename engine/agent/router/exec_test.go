package agentrouter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	srrouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubIdempotency struct {
	unique bool
	reason string
	err    error
}

func (s *stubIdempotency) CheckAndSet(
	_ context.Context,
	_ *gin.Context,
	_ string,
	_ []byte,
	_ time.Duration,
) (bool, string, error) {
	return s.unique, s.reason, s.err
}

type stubDirectExecutor struct {
	syncOutput  *core.Output
	syncExecID  core.ID
	syncErr     error
	asyncExecID core.ID
	asyncErr    error
	syncCalls   int
	asyncCalls  int
}

func (s *stubDirectExecutor) ExecuteSync(
	_ context.Context,
	_ *task.Config,
	_ *tkrouter.ExecMetadata,
	_ time.Duration,
) (*core.Output, core.ID, error) {
	s.syncCalls++
	return s.syncOutput, s.syncExecID, s.syncErr
}

func (s *stubDirectExecutor) ExecuteAsync(
	_ context.Context,
	_ *task.Config,
	_ *tkrouter.ExecMetadata,
) (core.ID, error) {
	s.asyncCalls++
	return s.asyncExecID, s.asyncErr
}

func installDirectExecutorStub(state *appstate.State, exec tkrouter.DirectExecutor) func() {
	tkrouter.SetDirectExecutorFactory(state, func(*appstate.State, task.Repository) (tkrouter.DirectExecutor, error) {
		return exec, nil
	})
	return func() { tkrouter.SetDirectExecutorFactory(state, nil) }
}

func Test_getAgentExecutionStatus(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	execID := core.MustNewID()
	repo.AddState(newTestTaskState(execID))
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/agents/"+execID.String(), http.NoBody)
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		Status  int                `json:"status"`
		Message string             `json:"message"`
		Data    ExecutionStatusDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, execID.String(), payload.Data.ExecID)
	require.Equal(t, core.StatusSuccess, payload.Data.Status)
}

func Test_getAgentExecutionStatus_NotFound(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/agents/"+core.MustNewID().String(), http.NoBody)
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func Test_getAgentExecutionStatus_RepoError(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	repo.SetError(errors.New("boom"))
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/agents/"+core.MustNewID().String(), http.NoBody)
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func Test_executeAgentSync_Validation(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	store := routertest.NewResourceStore(state)
	agentCfg := &agent.Config{ID: "agent-one"}
	m, err := agentCfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-one"},
		m,
	)
	require.NoError(t, err)
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(http.MethodPost, "/api/v0/agents/agent-one/executions", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func Test_executeAgentSync_TimeoutTooLarge(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	store := routertest.NewResourceStore(state)
	cfg := &agent.Config{ID: "agent-timeout"}
	m, err := cfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-timeout"},
		m,
	)
	require.NoError(t, err)
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v0/agents/agent-timeout/executions",
		strings.NewReader(`{"prompt":"run","timeout":301}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func Test_executeAgentSync_IdempotencyDuplicate(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	store := routertest.NewResourceStore(state)
	cfg := &agent.Config{ID: "agent-two"}
	m, err := cfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-two"},
		m,
	)
	require.NoError(t, err)
	cleanup := installDirectExecutorStub(state, &stubDirectExecutor{})
	defer cleanup()
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		stub := &stubIdempotency{unique: false, reason: "duplicate"}
		srrouter.SetAPIIdempotency(c, stub)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v0/agents/agent-two/executions",
		strings.NewReader(`{"prompt":"run"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code)
}

func Test_executeAgentSync_Success(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	store := routertest.NewResourceStore(state)
	cfg := &agent.Config{ID: "agent-three"}
	cfg.Model.Config.Provider = "openai"
	cfg.Model.Config.Model = "gpt-4"
	m, err := cfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-three"},
		m,
	)
	require.NoError(t, err)
	output := core.Output{"text": "ok"}
	stub := &stubDirectExecutor{syncOutput: &output, syncExecID: core.MustNewID()}
	cleanup := installDirectExecutorStub(state, stub)
	defer cleanup()
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v0/agents/agent-three/executions",
		strings.NewReader(`{"prompt":"hello"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var payload struct {
		Status int `json:"status"`
		Data   struct {
			ExecID string      `json:"exec_id"`
			Output core.Output `json:"output"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, http.StatusOK, payload.Status)
	require.Equal(t, stub.syncExecID.String(), payload.Data.ExecID)
	require.Equal(t, "ok", payload.Data.Output["text"])
}

func Test_executeAgentSync_Timeout(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	store := routertest.NewResourceStore(state)
	cfg := &agent.Config{ID: "agent-four"}
	m, err := cfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-four"},
		m,
	)
	require.NoError(t, err)
	stub := &stubDirectExecutor{syncExecID: core.MustNewID(), syncErr: context.DeadlineExceeded}
	cleanup := installDirectExecutorStub(state, stub)
	defer cleanup()
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v0/agents/agent-four/executions",
		strings.NewReader(`{"prompt":"hi"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusRequestTimeout, w.Code)
	var payload struct {
		Status  int                `json:"status"`
		Message string             `json:"message"`
		Data    map[string]any     `json:"data"`
		Error   srrouter.ErrorInfo `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, srrouter.ErrRequestTimeoutCode, payload.Error.Code)
	require.Equal(t, "execution timeout", payload.Message)
	execVal, ok := payload.Data["exec_id"].(string)
	require.True(t, ok)
	require.Equal(t, stub.syncExecID.String(), execVal)
}

func Test_executeAgentAsync_Success(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := routertest.NewTestAppState(t)
	repo := routertest.NewStubTaskRepo()
	store := routertest.NewResourceStore(state)
	cfg := &agent.Config{ID: "agent-five"}
	m, err := cfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-five"},
		m,
	)
	require.NoError(t, err)
	stub := &stubDirectExecutor{asyncExecID: core.MustNewID()}
	cleanup := installDirectExecutorStub(state, stub)
	defer cleanup()
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(srrouter.ErrorHandler())
	r.Use(func(c *gin.Context) {
		srrouter.SetTaskRepository(c, repo)
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v0/agents/agent-five/executions/async",
		strings.NewReader(`{"prompt":"hi"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req = routertest.WithConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	expectedLocation := routertest.ComposeLocation("agents", stub.asyncExecID)
	require.Equal(t, expectedLocation, w.Header().Get("Location"))
	var payload struct {
		Status int `json:"status"`
		Data   struct {
			ExecID  string `json:"exec_id"`
			ExecURL string `json:"exec_url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, stub.asyncExecID.String(), payload.Data.ExecID)
	require.Contains(t, payload.Data.ExecURL, stub.asyncExecID.String())
}
func newTestTaskState(execID core.ID) *task.State {
	now := time.Now().UTC()
	workflowExecID := core.MustNewID()
	output := core.Output{"result": "ok"}
	return &task.State{
		Component:      core.ComponentAgent,
		Status:         core.StatusSuccess,
		TaskID:         "agent-task",
		TaskExecID:     execID,
		WorkflowID:     "workflow-one",
		WorkflowExecID: workflowExecID,
		Output:         &output,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
