package agentrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/pubsub"
	appstate "github.com/compozy/compozy/engine/infra/server/appstate"
	routerpkg "github.com/compozy/compozy/engine/infra/server/router"
	routertest "github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/test/helpers"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
)

func newAgentStreamRouter(t *testing.T, repo *routertest.StubTaskRepo, state *appstate.State) *gin.Engine {
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

func TestStreamAgent_InvalidLastEventID(t *testing.T) {
	state := routertest.NewTestAppState(t)
	resourceStore := routertest.NewResourceStore(state)
	repo := routertest.NewStubTaskRepo()
	outputSchema := &schema.Schema{"type": "object"}
	agentCfg := &agent.Config{
		ID: "demo-agent",
		Actions: []*agent.ActionConfig{{
			ID:           "structured",
			OutputSchema: outputSchema,
		}},
	}
	_, err := resourceStore.Put(context.Background(), resources.ResourceKey{
		Project: state.ProjectConfig.Name,
		Type:    resources.ResourceAgent,
		ID:      agentCfg.ID,
	}, agentCfg)
	require.NoError(t, err)
	router := newAgentStreamRouter(t, repo, state)
	execID := core.MustNewID()
	repo.AddState(&task.State{
		TaskExecID:     execID,
		WorkflowExecID: core.MustNewID(),
		Status:         core.StatusRunning,
		AgentID:        strPtr("demo-agent"),
		ActionID:       strPtr("structured"),
		CreatedAt:      time.Unix(0, 0).UTC(),
		UpdatedAt:      time.Unix(0, 0).UTC(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/agents/"+execID.String()+"/stream", http.NoBody)
	req.Header.Set("Last-Event-ID", "invalid")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.Contains(t, res.Body.String(), "Last-Event-ID")
}

func TestStreamAgent_StructuredStream(t *testing.T) {
	state := routertest.NewTestAppState(t)
	projectName := state.ProjectConfig.Name
	resourceStore := routertest.NewResourceStore(state)
	repo := routertest.NewStubTaskRepo()
	router := newAgentStreamRouter(t, repo, state)

	outputSchema := &schema.Schema{"type": "object"}
	agentCfg := &agent.Config{
		ID: "demo-agent",
		Actions: []*agent.ActionConfig{{
			ID:           "structured",
			OutputSchema: outputSchema,
		}},
	}
	_, err := resourceStore.Put(context.Background(), resources.ResourceKey{
		Project: projectName,
		Type:    resources.ResourceAgent,
		ID:      agentCfg.ID,
	}, agentCfg)
	require.NoError(t, err)

	execID := core.MustNewID()
	workflowExecID := core.MustNewID()
	createdAt := time.Unix(0, 0).UTC()
	runningState := &task.State{
		TaskExecID:     execID,
		WorkflowExecID: workflowExecID,
		Status:         core.StatusRunning,
		AgentID:        strPtr("demo-agent"),
		ActionID:       strPtr("structured"),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	repo.AddState(runningState)

	successState := *runningState
	successState.Status = core.StatusSuccess
	successState.Output = &core.Output{"message": "done"}
	successState.UpdatedAt = time.Unix(5, 0).UTC()

	time.AfterFunc(80*time.Millisecond, func() {
		repo.AddState(&successState)
	})

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v0/executions/agents/"+execID.String()+"/stream?poll_ms=250",
		http.NoBody,
	)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Logf("response: %d %s", res.Code, res.Body.String())
	}

	require.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	require.Contains(t, body, "event: agent_status")
	require.Contains(t, body, "\"status\":\"RUNNING\"")
	require.Contains(t, body, "event: complete")
	require.Contains(t, body, "\"status\":\"SUCCESS\"")
	require.Contains(t, body, "\"message\":\"done\"")
}

func TestStreamAgent_TextStream(t *testing.T) {
	state := routertest.NewTestAppState(t)
	_ = routertest.NewResourceStore(state)
	redisHelper := helpers.NewRedisHelper(t)
	defer redisHelper.Cleanup(t)
	provider, err := pubsub.NewRedisProvider(redisHelper.GetClient())
	require.NoError(t, err)
	state.SetPubSubProvider(provider)

	repo := routertest.NewStubTaskRepo()
	router := newAgentStreamRouter(t, repo, state)

	execID := core.MustNewID()
	workflowExecID := core.MustNewID()
	createdAt := time.Unix(0, 0).UTC()
	runningState := &task.State{
		TaskExecID:     execID,
		WorkflowExecID: workflowExecID,
		Status:         core.StatusRunning,
		AgentID:        strPtr("demo-agent"),
		ActionID:       strPtr(promptActionID),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	repo.AddState(runningState)

	successState := *runningState
	successState.Status = core.StatusSuccess
	successState.Output = &core.Output{"text": "final"}
	successState.UpdatedAt = time.Unix(3, 0).UTC()

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		channel := redisTokenChannel(execID)
		err := redisHelper.GetClient().Publish(context.Background(), channel, "hello").Err()
		if err == nil {
			repo.AddState(&successState)
		}
		errCh <- err
	}()

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v0/executions/agents/"+execID.String()+"/stream?poll_ms=250",
		http.NoBody,
	)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Logf("response: %d %s", res.Code, res.Body.String())
	}
	require.NoError(t, <-errCh)

	require.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	require.Contains(t, body, "event: agent_status")
	require.Contains(t, body, "event: llm_chunk")
	require.Contains(t, body, "hello")
	require.Contains(t, body, "event: complete")
	require.Contains(t, body, "\"status\":\"SUCCESS\"")
}

func TestStreamAgent_TextStreamMissingRedis(t *testing.T) {
	state := routertest.NewTestAppState(t)
	routertest.NewResourceStore(state)
	repo := routertest.NewStubTaskRepo()
	router := newAgentStreamRouter(t, repo, state)

	execID := core.MustNewID()
	runningState := &task.State{
		TaskExecID:     execID,
		WorkflowExecID: core.MustNewID(),
		Status:         core.StatusRunning,
		AgentID:        strPtr("demo-agent"),
		ActionID:       strPtr(promptActionID),
		CreatedAt:      time.Unix(0, 0).UTC(),
		UpdatedAt:      time.Unix(0, 0).UTC(),
	}
	repo.AddState(runningState)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/executions/agents/"+execID.String()+"/stream", http.NoBody)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusServiceUnavailable, res.Code)
	require.Contains(t, res.Body.String(), "pubsub")
}

func strPtr(value string) *string {
	return &value
}
