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
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubDirectExecutor struct {
	syncOutput  *core.Output
	syncExecID  core.ID
	syncErr     error
	syncTimeout time.Duration
	asyncExecID core.ID
	asyncErr    error
}

type stateSavingTaskExecutor struct {
	repo  *routertest.StubTaskRepo
	state *task.State
}

type stubUsageRepo struct {
	taskRows     map[string]*usage.Row
	workflowRows map[string]*usage.Row
	summaryRows  map[string]*usage.Row
	err          error
}

func newStubUsageRepo() *stubUsageRepo {
	return &stubUsageRepo{
		taskRows:     make(map[string]*usage.Row),
		workflowRows: make(map[string]*usage.Row),
		summaryRows:  make(map[string]*usage.Row),
	}
}

func (s *stubUsageRepo) Upsert(_ context.Context, row *usage.Row) error {
	if row == nil {
		return nil
	}
	if row.TaskExecID != nil {
		s.taskRows[row.TaskExecID.String()] = row
	}
	if row.WorkflowExecID != nil {
		s.workflowRows[row.WorkflowExecID.String()] = row
		s.summaryRows[row.WorkflowExecID.String()] = row
	}
	return nil
}

func (s *stubUsageRepo) GetByTaskExecID(_ context.Context, id core.ID) (*usage.Row, error) {
	if s.err != nil {
		return nil, s.err
	}
	if row, ok := s.taskRows[id.String()]; ok {
		return row, nil
	}
	return nil, usage.ErrNotFound
}

func (s *stubUsageRepo) GetByWorkflowExecID(_ context.Context, id core.ID) (*usage.Row, error) {
	if s.err != nil {
		return nil, s.err
	}
	if row, ok := s.workflowRows[id.String()]; ok {
		return row, nil
	}
	return nil, usage.ErrNotFound
}

func (s *stubUsageRepo) SummarizeByWorkflowExecID(_ context.Context, id core.ID) (*usage.Row, error) {
	if s.err != nil {
		return nil, s.err
	}
	if row, ok := s.summaryRows[id.String()]; ok {
		return row, nil
	}
	return nil, usage.ErrNotFound
}

func (s *stateSavingTaskExecutor) ExecuteSync(
	_ context.Context,
	_ *task.Config,
	_ *ExecMetadata,
	_ time.Duration,
) (*core.Output, core.ID, error) {
	execID := core.MustNewID()
	clone := *s.state
	clone.TaskExecID = execID
	s.repo.AddState(&clone)
	return nil, execID, nil
}

func (s *stateSavingTaskExecutor) ExecuteAsync(_ context.Context, _ *task.Config, _ *ExecMetadata) (core.ID, error) {
	return core.MustNewID(), nil
}

