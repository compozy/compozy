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
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// stubDirectExecutor provides a deterministic DirectExecutor for exercising request flows in tests.
type stubDirectExecutor struct {
	syncOutput   *core.Output
	syncExecID   core.ID
	syncErr      error
	asyncExecID  core.ID
	asyncErr     error
	syncCalls    int
	asyncCalls   int
	lastMetadata tkrouter.ExecMetadata
	lastAction   string
	lastWith     *core.Input
}

type stateSavingAgentExecutor struct {
	repo  *routertest.StubTaskRepo
	state *task.State
}

func (s *stateSavingAgentExecutor) ExecuteSync(
	_ context.Context,
	_ *task.Config,
	_ *tkrouter.ExecMetadata,
	_ time.Duration,
) (*core.Output, core.ID, error) {
	execID := core.MustNewID()
	clone := *s.state
	clone.TaskExecID = execID
	s.repo.AddState(&clone)
	return nil, execID, nil
}

func (s *stateSavingAgentExecutor) ExecuteAsync(
	_ context.Context,
	_ *task.Config,
	_ *tkrouter.ExecMetadata,
) (core.ID, error) {
	return core.MustNewID(), nil
}

func (s *stubDirectExecutor) ExecuteSync(
	_ context.Context,
	cfg *task.Config,
	meta *tkrouter.ExecMetadata,
	_ time.Duration,
) (*core.Output, core.ID, error) {
	s.syncCalls++
	if meta != nil {
		s.lastMetadata = *meta
	} else {
		s.lastMetadata = tkrouter.ExecMetadata{}
	}
	if cfg != nil {
		s.lastAction = cfg.Action
		if cfg.With != nil {
			if cloned, err := core.DeepCopy(cfg.With); err == nil {
				s.lastWith = cloned
			} else {
				s.lastWith = nil
			}
		} else {
			s.lastWith = nil
		}
	} else {
		s.lastAction = ""
		s.lastWith = nil
	}
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

// installDirectExecutorStub sets a temporary DirectExecutor factory and returns a cleanup function.
func installDirectExecutorStub(state *appstate.State, exec tkrouter.DirectExecutor) func() {
	tkrouter.SetDirectExecutorFactory(
		state,
		func(context.Context, *appstate.State, task.Repository) (tkrouter.DirectExecutor, error) {
			return exec, nil
		},
	)
	return func() { tkrouter.SetDirectExecutorFactory(state, nil) }
}

func TestAgentExecutionRoutes(t *testing.T) {
	t.Run("ShouldGetExecutionStatus", func(t *testing.T) {
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
	})

	t.Run("ShouldReturnNotFoundWhenExecutionMissing", func(t *testing.T) {
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
	})

	t.Run("ShouldHandleRepositoryErrors", func(t *testing.T) {
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
	})

	t.Run("ShouldValidateExecuteAgentSyncPayload", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		agentCfg := &agent.Config{ID: "agent-one"}
		m, err := agentCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			t.Context(),
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
		req := httptest.NewRequest(http.MethodPost, "/api/v0/agents/agent-one/executions/sync", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ShouldRejectTimeoutExceedingMaximum", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		cfg := &agent.Config{ID: "agent-timeout"}
		m, err := cfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			t.Context(),
			resources.ResourceKey{
				Project: state.ProjectConfig.Name,
				Type:    resources.ResourceAgent,
				ID:      "agent-timeout",
			},
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
			"/api/v0/agents/agent-timeout/executions/sync",
			strings.NewReader(`{"prompt":"run","timeout":301}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ShouldExecuteAgentSyncSuccessfully", func(t *testing.T) {
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
			t.Context(),
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
			"/api/v0/agents/agent-three/executions/sync",
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
	})

	t.Run("ShouldExecuteAgentActionSyncSuccessfully", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		agentCfg := &agent.Config{ID: "agent-action"}
		agentCfg.Model.Config.Provider = "openai"
		agentCfg.Model.Config.Model = "gpt-4o"
		agentCfg.Actions = []*agent.ActionConfig{
			{
				ID:           "acknowledge",
				Prompt:       "Acknowledge {{ .with.message }}",
				OutputSchema: &schema.Schema{"type": "object"},
			},
		}
		m, err := agentCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			t.Context(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-action"},
			m,
		)
		require.NoError(t, err)
		output := core.Output{
			"acknowledgement":  "ack received",
			"received_message": "Agent action payload",
		}
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
			"/api/v0/agents/agent-action/executions/sync",
			strings.NewReader(`{"action":"acknowledge","with":{"message":"Agent action payload"}}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var payload struct {
			Status int `json:"status"`
			Data   struct {
				ExecID string         `json:"exec_id"`
				Output map[string]any `json:"output"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		require.Equal(t, http.StatusOK, payload.Status)
		require.Equal(t, stub.syncExecID.String(), payload.Data.ExecID)
		require.Equal(t, "ack received", payload.Data.Output["acknowledgement"])
		require.Equal(t, "Agent action payload", payload.Data.Output["received_message"])
		require.Equal(t, 1, stub.syncCalls)
		require.Equal(t, "acknowledge", stub.lastMetadata.ActionID)
		require.Equal(t, "acknowledge", stub.lastAction)
		require.NotNil(t, stub.lastWith)
		require.Equal(t, "Agent action payload", (*stub.lastWith)["message"])
	})

	t.Run("ShouldUseStateOutputWhenExecutorReturnsNil", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		agentCfg := &agent.Config{ID: "agent-state"}
		agentCfg.Model.Config.Provider = "openai"
		agentCfg.Model.Config.Model = "gpt-4"
		m, err := agentCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			t.Context(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceAgent, ID: "agent-state"},
			m,
		)
		require.NoError(t, err)
		fallback := core.Output{"text": "from-state"}
		stateSnapshot := newTestTaskState(core.MustNewID())
		stateSnapshot.Output = &fallback
		stub := &stateSavingAgentExecutor{repo: repo, state: stateSnapshot}
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
			"/api/v0/agents/agent-state/executions/sync?include=state",
			strings.NewReader(`{"prompt":"should use state"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var payload struct {
			Status int `json:"status"`
			Data   struct {
				ExecID string             `json:"exec_id"`
				Output map[string]any     `json:"output"`
				State  ExecutionStatusDTO `json:"state"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		require.Equal(t, http.StatusOK, payload.Status)
		require.Equal(t, fallback["text"], payload.Data.Output["text"])
		require.NotNil(t, payload.Data.State.Output)
		require.Equal(t, fallback["text"], (*payload.Data.State.Output)["text"])
	})

	t.Run("ShouldHandleAgentSyncTimeouts", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		cfg := &agent.Config{ID: "agent-four"}
		m, err := cfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			t.Context(),
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
			"/api/v0/agents/agent-four/executions/sync",
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
	})

	t.Run("ShouldExecuteAgentAsyncSuccessfully", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		cfg := &agent.Config{ID: "agent-five"}
		m, err := cfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			t.Context(),
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
			"/api/v0/agents/agent-five/executions",
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
	})
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
