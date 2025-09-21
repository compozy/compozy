package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appstate "github.com/compozy/compozy/engine/infra/server/appstate"
	ratelimit "github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	routerpkg "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWorkflowIntegrationServer(t *testing.T) (*gin.Engine, resources.ResourceStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	proj := &project.Config{Name: "integration"}
	require.NoError(t, proj.SetCWD(t.TempDir()))
	state, err := appstate.NewState(appstate.NewBaseDeps(proj, nil, nil, nil), nil)
	require.NoError(t, err)
	store := resources.NewMemoryResourceStore()
	state.SetResourceStore(store)
	cfgManager := config.NewManager(config.NewService())
	_, err = cfgManager.Load(context.Background(), config.NewDefaultProvider())
	require.NoError(t, err)
	rlManager, err := ratelimit.NewManager(ratelimit.DefaultConfig(), nil)
	require.NoError(t, err)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := logger.ContextWithLogger(c.Request.Context(), logger.NewForTests())
		ctx = config.ContextWithManager(ctx, cfgManager)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(rlManager.Middleware())
	r.Use(appstate.StateMiddleware(state))
	r.Use(routerpkg.ErrorHandler())
	api := r.Group("/api/v0")
	wfrouter.Register(api)
	return r, store
}

func workflowPayload(id string, description string) []byte {
	body := map[string]any{
		"id":          id,
		"description": description,
		"config":      map[string]any{},
		"tasks":       []map[string]any{},
	}
	payload, _ := json.Marshal(body)
	return payload
}

func TestWorkflowEndpointsIntegration(t *testing.T) {
	t.Run("Should create workflow and expose headers", func(t *testing.T) {
		srv, _ := setupWorkflowIntegrationServer(t)
		payload := workflowPayload("wf-int", "created via test")
		putReq := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/wf-int", bytes.NewReader(payload))
		putReq.Header.Set("Content-Type", "application/json")
		putRes := httptest.NewRecorder()
		srv.ServeHTTP(putRes, putReq)
		require.Equal(t, http.StatusCreated, putRes.Code)
		etag := putRes.Header().Get("ETag")
		require.NotEmpty(t, etag)
		assert.Equal(t, "/api/v0/workflows/wf-int", putRes.Header().Get("Location"))
		assert.NotEmpty(t, putRes.Header().Get("RateLimit-Limit"))
		assert.NotEmpty(t, putRes.Header().Get("RateLimit-Remaining"))
		assert.NotEmpty(t, putRes.Header().Get("RateLimit-Reset"))

		getReq := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/wf-int", http.NoBody)
		getRes := httptest.NewRecorder()
		srv.ServeHTTP(getRes, getReq)
		require.Equal(t, http.StatusOK, getRes.Code)
		assert.Equal(t, etag, getRes.Header().Get("ETag"))
		var body map[string]any
		require.NoError(t, json.Unmarshal(getRes.Body.Bytes(), &body))
		data, ok := body["data"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, etag, data["_etag"])
	})

	t.Run("Should paginate workflows and surface Link header", func(t *testing.T) {
		srv, _ := setupWorkflowIntegrationServer(t)
		for i := 0; i < 2; i++ {
			id := fmt.Sprintf("wf-page-%d", i)
			payload := workflowPayload(id, "page test")
			req := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/"+id, bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			srv.ServeHTTP(res, req)
			require.Equal(t, http.StatusCreated, res.Code)
		}
		listReq := httptest.NewRequest(http.MethodGet, "/api/v0/workflows?limit=1", http.NoBody)
		listRes := httptest.NewRecorder()
		srv.ServeHTTP(listRes, listReq)
		require.Equal(t, http.StatusOK, listRes.Code)
		link := listRes.Header().Get("Link")
		assert.NotEmpty(t, link)
		assert.True(t, strings.Contains(link, "rel=\"next\""))
		assert.NotEmpty(t, listRes.Header().Get("RateLimit-Limit"))
		var listBody map[string]any
		require.NoError(t, json.Unmarshal(listRes.Body.Bytes(), &listBody))
		data, ok := listBody["data"].(map[string]any)
		require.True(t, ok)
		page, ok := data["page"].(map[string]any)
		require.True(t, ok)
		_, hasCursor := page["next_cursor"]
		assert.True(t, hasCursor)
	})

	t.Run("Should reject stale If-Match with problem details", func(t *testing.T) {
		srv, _ := setupWorkflowIntegrationServer(t)
		payload := workflowPayload("wf-conflict", "conflict")
		createReq := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/wf-conflict", bytes.NewReader(payload))
		createReq.Header.Set("Content-Type", "application/json")
		createRes := httptest.NewRecorder()
		srv.ServeHTTP(createRes, createReq)
		require.Equal(t, http.StatusCreated, createRes.Code)
		staleReq := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/wf-conflict", bytes.NewReader(payload))
		staleReq.Header.Set("Content-Type", "application/json")
		staleReq.Header.Set("If-Match", "invalid")
		staleRes := httptest.NewRecorder()
		srv.ServeHTTP(staleRes, staleReq)
		assert.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
		assert.Equal(t, "application/problem+json", staleRes.Header().Get("Content-Type"))
		assert.Contains(t, staleRes.Body.String(), "etag mismatch")
	})
}
