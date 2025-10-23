package tkrouter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/pubsub"
	appstate "github.com/compozy/compozy/engine/infra/server/appstate"
	routerpkg "github.com/compozy/compozy/engine/infra/server/router"
	routertest "github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	taskdomain "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/test/helpers"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
)

func newTaskStreamRouter(t *testing.T, repo taskdomain.Repository, state *appstate.State) *gin.Engine {
	t.Helper()
	ginmode.EnsureGinTestMode()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := logger.ContextWithLogger(c.Request.Context(), logger.NewForTests())
		manager := config.NewManager(t.Context(), config.NewService())
		_, err := manager.Load(t.Context(), config.NewDefaultProvider())
		require.NoError(t, err)
		ctx = config.ContextWithManager(ctx, manager)
		c.Request = c.Request.WithContext(ctx)
		routerpkg.SetTaskRepository(c, repo)
		c.Next()
	})
	r.Use(appstate.StateMiddleware(state))
	r.Use(routerpkg.ErrorHandler())
	api := r.Group("/api/v0")
	Register(api)
	return r
}

func TestStreamTask_InvalidLastEventID(t *testing.T) {
	t.Run("Should reject invalid Last-Event-ID header", func(t *testing.T) {
		state := routertest.NewTestAppState(t)
		resourceStore := routertest.NewResourceStore(state)
		repo := routertest.NewStubTaskRepo()
		storeTaskConfig(t, resourceStore, state.ProjectConfig.Name, newStructuredTaskConfig())
		r := newTaskStreamRouter(t, repo, state)
		execID := core.MustNewID()
		repo.AddState(newRunningTaskState(execID, core.MustNewID(), "demo-task"))
		req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/tasks/"+execID.String()+"/stream", http.NoBody)
		req.Header.Set("Last-Event-ID", "invalid")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code)
		require.Contains(t, res.Body.String(), "Last-Event-ID")
	})
}

func TestStreamTask_StructuredStream(t *testing.T) {
	t.Run("Should stream structured status and completion", func(t *testing.T) {
		state := routertest.NewTestAppState(t)
		resourceStore := routertest.NewResourceStore(state)
		repo := routertest.NewStubTaskRepo()
		storeTaskConfig(t, resourceStore, state.ProjectConfig.Name, newStructuredTaskConfig())
		r := newTaskStreamRouter(t, repo, state)

		execID := core.MustNewID()
		workflowExecID := core.MustNewID()
		running := newRunningTaskState(execID, workflowExecID, "demo-task")
		repo.AddState(running)
		success := *running
		success.Status = core.StatusSuccess
		success.Output = &core.Output{"result": "ok"}
		success.UpdatedAt = time.Unix(5, 0).UTC()

		time.AfterFunc(80*time.Millisecond, func() {
			repo.AddState(&success)
		})

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v0/executions/tasks/"+execID.String()+"/stream?poll_ms=250",
			http.NoBody,
		)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Logf("response: %d %s", res.Code, res.Body.String())
		}
		require.Equal(t, http.StatusOK, res.Code)
		body := res.Body.String()
		require.Contains(t, body, "event: task_status")
		require.Contains(t, body, "\"status\":\"RUNNING\"")
		require.Contains(t, body, "event: complete")
		require.Contains(t, body, "\"status\":\"SUCCESS\"")
		require.Contains(t, body, "\"result\":\"ok\"")
	})
}

