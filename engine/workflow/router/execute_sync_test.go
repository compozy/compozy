package wfrouter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appstate "github.com/compozy/compozy/engine/infra/server/appstate"
	routerpkg "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
)

type stubWorkflowRunner struct {
	execID   core.ID
	err      error
	calls    int
	lastID   string
	lastReq  *core.Input
	lastTask string
}

func (s *stubWorkflowRunner) TriggerWorkflow(
	_ context.Context,
	workflowID string,
	input *core.Input,
	initTaskID string,
) (*worker.WorkflowInput, error) {
	s.calls++
	s.lastID = workflowID
	s.lastTask = initTaskID
	s.lastReq = input
	if s.err != nil {
		return nil, s.err
	}
	return &worker.WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: s.execID,
		Input:          input,
		InitialTaskID:  initTaskID,
	}, nil
}

type stubWorkflowRepo struct {
	mu     sync.Mutex
	states []*workflow.State
	last   *workflow.State
	err    error
}

func (s *stubWorkflowRepo) ListStates(context.Context, *workflow.StateFilter) ([]*workflow.State, error) {
	return nil, errors.New("not implemented")
}

func (s *stubWorkflowRepo) UpsertState(context.Context, *workflow.State) error {
	return errors.New("not implemented")
}

func (s *stubWorkflowRepo) UpdateStatus(context.Context, core.ID, core.StatusType) error {
	return errors.New("not implemented")
}

func (s *stubWorkflowRepo) GetState(_ context.Context, _ core.ID) (*workflow.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	if len(s.states) == 0 {
		if s.last == nil {
			return nil, store.ErrWorkflowNotFound
		}
		stateCopy := *s.last
		return &stateCopy, nil
	}
	state := s.states[0]
	if len(s.states) > 1 {
		s.states = s.states[1:]
	}
	s.last = state
	if state == nil {
		return nil, store.ErrWorkflowNotFound
	}
	stateCopy := *state
	return &stateCopy, nil
}

func (s *stubWorkflowRepo) GetStateByID(context.Context, string) (*workflow.State, error) {
	return nil, errors.New("not implemented")
}

func (s *stubWorkflowRepo) GetStateByTaskID(context.Context, string, string) (*workflow.State, error) {
	return nil, errors.New("not implemented")
}

func (s *stubWorkflowRepo) GetStateByAgentID(context.Context, string, string) (*workflow.State, error) {
	return nil, errors.New("not implemented")
}

func (s *stubWorkflowRepo) GetStateByToolID(context.Context, string, string) (*workflow.State, error) {
	return nil, errors.New("not implemented")
}

func (s *stubWorkflowRepo) CompleteWorkflow(
	context.Context,
	core.ID,
	workflow.OutputTransformer,
) (*workflow.State, error) {
	return nil, errors.New("not implemented")
}

func (s *stubWorkflowRepo) MergeUsage(_ context.Context, execID core.ID, summary *usage.Summary) error {
	if summary == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, state := range s.states {
		if state != nil && state.WorkflowExecID == execID {
			if state.Usage == nil {
				state.Usage = summary.Clone()
			} else {
				clone := state.Usage.Clone()
				clone.MergeAll(summary)
				clone.Sort()
				state.Usage = clone
			}
			return nil
		}
	}
	if s.last != nil && s.last.WorkflowExecID == execID {
		if s.last.Usage == nil {
			s.last.Usage = summary.Clone()
		} else {
			clone := s.last.Usage.Clone()
			clone.MergeAll(summary)
			clone.Sort()
			s.last.Usage = clone
		}
		return nil
	}
	return errors.New("state not found")
}

func setupWorkflowSyncRouter(
	t *testing.T,
	repo workflow.Repository,
	runner routerpkg.WorkflowRunner,
	store resources.ResourceStore,
) (*gin.Engine, *appstate.State) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	proj := &project.Config{Name: "demo"}
	require.NoError(t, proj.SetCWD(t.TempDir()))
	deps := appstate.NewBaseDeps(proj, nil, nil, nil)
	state, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	if store != nil {
		state.SetResourceStore(store)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := logger.ContextWithLogger(c.Request.Context(), logger.NewForTests())
		manager := config.NewManager(config.NewService())
		_, loadErr := manager.Load(context.Background(), config.NewDefaultProvider())
		require.NoError(t, loadErr)
		ctx = config.ContextWithManager(ctx, manager)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(appstate.StateMiddleware(state))
	r.Use(routerpkg.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if repo != nil {
			routerpkg.SetWorkflowRepository(c, repo)
		}
		if runner != nil {
			routerpkg.SetWorkflowRunner(c, runner)
		}
		c.Next()
	})
	api := r.Group("/api/v0")
	Register(api)
	return r, state
}

func putWorkflowConfig(t *testing.T, store resources.ResourceStore, project string, workflowID string) {
	t.Helper()
	cfg := &workflow.Config{ID: workflowID}
	data, err := cfg.AsMap()
	require.NoError(t, err)
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: project, Type: resources.ResourceWorkflow, ID: workflowID},
		data,
	)
	require.NoError(t, err)
}

