package resourcesintegration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	infrarouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	resourcesrouter "github.com/compozy/compozy/engine/resources/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newIntegrationServer(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	prj := &project.Config{Name: "p", Version: "1.0"}
	require.NoError(t, prj.SetCWD("."))
	deps := appstate.NewBaseDeps(prj, nil, nil, nil)
	st, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	st.SetResourceStore(resources.NewMemoryResourceStore())
	r.Use(appstate.StateMiddleware(st))
	r.Use(infrarouter.ErrorHandler())
	api := r.Group("/api/v0")
	resourcesrouter.Register(api)
	return r
}

func TestResourcesHTTP_CRUD_ETag_Integration(t *testing.T) {
	srv := newIntegrationServer(t)
	t.Run("Should create, get and update with If-Match", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v0/resources/agent",
			strings.NewReader(`{"id":"a","type":"agent","instructions":"x"}`),
		)
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusCreated, res.Code)
		et := res.Header().Get("ETag")
		require.NotEmpty(t, et)
		g := httptest.NewRecorder()
		gr := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent/a", http.NoBody)
		srv.ServeHTTP(g, gr)
		require.Equal(t, http.StatusOK, g.Code)
		require.Equal(t, et, g.Header().Get("ETag"))
		u := httptest.NewRecorder()
		ur := httptest.NewRequest(
			http.MethodPut,
			"/api/v0/resources/agent/a",
			strings.NewReader(`{"id":"a","type":"agent","instructions":"y"}`),
		)
		ur.Header.Set("If-Match", et)
		srv.ServeHTTP(u, ur)
		require.Equal(t, http.StatusOK, u.Code)
		require.NotEqual(t, et, u.Header().Get("ETag"))
	})
	t.Run("Should return 404 on get missing id", func(t *testing.T) {
		r := httptest.NewRecorder()
		q := httptest.NewRequest(http.MethodGet, "/api/v0/resources/agent/missing", http.NoBody)
		srv.ServeHTTP(r, q)
		require.Equal(t, http.StatusNotFound, r.Code)
		var body struct {
			Status int
			Error  struct {
				Code    string
				Message string
			}
		}
		_ = json.Unmarshal(r.Body.Bytes(), &body)
		require.Equal(t, http.StatusNotFound, body.Status)
	})
}
