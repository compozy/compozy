package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/repo"
	serverpkg "github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	lgmiddleware "github.com/compozy/compozy/engine/infra/server/middleware/logger"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	taskdomain "github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/test/helpers"
	ctxhelpers "github.com/compozy/compozy/test/helpers/ctx"
	"github.com/compozy/compozy/test/helpers/ginmode"
	serverhelpers "github.com/compozy/compozy/test/helpers/server"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workflowRunnerInjector struct {
	stub *workflowRunnerStub
}

func (i *workflowRunnerInjector) Middleware(c *gin.Context) {
	if i.stub != nil {
		i.stub.markAttached()
		router.SetWorkflowRunner(c, i.stub)
	}
	c.Next()
}

func newServerHarnessWithMiddleware(t *testing.T, extra ...gin.HandlerFunc) *serverhelpers.ServerHarness {
	t.Helper()
	ctx := ctxhelpers.TestContext(t)
	manager := config.NewManager(config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	ctx = config.ContextWithManager(ctx, manager)
	cfg := manager.Get()
	cfg.Server.Auth.Enabled = false
	cfg.Server.CORSEnabled = false
	cfg.Server.SourceOfTruth = "repo"
	cfg.Server.Timeouts.StartProbeDelay = time.Millisecond
	cfg.RateLimit.GlobalRate.Limit = 0
	cfg.RateLimit.APIKeyRate.Limit = 0
	cfg.Webhooks.DefaultMaxBody = 1 << 20
	cfg.Webhooks.DefaultDedupeTTL = time.Minute
	tempDir := t.TempDir()
	projFile := filepath.Join(tempDir, "compozy.yaml")
	projectName := strings.ToLower(t.Name())
	projectName = strings.ReplaceAll(projectName, " ", "-")
	projectName = strings.ReplaceAll(projectName, "/", "-")
	projectName = strings.ReplaceAll(projectName, "\\", "-")
	projectName = strings.Trim(projectName, "-")
	if projectName == "" {
		projectName = fmt.Sprintf("project-%d", time.Now().UnixNano())
	}
	yamlContent := fmt.Sprintf("name: %s\nversion: 1.0.0\n", projectName)
	require.NoError(t, os.WriteFile(projFile, []byte(yamlContent), 0o600))
	proj := &project.Config{Name: projectName, Version: "1.0.0"}
	require.NoError(t, proj.SetCWD(tempDir))
	proj.SetFilePath(projFile)
	pool, cleanup := helpers.GetSharedPostgresDB(ctx, t)
	t.Cleanup(cleanup)
	require.NoError(t, helpers.EnsureTablesExistForTest(pool))
	cfg.Database.ConnString = pool.Config().ConnString()
	cfg.Database.AutoMigrate = false
	deps := appstate.NewBaseDeps(proj, nil, repo.NewProvider(pool), nil)
	state, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	store := resources.NewMemoryResourceStore()
	state.SetResourceStore(store)
	ginmode.EnsureGinTestMode()
	srv, err := serverpkg.NewServer(ctx, tempDir, projFile, "")
	require.NoError(t, err)
	r := gin.New()
	r.Use(gin.Recovery())
	for _, mw := range extra {
		if mw != nil {
			r.Use(mw)
		}
	}
	authFactory := authuc.NewFactory(state.Store.NewAuthRepo())
	authManager := authmw.NewManager(authFactory, cfg)
	r.Use(authManager.Middleware())
	r.Use(lgmiddleware.Middleware(ctx))
	r.Use(appstate.StateMiddleware(state))
	r.Use(router.ErrorHandler())
	require.NoError(t, serverpkg.RegisterRoutes(ctx, r, state, srv))
	return &serverhelpers.ServerHarness{
		Engine:        r,
		State:         state,
		Ctx:           ctx,
		Config:        cfg,
		ResourceStore: store,
		Project:       proj,
		Server:        srv,
		DB:            pool,
	}
}

func TestExecutionEndpoints_WorkflowSync(t *testing.T) {
	t.Run("Should execute workflow synchronously and return final state", func(t *testing.T) {
		injector := &workflowRunnerInjector{}
		harness := newServerHarnessWithMiddleware(t, injector.Middleware)
		ctx := ctxhelpers.TestContext(t)
		workflowID := "wf-sync-success"
		putWorkflowResource(t, harness.Engine, workflowID)
		repo := harness.State.Store.NewWorkflowRepo()
		stub := newWorkflowRunnerStub(repo, workflowRunnerConfig{
			completeDelay: 10 * time.Millisecond,
			output:        core.Output{"message": "done"},
		})
		injector.stub = stub
		payload := map[string]any{
			"input": map[string]any{"name": "World"},
		}
		res := doRequest(
			t,
			harness.Engine,
			http.MethodPost,
			fmt.Sprintf("/api/v0/workflows/%s/executions/sync", workflowID),
			payload,
			nil,
		)
		require.Equal(t, http.StatusOK, res.Code)
		require.Greater(t, stub.attachedCount(), 0)
		envelope := decodeEnvelope(t, res)
		data := envelope.Data
		wfData, ok := data["workflow"].(map[string]any)
		require.True(t, ok)
		status, ok := wfData["status"].(string)
		require.True(t, ok)
		assert.Equal(t, string(core.StatusSuccess), status)
		execID, ok := data["exec_id"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, execID)
		stub.assertCalls(t, 1)
		state, err := repo.GetState(context.WithoutCancel(ctx), stub.lastExecID())
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.Equal(t, core.StatusSuccess, state.Status)
	})

	t.Run("Should return timeout response when workflow exceeds caller deadline", func(t *testing.T) {
		injector := &workflowRunnerInjector{}
		harness := newServerHarnessWithMiddleware(t, injector.Middleware)
		workflowID := "wf-sync-timeout"
		putWorkflowResource(t, harness.Engine, workflowID)
		repo := harness.State.Store.NewWorkflowRepo()
		stub := newWorkflowRunnerStub(repo, workflowRunnerConfig{
			completeDelay: 1250 * time.Millisecond,
			output:        core.Output{"message": "late"},
		})
		injector.stub = stub
		payload := map[string]any{
			"input":   map[string]any{"slow": true},
			"timeout": 1,
		}
		start := time.Now()
		res := doRequest(
			t,
			harness.Engine,
			http.MethodPost,
			fmt.Sprintf("/api/v0/workflows/%s/executions/sync", workflowID),
			payload,
			nil,
		)
		require.Equal(t, http.StatusRequestTimeout, res.Code)
		require.Greater(t, stub.attachedCount(), 0)
		require.GreaterOrEqual(t, time.Since(start), time.Second)
		envelope := decodeEnvelope(t, res)
		data := envelope.Data
		execID, ok := data["exec_id"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, execID)
		workflowPayload, hasWorkflow := data["workflow"].(map[string]any)
		assert.True(t, hasWorkflow)
		if hasWorkflow {
			status, ok := workflowPayload["status"].(string)
			require.True(t, ok)
			assert.Equal(t, string(core.StatusRunning), status)
		}
		stub.assertCalls(t, 1)
		waitForWorkflowStatus(t, repo, stub.lastExecID(), core.StatusSuccess, 3*time.Second)
	})
}

func TestExecutionEndpoints_AgentAsync(t *testing.T) {
	harness := newServerHarnessWithMiddleware(t)
	agentID := "agent-async"
	putAgentResource(t, harness.Engine, agentID)
	repo := harness.State.Store.NewTaskRepo()
	wfRepo := harness.State.Store.NewWorkflowRepo()
	stub := newDirectExecutorStub(repo, wfRepo, directExecutorConfig{
		syncOutput:  core.Output{"result": "sync"},
		asyncDelay:  20 * time.Millisecond,
		asyncOutput: core.Output{"result": "async"},
	})
	installDirectExecutorStub(t, harness.State, stub)
	body := map[string]any{
		"prompt": "Review this",
		"with":   map[string]any{"code": "package main"},
	}
	res := doRequest(
		t,
		harness.Engine,
		http.MethodPost,
		fmt.Sprintf("/api/v0/agents/%s/executions", agentID),
		body,
		nil,
	)
	require.Equal(t, http.StatusAccepted, res.Code)
	assert.Contains(t, res.Header().Get("Location"), "/api/v0/executions/agents/")
	envelope := decodeEnvelope(t, res)
	execIDStr, ok := envelope.Data["exec_id"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, execIDStr)
	statusPath := fmt.Sprintf("/api/v0/executions/agents/%s", execIDStr)
	statusData := pollExecutionStatus(t, harness.Engine, statusPath, core.ComponentAgent, 2*time.Second)
	assert.Equal(t, string(core.StatusSuccess), statusData["status"].(string))
	output, ok := statusData["output"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "async", output["result"].(string))
	stub.assertAsyncCalls(t, 1)
}

func TestExecutionEndpoints_TaskAsync(t *testing.T) {
	harness := newServerHarnessWithMiddleware(t)
	taskID := "task-async"
	putTaskResource(t, harness.Engine, taskID)
	repo := harness.State.Store.NewTaskRepo()
	wfRepo := harness.State.Store.NewWorkflowRepo()
	stub := newDirectExecutorStub(repo, wfRepo, directExecutorConfig{
		syncOutput:  core.Output{"result": "sync"},
		asyncDelay:  25 * time.Millisecond,
		asyncOutput: core.Output{"result": "task"},
	})
	installDirectExecutorStub(t, harness.State, stub)
	body := map[string]any{
		"with": map[string]any{"input": "value"},
	}
	res := doRequest(
		t,
		harness.Engine,
		http.MethodPost,
		fmt.Sprintf("/api/v0/tasks/%s/executions", taskID),
		body,
		nil,
	)
	require.Equal(t, http.StatusAccepted, res.Code)
	location := res.Header().Get("Location")
	assert.Contains(t, location, "/api/v0/executions/tasks/")
	envelope := decodeEnvelope(t, res)
	execIDStr := envelope.Data["exec_id"].(string)
	statusPath := fmt.Sprintf("/api/v0/executions/tasks/%s", execIDStr)
	statusData := pollExecutionStatus(t, harness.Engine, statusPath, core.ComponentTask, 2*time.Second)
	assert.Equal(t, string(core.StatusSuccess), statusData["status"].(string))
	output := statusData["output"].(map[string]any)
	assert.Equal(t, "task", output["result"].(string))
	stub.assertAsyncCalls(t, 1)
}

func doRequest(
	t *testing.T,
	engine *gin.Engine,
	method string,
	path string,
	payload any,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res := httptest.NewRecorder()
	engine.ServeHTTP(res, req)
	return res
}

func decodeEnvelope(t *testing.T, res *httptest.ResponseRecorder) responseEnvelope {
	t.Helper()
	var env responseEnvelope
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &env))
	return env
}

