package toolrouter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	infraRouter "github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type errorResponse struct {
	Status int `json:"status"`
	Error  struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newTestServer(t *testing.T, workflows []*workflow.Config) *gin.Engine {
	r := gin.New()
	prj := &project.Config{Name: "p", Version: "1.0"}
	require.NoError(t, prj.SetCWD("."))
	deps := appstate.NewBaseDeps(prj, workflows, nil, nil)
	st, err := appstate.NewState(deps, nil)
	require.NoError(t, err)
	r.Use(appstate.StateMiddleware(st))
	r.Use(infraRouter.ErrorHandler())
	api := r.Group("/api/v0")
	Register(api)
	return r
}

func TestRouter_ListAndGetTools(t *testing.T) {
	t.Parallel()
	t.Run("Should list tools for a workflow", func(t *testing.T) {
		t.Parallel()
		wf := &workflow.Config{ID: "wf1", Tools: []tool.Config{{ID: "fmt"}, {ID: "lint"}}}
		require.NoError(t, wf.SetCWD("."))
		srv := newTestServer(t, []*workflow.Config{wf})
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/wf1/tools", http.NoBody)
		res := httptest.NewRecorder()
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code)
		require.Equal(t, "application/json; charset=utf-8", res.Header().Get("Content-Type"))
		var body struct {
			Status int `json:"status"`
			Data   struct {
				Tools []tool.Config `json:"tools"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		require.Equal(t, 200, body.Status)
		assert.Len(t, body.Data.Tools, 2)
		ids := map[string]bool{}
		for i := range body.Data.Tools {
			ids[body.Data.Tools[i].ID] = true
		}
		assert.True(t, ids["fmt"])
		assert.True(t, ids["lint"])
	})
	t.Run("Should get single tool by ID", func(t *testing.T) {
		t.Parallel()
		wf := &workflow.Config{ID: "wf1", Tools: []tool.Config{{ID: "fmt", Description: "Formatter"}}}
		require.NoError(t, wf.SetCWD("."))
		srv := newTestServer(t, []*workflow.Config{wf})
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/wf1/tools/fmt", http.NoBody)
		res := httptest.NewRecorder()
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code)
		require.Equal(t, "application/json; charset=utf-8", res.Header().Get("Content-Type"))
		var body struct {
			Status int         `json:"status"`
			Data   tool.Config `json:"data"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		require.Equal(t, 200, body.Status)
		assert.Equal(t, "fmt", body.Data.ID)
		assert.Equal(t, "Formatter", body.Data.Description)
	})
	t.Run("Should return 404 when tool not found", func(t *testing.T) {
		t.Parallel()
		wf := &workflow.Config{ID: "wf1", Tools: []tool.Config{{ID: "a"}}}
		require.NoError(t, wf.SetCWD("."))
		srv := newTestServer(t, []*workflow.Config{wf})
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/wf1/tools/missing", http.NoBody)
		res := httptest.NewRecorder()
		srv.ServeHTTP(res, req)
		require.Equal(t, http.StatusNotFound, res.Code)
		var body errorResponse
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		assert.Equal(t, "NOT_FOUND", body.Error.Code)
		assert.Contains(t, body.Error.Message, "tool not found")
	})
	t.Run("Should return 500 when app state is missing", func(t *testing.T) {
		t.Parallel()
		r := gin.New()
		r.Use(infraRouter.ErrorHandler())
		api := r.Group("/api/v0")
		Register(api)
		req := httptest.NewRequest(http.MethodGet, "/api/v0/workflows/wf1/tools", http.NoBody)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.Equal(t, http.StatusInternalServerError, res.Code)
		var body errorResponse
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		assert.Equal(t, "INTERNAL_ERROR", body.Error.Code)
	})
}
