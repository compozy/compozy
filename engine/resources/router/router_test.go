package resourcesrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	infrarouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { gin.SetMode(gin.TestMode); os.Exit(m.Run()) }

type apiError struct {
	Status int `json:"status"`
	Error  struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details string `json:"details"`
	} `json:"error"`
}

func newServerWithStore(t *testing.T) *gin.Engine {
	t.Helper()
	r := gin.New()
	prj := &project.Config{Name: "p", Version: "1.0"}
	tmp := t.TempDir()
	require.NoError(t, prj.SetCWD(tmp))
	deps := appstate.NewBaseDeps(prj, nil, nil, nil)
	st, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	st.SetResourceStore(resources.NewMemoryResourceStore())
	r.Use(appstate.StateMiddleware(st))
	r.Use(infrarouter.ErrorHandler())
	api := r.Group("/api/v0")
	Register(api)
	return r
}

func TestResourcesRouter_CRUDAndETag(t *testing.T) {
	srv := newServerWithStore(t)
	t.Run("Should create and get with same ETag", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		body := `{"id":"a1","type":"agent","name":"A"}`
		res1 := httptest.NewRecorder()
		req1 := httptest.NewRequest(http.MethodPost, "/api/v0/resources/agent", strings.NewReader(body))
		req1.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(res1, req1)
		require.Equal(t, http.StatusCreated, res1.Code)
		etag := res1.Header().Get("ETag")
		require.NotEmpty(t, etag)
		loc := res1.Header().Get("Location")
		assert.Equal(t, "/api/v0/resources/agent/a1", loc)
		res2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent/a1", http.NoBody)
		req2.Header.Set("Accept", "application/json")
		srv.ServeHTTP(res2, req2)
		require.Equal(t, http.StatusOK, res2.Code)
		assert.Equal(t, etag, res2.Header().Get("ETag"))
	})
	// Not found path
	t.Run("Should return 404 when resource not found", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent/missing", http.NoBody)
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusNotFound, res.Code)
		var e apiError
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &e))
		assert.Equal(t, http.StatusNotFound, e.Status)
		assert.NotEmpty(t, e.Error.Code)
		assert.NotEmpty(t, e.Error.Message)
	})
	t.Run("Should list and filter by prefix", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		_ = sendJSON(srv, http.MethodPost, "/api/v0/resources/agent", `{"id":"pre1","type":"agent"}`)
		_ = sendJSON(srv, http.MethodPost, "/api/v0/resources/agent", `{"id":"pre2","type":"agent"}`)
		_ = sendJSON(srv, http.MethodPost, "/api/v0/resources/agent", `{"id":"x","type":"agent"}`)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent?q=pre", http.NoBody)
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code)
		var body struct {
			Status int `json:"status"`
			Data   struct {
				Keys []string `json:"keys"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		assert.ElementsMatch(t, []string{"pre1", "pre2"}, body.Data.Keys)
	})
	t.Run("Should reject project field in body", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		res := sendJSON(srv, http.MethodPost, "/api/v0/resources/agent", `{"id":"bad1","type":"agent","project":"x"}`)
		require.Equal(t, http.StatusBadRequest, res.Code)
	})
	t.Run("Should reject invalid id with whitespace", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		res := sendJSON(srv, http.MethodPost, "/api/v0/resources/agent", `{"id":"a 1","type":"agent"}`)
		require.Equal(t, http.StatusBadRequest, res.Code)
	})
	t.Run("Should handle PUT with If-Match and conflicts", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		res := sendJSON(srv, http.MethodPost, "/api/v0/resources/tool", `{"id":"t1","type":"tool","v":1}`)
		et := res.Header().Get("ETag")
		resBad := httptest.NewRecorder()
		reqBad := httptest.NewRequest(
			http.MethodPut,
			"/api/v0/resources/tool/t1",
			strings.NewReader(`{"id":"t1","type":"tool","v":2}`),
		)
		reqBad.Header.Set("If-Match", "stale")
		srv.ServeHTTP(resBad, reqBad)
		require.Equal(t, http.StatusConflict, resBad.Code)
		var e apiError
		require.NoError(t, json.Unmarshal(resBad.Body.Bytes(), &e))
		assert.Equal(t, "CONFLICT", e.Error.Code)
		resOK := httptest.NewRecorder()
		reqOK := httptest.NewRequest(
			http.MethodPut,
			"/api/v0/resources/tool/t1",
			strings.NewReader(`{"id":"t1","type":"tool","v":3}`),
		)
		reqOK.Header.Set("If-Match", et)
		srv.ServeHTTP(resOK, reqOK)
		require.Equal(t, http.StatusOK, resOK.Code)
		assert.NotEqual(t, et, resOK.Header().Get("ETag"))
	})
	t.Run("Should delete idempotently", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		_ = sendJSON(srv, http.MethodPost, "/api/v0/resources/mcp", `{"id":"m1","type":"mcp"}`)
		res1 := httptest.NewRecorder()
		req1 := httptest.NewRequest(http.MethodDelete, "/api/v0/resources/mcp/m1", http.NoBody)
		srv.ServeHTTP(res1, req1)
		require.Equal(t, http.StatusOK, res1.Code)
		res2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodDelete, "/api/v0/resources/mcp/m1", http.NoBody)
		srv.ServeHTTP(res2, req2)
		require.Equal(t, http.StatusOK, res2.Code)
	})
	t.Run("Should return 400 for unknown type and bad JSON", func(t *testing.T) {
		// Intentionally not parallel: shared srv
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v0/resources/unknown", strings.NewReader(`{"id":"a"}`))
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code)
		r2 := httptest.NewRecorder()
		q := httptest.NewRequest(http.MethodPost, "/api/v0/resources/agent", strings.NewReader("{invalid json}"))
		q.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(r2, q)
		require.Equal(t, http.StatusBadRequest, r2.Code)
	})
}

func TestResourcesRouter_MissingState(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.Use(infrarouter.ErrorHandler())
	api := r.Group("/api/v0")
	Register(api)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent/a1", http.NoBody)
	r.ServeHTTP(res, req)
	require.Equal(t, http.StatusInternalServerError, res.Code)
	var e apiError
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &e))
	assert.Equal(t, http.StatusInternalServerError, e.Status)
	assert.NotEmpty(t, e.Error.Code)
	assert.NotEmpty(t, e.Error.Message)
}

type errListStore struct{ resources.MemoryResourceStore }

func (e *errListStore) List(_ context.Context, _ string, _ resources.ResourceType) ([]resources.ResourceKey, error) {
	return nil, assert.AnError
}

func TestResourcesRouter_Errors(t *testing.T) {
	t.Parallel()
	t.Run("Should return 500 on list error", func(t *testing.T) {
		r := gin.New()
		prj := &project.Config{Name: "p", Version: "1.0"}
		tmp := t.TempDir()
		require.NoError(t, prj.SetCWD(tmp))
		deps := appstate.NewBaseDeps(prj, nil, nil, nil)
		st, err := appstate.NewState(deps, nil)
		require.NoError(t, err)
		st.SetResourceStore(&errListStore{})
		r.Use(appstate.StateMiddleware(st))
		r.Use(infrarouter.ErrorHandler())
		api := r.Group("/api/v0")
		Register(api)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent", http.NoBody)
		req.Header.Set("Accept", "application/json")
		r.ServeHTTP(res, req)
		require.Equal(t, http.StatusInternalServerError, res.Code)
		var e apiError
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &e))
		assert.Equal(t, http.StatusInternalServerError, e.Status)
		assert.NotEmpty(t, e.Error.Code)
		assert.NotEmpty(t, e.Error.Message)
	})
	t.Run("Should return 400 on create without id", func(t *testing.T) {
		srv := newServerWithStore(t)
		res := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/resources/agent",
			strings.NewReader(`{"type":"agent","instructions":"x"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusBadRequest, res.Code)
	})
	t.Run("Should return 400 when id param is blank", func(t *testing.T) {
		srv := newServerWithStore(t)
		r1 := httptest.NewRecorder()
		q1 := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent/%20", http.NoBody)
		srv.ServeHTTP(r1, q1)
		require.Equal(t, http.StatusBadRequest, r1.Code)
		r2 := httptest.NewRecorder()
		q2 := httptest.NewRequest(http.MethodDelete, "/api/v0/resources/agent/%20", http.NoBody)
		srv.ServeHTTP(r2, q2)
		require.Equal(t, http.StatusBadRequest, r2.Code)
		r3 := httptest.NewRecorder()
		q3 := httptest.NewRequest(
			http.MethodPut,
			"/api/v0/resources/agent/%20",
			strings.NewReader(`{"id":"a","type":"agent"}`),
		)
		q3.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(r3, q3)
		require.Equal(t, http.StatusBadRequest, r3.Code)
	})
}

func sendJSON(srv *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	srv.ServeHTTP(rec, req)
	return rec
}
