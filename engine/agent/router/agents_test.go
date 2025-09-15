package agentrouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	appstatepkg "github.com/compozy/compozy/engine/infra/server/appstate"
	router "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRouterWithState creates a test gin router with app state middleware installed.
func setupRouterWithState(t *testing.T, state *appstatepkg.State) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(appstatepkg.StateMiddleware(state))
	api := r.Group("/api/v0")
	Register(api)
	return r
}

func Test_listAgents_Handler(t *testing.T) {
	t.Run("Should list agents from app state", func(t *testing.T) {
		// Minimal project + state
		proj := &project.Config{}
		require.NoError(t, proj.SetCWD("/tmp"))
		st, err := appstatepkg.NewState(
			appstatepkg.NewBaseDeps(proj, []*workflow.Config{{Agents: []agent.Config{{ID: "dev"}}}}, nil, nil),
			nil,
		)
		require.NoError(t, err)

		r := setupRouterWithState(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/w1/agents", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "agents")
	})
}

func Test_getAgentByID_Handler(t *testing.T) {
	t.Run("Should return 200 when agent exists", func(t *testing.T) {
		proj := &project.Config{}
		require.NoError(t, proj.SetCWD("/tmp"))
		wf := &workflow.Config{Agents: []agent.Config{{ID: "code-assistant"}}}
		st, err := appstatepkg.NewState(appstatepkg.NewBaseDeps(proj, []*workflow.Config{wf}, nil, nil), nil)
		require.NoError(t, err)

		r := setupRouterWithState(t, st)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/w1/agents/code-assistant", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "agent retrieved")
	})

	t.Run("Should return 404 when agent does not exist", func(t *testing.T) {
		proj := &project.Config{}
		require.NoError(t, proj.SetCWD("/tmp"))
		wf := &workflow.Config{Agents: []agent.Config{{ID: "one"}}}
		st, err := appstatepkg.NewState(appstatepkg.NewBaseDeps(proj, []*workflow.Config{wf}, nil, nil), nil)
		require.NoError(t, err)
		r := setupRouterWithState(t, st)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/w1/agents/missing", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), router.ErrNotFoundCode)
	})
}

func Test_AgentHandlers_MissingAppState(t *testing.T) {
	t.Run("Should return 500 when app state is missing - listAgents", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		api := r.Group("/api/v0")
		Register(api)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/w1/agents", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Should return 500 when app state is missing - getAgentByID", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		api := r.Group("/api/v0")
		Register(api)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/w1/agents/a1", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