func (s *stubDirectExecutor) ExecuteSync(
	_ context.Context,
	_ *task.Config,
	_ *ExecMetadata,
	timeout time.Duration,
) (*core.Output, core.ID, error) {
	s.syncTimeout = timeout
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
		usageRepo := newStubUsageRepo()
		usageRepo.taskRows[execID.String()] = &usage.Row{
			Provider:         "openai",
			Model:            "gpt-4o",
			PromptTokens:     12,
			CompletionTokens: 7,
			TotalTokens:      19,
		}
		r := gin.New()
		r.Use(appstate.StateMiddleware(state))
		r.Use(srrouter.ErrorHandler())
		r.Use(func(c *gin.Context) {
			srrouter.SetTaskRepository(c, repo)
			srrouter.SetUsageRepository(c, usageRepo)
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
		require.NotNil(t, payload.Data.Usage)
		require.Equal(t, "openai", payload.Data.Usage.Provider)
		require.Equal(t, 19, payload.Data.Usage.TotalTokens)
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
			"/api/v0/tasks/task-one/executions/sync",
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
			"/api/v0/tasks/task-limit/executions/sync",
			strings.NewReader(`{"with":{"foo":"bar"},"timeout":301}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
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
		output := core.Output{"foo": "bar"}
		stub := &stubDirectExecutor{syncOutput: &output, syncExecID: core.MustNewID()}
		cleanup := installTaskExecutor(state, stub)
		defer cleanup()
		usageRepo := newStubUsageRepo()
		usageRepo.taskRows[stub.syncExecID.String()] = &usage.Row{
			Provider:         "openai",
			Model:            "gpt-4o",
			PromptTokens:     9,
			CompletionTokens: 4,
			TotalTokens:      13,
		}
		stateSnapshot := newTestTaskState(stub.syncExecID)
		matchOutput := core.Output{"foo": "bar"}
		stateSnapshot.Output = &matchOutput
		repo.AddState(stateSnapshot)
		r := gin.New()
		r.Use(appstate.StateMiddleware(state))
		r.Use(srrouter.ErrorHandler())
		r.Use(func(c *gin.Context) {
			srrouter.SetTaskRepository(c, repo)
			srrouter.SetUsageRepository(c, usageRepo)
			c.Next()
		})
		api := r.Group("/api/v0")
		Register(api)
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/tasks/task-three/executions/sync",
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
				ExecID string                 `json:"exec_id"`
				Output core.Output            `json:"output"`
				Usage  *srrouter.UsageSummary `json:"usage"`
				State  TaskExecutionStatusDTO `json:"state"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		require.Equal(t, http.StatusOK, payload.Status)
		require.Equal(t, stub.syncExecID.String(), payload.Data.ExecID)
		require.Equal(t, "bar", payload.Data.Output["foo"])
		require.NotNil(t, payload.Data.Usage)
		require.Equal(t, 13, payload.Data.Usage.TotalTokens)
		require.NotNil(t, payload.Data.State.Usage)
		require.Equal(t, "openai", payload.Data.State.Usage.Provider)
		require.Equal(t, 60*time.Second, stub.syncTimeout)
	})

	t.Run("ShouldRespectTaskConfigTimeout", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{
			BaseConfig: task.BaseConfig{ID: "task-with-timeout", Type: task.TaskTypeBasic, Timeout: "5m"},
		}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{
				Project: state.ProjectConfig.Name,
				Type:    resources.ResourceTask,
				ID:      "task-with-timeout",
			},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{syncExecID: core.MustNewID()}
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
			"/api/v0/tasks/task-with-timeout/executions/sync",
			strings.NewReader(`{"with":{"foo":"bar"}}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, 5*time.Minute, stub.syncTimeout)
	})

	t.Run("ShouldRespectProjectRuntimeTimeoutDefaults", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		state.ProjectConfig.Runtime.TaskExecutionTimeoutDefault = 2 * time.Minute
		state.ProjectConfig.Runtime.TaskExecutionTimeoutMax = 10 * time.Minute
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-project-timeout", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{
				Project: state.ProjectConfig.Name,
				Type:    resources.ResourceTask,
				ID:      "task-project-timeout",
			},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{syncExecID: core.MustNewID()}
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
			"/api/v0/tasks/task-project-timeout/executions/sync",
			strings.NewReader(`{"with":{"foo":"bar"}}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, 2*time.Minute, stub.syncTimeout)
	})

	t.Run("ShouldRejectTimeoutAboveConfiguredMax", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		state.ProjectConfig.Runtime.TaskExecutionTimeoutDefault = 20 * time.Second
		state.ProjectConfig.Runtime.TaskExecutionTimeoutMax = 45 * time.Second
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-max", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-max"},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{syncExecID: core.MustNewID()}
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
			"/api/v0/tasks/task-max/executions/sync",
			strings.NewReader(`{"timeout":50}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ShouldRejectTaskConfigTimeoutAboveMax", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		state.ProjectConfig.Runtime.TaskExecutionTimeoutDefault = 30 * time.Second
		state.ProjectConfig.Runtime.TaskExecutionTimeoutMax = 45 * time.Second
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{
			BaseConfig: task.BaseConfig{ID: "task-config-max", Type: task.TaskTypeBasic, Timeout: "2m"},
		}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{
				Project: state.ProjectConfig.Name,
				Type:    resources.ResourceTask,
				ID:      "task-config-max",
			},
			m,
		)
		require.NoError(t, err)
		stub := &stubDirectExecutor{syncExecID: core.MustNewID()}
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
			"/api/v0/tasks/task-config-max/executions/sync",
			strings.NewReader(`{"with":{"foo":"bar"}}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ShouldUseStateOutputWhenExecutorReturnsNil", func(t *testing.T) {
		ginmode.EnsureGinTestMode()
		state := routertest.NewTestAppState(t)
		repo := routertest.NewStubTaskRepo()
		store := routertest.NewResourceStore(state)
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "task-state", Type: task.TaskTypeBasic}}
		m, err := taskCfg.AsMap()
		require.NoError(t, err)
		_, err = store.Put(
			context.Background(),
			resources.ResourceKey{Project: state.ProjectConfig.Name, Type: resources.ResourceTask, ID: "task-state"},
			m,
		)
		require.NoError(t, err)
		fallback := core.Output{"foo": "from-state"}
		stateSnapshot := newTestTaskState(core.MustNewID())
		stateSnapshot.Output = &fallback
		stub := &stateSavingTaskExecutor{repo: repo, state: stateSnapshot}
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
			"/api/v0/tasks/task-state/executions/sync",
			strings.NewReader(`{"with":{"foo":"fallback"}}`),
		)
		req.Header.Set("Content-Type", "application/json")
		req = routertest.WithConfig(t, req)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var payload struct {
			Status int `json:"status"`
			Data   struct {
				ExecID string                 `json:"exec_id"`
				Output map[string]any         `json:"output"`
				State  TaskExecutionStatusDTO `json:"state"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		require.Equal(t, http.StatusOK, payload.Status)
		require.Equal(t, fallback["foo"], payload.Data.Output["foo"])
		require.NotNil(t, payload.Data.State.Output)
		require.Equal(t, fallback["foo"], (*payload.Data.State.Output)["foo"])
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
			"/api/v0/tasks/task-four/executions/sync",
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
			"/api/v0/tasks/task-five/executions",
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