func TestStreamTask_TextStream(t *testing.T) {
	setup := func(t *testing.T) (*gin.Engine, *routertest.StubTaskRepo, *helpers.RedisHelper, core.ID, *taskdomain.State) {
		state := routertest.NewTestAppState(t)
		resourceStore := routertest.NewResourceStore(state)
		repo := routertest.NewStubTaskRepo()
		storeTaskConfig(t, resourceStore, state.ProjectConfig.Name, newTextTaskConfig())
		redisHelper := helpers.NewRedisHelper(t)
		provider, err := pubsub.NewRedisProvider(redisHelper.GetClient())
		require.NoError(t, err)
		state.SetPubSubProvider(provider)
		r := newTaskStreamRouter(t, repo, state)
		execID := core.MustNewID()
		workflowExecID := core.MustNewID()
		running := newRunningTaskState(execID, workflowExecID, "demo-task")
		repo.AddState(running)
		success := *running
		success.Status = core.StatusSuccess
		success.Output = &core.Output{"final": "text"}
		success.UpdatedAt = time.Unix(3, 0).UTC()
		return r, repo, redisHelper, execID, &success
	}

	t.Run("Should stream text chunks and completion", func(t *testing.T) {
		r, repo, redisHelper, execID, success := setup(t)
		t.Cleanup(func() { redisHelper.Cleanup(t) })
		errCh := make(chan error, 1)
		go func() {
			time.Sleep(50 * time.Millisecond)
			channel := redisTokenChannel(taskdomain.DefaultStreamChannelPrefix, execID)
			err := redisHelper.GetClient().Publish(t.Context(), channel, "chunk").Err()
			if err == nil {
				repo.AddState(success)
			}
			errCh <- err
		}()

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v0/executions/tasks/"+execID.String()+"/stream?poll_ms=250",
			http.NoBody,
		)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.NoError(t, <-errCh)
		if res.Code != http.StatusOK {
			t.Logf("response: %d %s", res.Code, res.Body.String())
		}
		require.Equal(t, http.StatusOK, res.Code)
		body := res.Body.String()
		require.Contains(t, body, "event: task_status")
		require.Contains(t, body, "event: llm_chunk")
		require.Contains(t, body, "chunk")
		require.Contains(t, body, "event: complete")
		require.Contains(t, body, "\"status\":\"SUCCESS\"")
	})

	t.Run("Should filter events per query parameter", func(t *testing.T) {
		r, repo, redisHelper, execID, success := setup(t)
		t.Cleanup(func() { redisHelper.Cleanup(t) })
		errCh := make(chan error, 1)
		go func() {
			time.Sleep(50 * time.Millisecond)
			channel := redisTokenChannel(taskdomain.DefaultStreamChannelPrefix, execID)
			err := redisHelper.GetClient().Publish(t.Context(), channel, "chunk").Err()
			if err == nil {
				repo.AddState(success)
			}
			errCh <- err
		}()

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v0/executions/tasks/"+execID.String()+"/stream?poll_ms=250&events="+taskStatusEvent,
			http.NoBody,
		)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.NoError(t, <-errCh)
		require.Equal(t, http.StatusOK, res.Code)
		body := res.Body.String()
		require.Contains(t, body, "event: task_status")
		require.NotContains(t, body, "event: llm_chunk")
		require.NotContains(t, body, "event: complete")
	})
}

func TestStreamTask_TextStreamMissingRedis(t *testing.T) {
	t.Run("Should return service unavailable when Redis missing", func(t *testing.T) {
		state := routertest.NewTestAppState(t)
		resourceStore := routertest.NewResourceStore(state)
		repo := routertest.NewStubTaskRepo()
		storeTaskConfig(t, resourceStore, state.ProjectConfig.Name, newTextTaskConfig())
		r := newTaskStreamRouter(t, repo, state)

		execID := core.MustNewID()
		repo.AddState(newRunningTaskState(execID, core.MustNewID(), "demo-task"))
		req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/tasks/"+execID.String()+"/stream", http.NoBody)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.Equal(t, http.StatusServiceUnavailable, res.Code)
		require.Contains(t, res.Body.String(), "pubsub")
	})
}

func storeTaskConfig(t *testing.T, store resources.ResourceStore, project string, cfg *taskdomain.Config) {
	t.Helper()
	_, err := store.Put(t.Context(), resources.ResourceKey{
		Project: project,
		Type:    resources.ResourceTask,
		ID:      cfg.ID,
	}, cfg)
	require.NoError(t, err)
}

func newStructuredTaskConfig() *taskdomain.Config {
	return &taskdomain.Config{
		BaseConfig: taskdomain.BaseConfig{
			ID:           "demo-task",
			Type:         taskdomain.TaskTypeBasic,
			OutputSchema: &schema.Schema{"type": "object"},
		},
	}
}

func newTextTaskConfig() *taskdomain.Config {
	return &taskdomain.Config{
		BaseConfig: taskdomain.BaseConfig{
			ID:   "demo-task",
			Type: taskdomain.TaskTypeBasic,
		},
	}
}

func newRunningTaskState(execID, workflowExecID core.ID, taskID string) *taskdomain.State {
	return &taskdomain.State{
		TaskExecID:     execID,
		WorkflowExecID: workflowExecID,
		TaskID:         taskID,
		Status:         core.StatusRunning,
		Component:      core.ComponentTask,
		CreatedAt:      time.Unix(0, 0).UTC(),
		UpdatedAt:      time.Unix(0, 0).UTC(),
	}
}
