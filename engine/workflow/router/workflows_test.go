package wfrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appstate "github.com/compozy/compozy/engine/infra/server/appstate"
	routerpkg "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWorkflowTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	proj := &project.Config{Name: "demo"}
	require.NoError(t, proj.SetCWD(t.TempDir()))
	state, err := appstate.NewState(appstate.NewBaseDeps(proj, nil, nil, nil), nil)
	require.NoError(t, err)
	state.SetResourceStore(resources.NewMemoryResourceStore())
	cfgManager := config.NewManager(config.NewService())
	_, err = cfgManager.Load(context.Background(), config.NewDefaultProvider())
	require.NoError(t, err)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := logger.ContextWithLogger(c.Request.Context(), logger.NewForTests())
		ctx = config.ContextWithManager(ctx, cfgManager)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(appstate.StateMiddleware(state))
	r.Use(routerpkg.ErrorHandler())
	api := r.Group("/api/v0")
	Register(api)
	return r
}

func workflowBody(overrides map[string]any) []byte {
	base := map[string]any{
		"id":     "workflow-demo",
		"config": map[string]any{},
		"tasks":  []map[string]any{},
	}
	for k, v := range overrides {
		base[k] = v
	}
	payload, _ := json.Marshal(base)
	return payload
}

func TestUpsertWorkflow_InvalidJSON(t *testing.T) {
	r := setupWorkflowTestRouter(t)
	req := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/workflow-demo", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	assert.Equal(t, http.StatusBadRequest, res.Code)
	assert.Contains(t, res.Body.String(), "invalid request body")
}

func TestUpsertWorkflow_WeakETagRejected(t *testing.T) {
	r := setupWorkflowTestRouter(t)
	body := workflowBody(nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/workflow-demo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", "W/\"etag\"")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	assert.Equal(t, http.StatusBadRequest, res.Code)
	assert.Contains(t, res.Body.String(), "invalid If-Match header")
}

func TestUpsertWorkflow_CreateUpdateAndETagMismatch(t *testing.T) {
	r := setupWorkflowTestRouter(t)
	createBody := workflowBody(map[string]any{"description": "first"})
	createReq := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/workflow-demo", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	r.ServeHTTP(createRes, createReq)
	require.Equal(t, http.StatusCreated, createRes.Code)
	createdETag := createRes.Header().Get("ETag")
	require.NotEmpty(t, createdETag)
	assert.Equal(t, "/api/v0/workflows/workflow-demo", createRes.Header().Get("Location"))

	updateBody := workflowBody(map[string]any{"description": "updated"})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/workflow-demo", bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("If-Match", createdETag)
	updateRes := httptest.NewRecorder()
	r.ServeHTTP(updateRes, updateReq)
	require.Equal(t, http.StatusOK, updateRes.Code)
	assert.Empty(t, updateRes.Header().Get("Location"))
	refreshedETag := updateRes.Header().Get("ETag")
	require.NotEmpty(t, refreshedETag)

	staleReq := httptest.NewRequest(http.MethodPut, "/api/v0/workflows/workflow-demo", bytes.NewReader(updateBody))
	staleReq.Header.Set("Content-Type", "application/json")
	staleReq.Header.Set("If-Match", "bogus")
	staleRes := httptest.NewRecorder()
	r.ServeHTTP(staleRes, staleReq)
	assert.Equal(t, http.StatusPreconditionFailed, staleRes.Code)
	assert.Contains(t, staleRes.Body.String(), "etag mismatch")
}

func TestGetWorkflowByID_NotFoundProblem(t *testing.T) {
	r := setupWorkflowTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/missing", http.NoBody)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	assert.Equal(t, http.StatusNotFound, res.Code)
	assert.Contains(t, res.Body.String(), "not found")
}
