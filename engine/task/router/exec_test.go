package tkrouter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	srrouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubAPIIdempotency struct {
	unique bool
	reason string
	err    error
}

func (s *stubAPIIdempotency) CheckAndSet(
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
}

func (s *stubDirectExecutor) ExecuteSync(
	_ context.Context,
	_ *task.Config,
	_ *ExecMetadata,
	_ time.Duration,
) (*core.Output, core.ID, error) {
	return s.syncOutput, s.syncExecID, s.syncErr
}

func (s *stubDirectExecutor) ExecuteAsync(
	_ context.Context,
	_ *task.Config,
	_ *ExecMetadata,
) (core.ID, error) {
	return s.asyncExecID, s.asyncErr
}

func installTaskExecutor(state *appstate.State, exec DirectExecutor) func() {
	SetDirectExecutorFactory(state, func(*appstate.State, task.Repository) (DirectExecutor, error) {
		return exec, nil
	})
	return func() { SetDirectExecutorFactory(state, nil) }
}

func TestTaskExecutionRoutes(t *testing.T) {
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
		req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/tasks/"+execID.String(), http.NoBody)
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var payload struct {
			Status  int                    `json:"status"`
			Message string                 `json:"message"`
			Data    TaskExecutionStatusDTO `json:"data"`
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
		req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/tasks/"+core.MustNewID().String(), http.NoBody)
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
		req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/tasks/"+core.MustNewID().String(), http.NoBody)
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ShouldRejectNegativeTimeout", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-one", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-one"},
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
			"/api/v0/tasks/task-one/executions",
			strings.NewReader(`{"timeout":-1}`),
		)
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
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-limit", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-limit"},
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
			"/api/v0/tasks/task-limit/executions",
			strings.NewReader(`{"with":{"foo":"bar"},"timeout":301}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ShouldReturnConflictForDuplicateRequests", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-two", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-two"},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{}
		cleanup := installTaskExecutor(state, stub)
		defer cleanup()
		r := gin.New()
		r.Use(appstate.StateMiddleware(state))
		r.Use(srrouter.ErrorHandler())
		r.Use(func(c *gin.Context) {
			srrouter.SetTaskRepository(c, repo)
			stub := &stubAPIIdempotency{unique: false, reason: "duplicate"}
			srrouter.SetAPIIdempotency(c, stub)
			c.Next()
		})
		api := r.Group("/api/v0")
		Register(api)
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/tasks/task-two/executions",
			strings.NewReader(`{"with":{"foo":"bar"}}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("ShouldExecuteTaskSyncSuccessfully", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-three", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-three"},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{syncOutput: &core.Output{"foo": "bar"}, syncExecID: core.MustNewID()}
		cleanup := installTaskExecutor(state, stub)
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
			"/api/v0/tasks/task-three/executions",
			strings.NewReader(`{"with":{"foo":"bar"}}`),
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
		require.Equal(t, "bar", payload.Data.Output["foo"])
	})

	t.Run("ShouldHandleTaskSyncTimeouts", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-four", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-four"},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{syncExecID: core.MustNewID(), syncErr: context.DeadlineExceeded}
		cleanup := installTaskExecutor(state, stub)
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
			"/api/v0/tasks/task-four/executions",
			strings.NewReader(`{"with":{"a":1}}`),
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

	t.Run("ShouldExecuteTaskAsyncSuccessfully", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-five", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-five"},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{asyncExecID: core.MustNewID()}
		cleanup := installTaskExecutor(state, stub)
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
			"/api/v0/tasks/task-five/executions/async",
			strings.NewReader(`{"with":{"b":2}}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusAccepted, w.Code)
		expectedLocation := routertest.ComposeLocation("tasks", stub.asyncExecID)
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
		Component:      core.ComponentTask,
		Status:         core.StatusSuccess,
		TaskID:         "task-one",
		TaskExecID:     execID,
		WorkflowID:     "workflow-one",
		WorkflowExecID: workflowExecID,
		Output:         &output,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