type responseEnvelope struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
	Error   map[string]any `json:"error"`
}

func putWorkflowResource(t *testing.T, engine *gin.Engine, id string) {
	t.Helper()
	payload := map[string]any{
		"id":          id,
		"description": "workflow",
		"config":      map[string]any{},
		"tasks":       []map[string]any{},
		"agents":      []map[string]any{},
		"tools":       []map[string]any{},
	}
	res := doRequest(t, engine, http.MethodPut, fmt.Sprintf("/api/v0/workflows/%s", id), payload, nil)
	require.Contains(t, []int{http.StatusCreated, http.StatusOK}, res.Code)
}

func putAgentResource(t *testing.T, engine *gin.Engine, id string) {
	t.Helper()
	payload := map[string]any{
		"id":           id,
		"instructions": "Do things",
		"model": map[string]any{
			"provider": "openai",
			"model":    "gpt-4o-mini",
		},
	}
	res := doRequest(t, engine, http.MethodPut, fmt.Sprintf("/api/v0/agents/%s", id), payload, nil)
	require.Contains(t, []int{http.StatusCreated, http.StatusOK}, res.Code)
}

func putTaskResource(t *testing.T, engine *gin.Engine, id string) {
	t.Helper()
	payload := map[string]any{
		"id":           id,
		"type":         "basic",
		"instructions": "Process input",
		"model": map[string]any{
			"provider": "openai",
			"model":    "gpt-4o-mini",
		},
	}
	res := doRequest(t, engine, http.MethodPut, fmt.Sprintf("/api/v0/tasks/%s", id), payload, nil)
	require.Contains(t, []int{http.StatusCreated, http.StatusOK}, res.Code)
}

