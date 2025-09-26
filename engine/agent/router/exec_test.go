package agentrouter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	srrouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/testutil"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubTaskRepo struct {
	*testutil.InMemoryRepo
	err error
}

func newStubTaskRepo() *stubTaskRepo {
	return &stubTaskRepo{InMemoryRepo: testutil.NewInMemoryRepo()}
}

func (s *stubTaskRepo) GetState(ctx context.Context, id core.ID) (*task.State, error) {
	if s.err != nil {
		return nil, s.err
	}
	state, err := s.InMemoryRepo.GetState(ctx, id)
	if err != nil {
		return nil, store.ErrTaskNotFound
	}
	return state, nil
}

func Test_getAgentExecutionStatus(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := newTestAppState(t)
	repo := newStubTaskRepo()
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
	req = withConfig(t, req)
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
	state := newTestAppState(t)
	repo := newStubTaskRepo()
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
	req = withConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func Test_getAgentExecutionStatus_RepoError(t *testing.T) {
	ginmode.EnsureGinTestMode()
	state := newTestAppState(t)
	repo := newStubTaskRepo()
	repo.err = errors.New("boom")
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
	req = withConfig(t, req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func newTestAppState(t *testing.T) *appstate.State {
	t.Helper()
	proj := &project.Config{Name: "test"}
	require.NoError(t, proj.SetCWD(t.TempDir()))
	deps := appstate.NewBaseDeps(proj, nil, nil, nil)
	state, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	return state
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

func withConfig(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	manager := config.NewManager(config.NewService())
	_, err := manager.Load(context.Background(), config.NewDefaultProvider())
	require.NoError(t, err)
	ctx := config.ContextWithManager(req.Context(), manager)
	return req.WithContext(ctx)
}