func Test_executeWorkflowSync(t *testing.T) {
	prevInitial := workflowPollInitialBackoff
	prevMax := workflowPollMaxBackoff
	prevTimeoutUnit := workflowTimeoutUnit
	workflowPollInitialBackoff = 2 * time.Millisecond
	workflowPollMaxBackoff = 10 * time.Millisecond
	workflowTimeoutUnit = 10 * time.Millisecond
	t.Cleanup(func() {
		workflowPollInitialBackoff = prevInitial
		workflowPollMaxBackoff = prevMax
		workflowTimeoutUnit = prevTimeoutUnit
	})

	workflowID := "sync-demo"
	t.Run("ShouldExecuteWorkflowSyncSuccessfully", func(t *testing.T) {
		execID := core.MustNewID()
		runner := &stubWorkflowRunner{execID: execID}
		completed := workflow.NewState(workflowID, execID, nil).WithStatus(core.StatusSuccess)
		output := core.Output{"result": "ok"}
		completed.Output = &output
		completed.Usage = &usage.Summary{Entries: []usage.Entry{{
			Provider:         "openai",
			Model:            "gpt-4o",
			PromptTokens:     18,
			CompletionTokens: 7,
			TotalTokens:      25,
			Source:           string(usage.SourceWorkflow),
		}}}
		repo := &stubWorkflowRepo{states: []*workflow.State{completed}}
		store := resources.NewMemoryResourceStore()
		r, state := setupWorkflowSyncRouter(t, repo, runner, store)
		putWorkflowConfig(t, store, state.ProjectConfig.Name, workflowID)
		body := `{"input": {"foo": "bar"}}`
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/workflows/"+workflowID+"/executions/sync",
			strings.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Status int `json:"status"`
			Data   struct {
				ExecID   string                `json:"exec_id"`
				Output   *core.Output          `json:"output"`
				Workflow *WorkflowExecutionDTO `json:"workflow"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, execID.String(), resp.Data.ExecID)
		require.NotNil(t, resp.Data.Workflow)
		assert.Equal(t, core.StatusSuccess, resp.Data.Workflow.Status)
		require.NotNil(t, resp.Data.Workflow.Usage)
		require.Len(t, resp.Data.Workflow.Usage.Entries, 1)
		assert.Equal(t, 25, resp.Data.Workflow.Usage.Entries[0].TotalTokens)
		require.NotNil(t, resp.Data.Output)
		assert.Equal(t, "ok", (*resp.Data.Output)["result"])
	})

	t.Run("ShouldReturnTimeoutWhenExecutionDoesNotComplete", func(t *testing.T) {
		execID := core.MustNewID()
		runner := &stubWorkflowRunner{execID: execID}
		running := workflow.NewState(workflowID, execID, nil)
		running.Usage = &usage.Summary{Entries: []usage.Entry{{
			Provider:         "anthropic",
			Model:            "claude-3",
			PromptTokens:     11,
			CompletionTokens: 5,
			TotalTokens:      16,
			Source:           string(usage.SourceWorkflow),
		}}}
		repo := &stubWorkflowRepo{states: []*workflow.State{running}}
		store := resources.NewMemoryResourceStore()
		r, state := setupWorkflowSyncRouter(t, repo, runner, store)
		putWorkflowConfig(t, store, state.ProjectConfig.Name, workflowID)
		body := `{"timeout": 1}`
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/workflows/"+workflowID+"/executions/sync",
			strings.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		start := time.Now()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusRequestTimeout, w.Code)
		assert.Less(t, time.Since(start), 100*time.Millisecond)
		var resp struct {
			Status int            `json:"status"`
			Data   map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		execVal, ok := resp.Data["exec_id"].(string)
		require.True(t, ok)
		assert.Equal(t, execID.String(), execVal)
		_, hasUsage := resp.Data["usage"]
		require.False(t, hasUsage)
		workflowPayload, ok := resp.Data["workflow"].(map[string]any)
		require.True(t, ok)
		usageArr, ok := workflowPayload["usage"].([]any)
		require.True(t, ok)
		require.Len(t, usageArr, 1)
		entry, ok := usageArr[0].(map[string]any)
		require.True(t, ok)
		total, ok := entry["total_tokens"].(float64)
		require.True(t, ok)
		assert.Equal(t, float64(16), total)
	})

	t.Run("ShouldRejectTimeoutAboveLimit", func(t *testing.T) {
		execID := core.MustNewID()
		runner := &stubWorkflowRunner{execID: execID}
		repo := &stubWorkflowRepo{states: []*workflow.State{}}
		store := resources.NewMemoryResourceStore()
		r, state := setupWorkflowSyncRouter(t, repo, runner, store)
		putWorkflowConfig(t, store, state.ProjectConfig.Name, workflowID)
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/workflows/"+workflowID+"/executions/sync",
			strings.NewReader(`{"timeout": 301}`),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ShouldReturnNotFoundWhenWorkflowMissing", func(t *testing.T) {
		execID := core.MustNewID()
		runner := &stubWorkflowRunner{execID: execID}
		repo := &stubWorkflowRepo{states: []*workflow.State{}}
		store := resources.NewMemoryResourceStore()
		r, _ := setupWorkflowSyncRouter(t, repo, runner, store)
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/workflows/"+workflowID+"/executions/sync",
			strings.NewReader(`{}`),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code)
	})
}