func pollExecutionStatus(
	t *testing.T,
	engine *gin.Engine,
	path string,
	component core.ComponentType,
	timeout time.Duration,
) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		res := doRequest(t, engine, http.MethodGet, path, nil, nil)
		if res.Code == http.StatusOK {
			env := decodeEnvelope(t, res)
			statusData := env.Data
			comp, ok := statusData["component"].(string)
			require.True(t, ok)
			if comp == string(component) {
				status, ok := statusData["status"].(string)
				require.True(t, ok)
				if status == string(core.StatusSuccess) {
					return statusData
				}
			}
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "execution did not reach success state before timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForWorkflowStatus(
	t *testing.T,
	repo workflow.Repository,
	execID core.ID,
	desired core.StatusType,
	timeout time.Duration,
) {
	t.Helper()
	baseCtx := ctxhelpers.TestContext(t)
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	t.Cleanup(cancel)
	for {
		state, err := repo.GetState(context.WithoutCancel(ctx), execID)
		if err == nil && state != nil && state.Status == desired {
			return
		}
		select {
		case <-ctx.Done():
			require.FailNow(t, "workflow did not reach desired status before timeout")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}

type workflowRunnerConfig struct {
	completeDelay time.Duration
	output        core.Output
}

type workflowRunnerStub struct {
	mu       sync.Mutex
	repo     workflow.Repository
	record   []workflowCall
	cfg      workflowRunnerConfig
	attached int
}

type workflowCall struct {
	workflowID string
	execID     core.ID
	input      *core.Input
}

func newWorkflowRunnerStub(repo workflow.Repository, cfg workflowRunnerConfig) *workflowRunnerStub {
	return &workflowRunnerStub{repo: repo, cfg: cfg}
}

func (s *workflowRunnerStub) TriggerWorkflow(
	ctx context.Context,
	workflowID string,
	input *core.Input,
	initTaskID string,
) (*worker.WorkflowInput, error) {
	execID := core.MustNewID()
	state := workflow.NewState(workflowID, execID, input)
	state.Status = core.StatusRunning
	if err := s.repo.UpsertState(context.WithoutCancel(ctx), state); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.record = append(s.record, workflowCall{workflowID: workflowID, execID: execID, input: input})
	delay := s.cfg.completeDelay
	output := s.cfg.output
	s.mu.Unlock()
	if delay >= 0 {
		go s.complete(ctx, workflowID, execID, output, delay)
	}
	return &worker.WorkflowInput{
		WorkflowID:     workflowID,
		WorkflowExecID: execID,
		Input:          input,
		InitialTaskID:  initTaskID,
	}, nil
}

func (s *workflowRunnerStub) complete(
	ctx context.Context,
	workflowID string,
	execID core.ID,
	output core.Output,
	delay time.Duration,
) {
	time.Sleep(delay)
	state, err := s.repo.GetState(context.WithoutCancel(ctx), execID)
	if err != nil || state == nil {
		state = workflow.NewState(workflowID, execID, nil)
	}
	state.Status = core.StatusSuccess
	if len(output) > 0 {
		copyOutput := make(core.Output)
		maps.Copy(copyOutput, output)
		state.Output = &copyOutput
	}
	if err := s.repo.UpsertState(context.WithoutCancel(ctx), state); err != nil {
		logger.FromContext(ctx).Warn("failed to mark workflow complete", "error", err)
	}
}

func (s *workflowRunnerStub) assertCalls(t *testing.T, expected int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Len(t, s.record, expected)
}

func (s *workflowRunnerStub) lastExecID() core.ID {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.record) == 0 {
		return core.ID("")
	}
	return s.record[len(s.record)-1].execID
}

func (s *workflowRunnerStub) markAttached() {
	s.mu.Lock()
	s.attached++
	s.mu.Unlock()
}

func (s *workflowRunnerStub) attachedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attached
}

