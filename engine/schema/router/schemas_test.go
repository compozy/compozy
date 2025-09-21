package schemarouter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSchemaRouter(t *testing.T, store resources.ResourceStore) *gin.Engine {
	t.Helper()
	proj := &project.Config{Name: "demo"}
	require.NoError(t, proj.SetCWD(t.TempDir()))
	state, err := appstate.NewState(appstate.NewBaseDeps(proj, nil, nil, nil), nil)
	require.NoError(t, err)
	state.SetResourceStore(store)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(appstate.StateMiddleware(state))
	r.Use(router.ErrorHandler())
	api := r.Group("/api/v0")
	Register(api)
	return r
}

func TestSchemaRouter_PutGetDelete(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	r := setupSchemaRouter(t, store)
	body := map[string]any{"id": "user", "type": "object"}
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	putReq := httptest.NewRequest(http.MethodPut, "/api/v0/schemas/user", bytes.NewReader(payload))
	putReq.Header.Set("Content-Type", "application/json")
	putRes := httptest.NewRecorder()
	r.ServeHTTP(putRes, putReq)
	assert.Equal(t, http.StatusCreated, putRes.Code)
	getReq := httptest.NewRequest(http.MethodGet, "/api/v0/schemas/user", http.NoBody)
	getRes := httptest.NewRecorder()
	r.ServeHTTP(getRes, getReq)
	assert.Equal(t, http.StatusOK, getRes.Code)
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v0/schemas/user", http.NoBody)
	deleteRes := httptest.NewRecorder()
	r.ServeHTTP(deleteRes, deleteReq)
	assert.Equal(t, http.StatusNoContent, deleteRes.Code)
}

func TestSchemaRouter_DeleteConflict(t *testing.T) {
	store := resources.NewMemoryResourceStore()
	r := setupSchemaRouter(t, store)
	body := map[string]any{"id": "user", "type": "object"}
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	putReq := httptest.NewRequest(http.MethodPut, "/api/v0/schemas/user", bytes.NewReader(payload))
	putReq.Header.Set("Content-Type", "application/json")
	putRes := httptest.NewRecorder()
	r.ServeHTTP(putRes, putReq)
	assert.Equal(t, http.StatusCreated, putRes.Code)
	ref := schema.Schema(map[string]any{"__schema_ref__": "user"})
	wf := &workflow.Config{ID: "wf1", Opts: workflow.Opts{InputSchema: &ref}}
	_, err = store.Put(
		context.Background(),
		resources.ResourceKey{Project: "demo", Type: resources.ResourceWorkflow, ID: "wf1"},
		wf,
	)
	require.NoError(t, err)
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v0/schemas/user", http.NoBody)
	deleteRes := httptest.NewRecorder()
	r.ServeHTTP(deleteRes, deleteReq)
	assert.Equal(t, http.StatusConflict, deleteRes.Code)
}