type directExecutorConfig struct {
	syncOutput  core.Output
	asyncDelay  time.Duration
	asyncOutput core.Output
}

type directExecutorStub struct {
	mu           sync.Mutex
	repo         taskdomain.Repository
	workflowRepo workflow.Repository
	cfg          directExecutorConfig
	syncCalls    int
	asyncCalls   int
	lastSyncID   core.ID
	lastAsyncID  core.ID
}

func newDirectExecutorStub(
	repo taskdomain.Repository,
	wfRepo workflow.Repository,
	cfg directExecutorConfig,
) *directExecutorStub {
	return &directExecutorStub{repo: repo, workflowRepo: wfRepo, cfg: cfg}
}

func (s *directExecutorStub) ExecuteSync(
	ctx context.Context,
	cfg *taskdomain.Config,
	meta *tkrouter.ExecMetadata,
	_ time.Duration,
) (*core.Output, core.ID, error) {
	execID := core.MustNewID()
	now := time.Now().UTC()
	state := &taskdomain.State{
		Component:      meta.Component,
		Status:         core.StatusSuccess,
		TaskID:         cfg.ID,
		TaskExecID:     execID,
		WorkflowID:     meta.WorkflowID,
		WorkflowExecID: execID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if cfg.With != nil {
		state.Input = cfg.With
	}
	if len(s.cfg.syncOutput) > 0 {
		copyOutput := make(core.Output)
		maps.Copy(copyOutput, s.cfg.syncOutput)
		state.Output = &copyOutput
	}
	s.ensureWorkflowState(ctx, meta.WorkflowID, execID)
	if err := s.repo.UpsertState(context.WithoutCancel(ctx), state); err != nil {
		return nil, execID, err
	}
	s.mu.Lock()
	s.syncCalls++
	s.lastSyncID = execID
	s.mu.Unlock()
	s.finishWorkflowState(ctx, execID, s.cfg.syncOutput)
	if len(s.cfg.syncOutput) > 0 {
		outCopy := make(core.Output)
		maps.Copy(outCopy, s.cfg.syncOutput)
		return &outCopy, execID, nil
	}
	return nil, execID, nil
}

func (s *directExecutorStub) ExecuteAsync(
	ctx context.Context,
	cfg *taskdomain.Config,
	meta *tkrouter.ExecMetadata,
) (core.ID, error) {
	execID := core.MustNewID()
	now := time.Now().UTC()
	state := &taskdomain.State{
		Component:      meta.Component,
		Status:         core.StatusRunning,
		TaskID:         cfg.ID,
		TaskExecID:     execID,
		WorkflowID:     meta.WorkflowID,
		WorkflowExecID: execID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if cfg.With != nil {
		state.Input = cfg.With
	}
	s.ensureWorkflowState(ctx, meta.WorkflowID, execID)
	if err := s.repo.UpsertState(context.WithoutCancel(ctx), state); err != nil {
		return execID, err
	}
	s.mu.Lock()
	s.asyncCalls++
	s.lastAsyncID = execID
	delay := s.cfg.asyncDelay
	output := s.cfg.asyncOutput
	s.mu.Unlock()
	if delay < 0 {
		return execID, nil
	}
	go s.completeAsync(ctx, execID, output, delay)
	return execID, nil
}

func (s *directExecutorStub) completeAsync(
	ctx context.Context,
	execID core.ID,
	output core.Output,
	delay time.Duration,
) {
	time.Sleep(delay)
	now := time.Now().UTC()
	state, err := s.repo.GetState(context.WithoutCancel(ctx), execID)
	if err != nil || state == nil {
		state = &taskdomain.State{TaskExecID: execID, WorkflowExecID: execID, CreatedAt: now}
	}
	state.Status = core.StatusSuccess
	if len(output) > 0 {
		copyOutput := make(core.Output)
		maps.Copy(copyOutput, output)
		state.Output = &copyOutput
	}
	state.UpdatedAt = now
	if err := s.repo.UpsertState(context.WithoutCancel(ctx), state); err != nil {
		logger.FromContext(ctx).Warn("failed to mark task execution complete", "error", err)
	}
	s.finishWorkflowState(ctx, execID, output)
}

func (s *directExecutorStub) ensureWorkflowState(ctx context.Context, workflowID string, execID core.ID) {
	if s.workflowRepo == nil {
		return
	}
	if workflowID == "" {
		workflowID = execID.String()
	}
	state := workflow.NewState(workflowID, execID, nil)
	state.Status = core.StatusRunning
	_ = s.workflowRepo.UpsertState(context.WithoutCancel(ctx), state)
}

func (s *directExecutorStub) finishWorkflowState(ctx context.Context, execID core.ID, output core.Output) {
	if s.workflowRepo == nil {
		return
	}
	state, err := s.workflowRepo.GetState(context.WithoutCancel(ctx), execID)
	if err != nil || state == nil {
		return
	}
	state.Status = core.StatusSuccess
	if len(output) > 0 {
		copyOutput := make(core.Output)
		maps.Copy(copyOutput, output)
		state.Output = &copyOutput
	}
	_ = s.workflowRepo.UpsertState(context.WithoutCancel(ctx), state)
}

func (s *directExecutorStub) assertAsyncCalls(t *testing.T, expected int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Equal(t, expected, s.asyncCalls)
}

func installDirectExecutorStub(t *testing.T, state *appstate.State, stub *directExecutorStub) {
	t.Helper()
	tkrouter.SetDirectExecutorFactory(
		state,
		func(*appstate.State, taskdomain.Repository) (tkrouter.DirectExecutor, error) {
			return stub, nil
		},
	)
	t.Cleanup(func() {
		tkrouter.SetDirectExecutorFactory(state, nil)
	})
}
