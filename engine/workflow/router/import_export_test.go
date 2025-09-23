package wfrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	testhelpers "github.com/compozy/compozy/test/helpers"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRouter_Export(t *testing.T) {
	t.Parallel()
	t.Run("Should export workflows to project root", func(t *testing.T) {
		t.Parallel()
		testhelpers.EnsureGinTestMode()
		ctx := context.Background()
		projectID := "demo"
		store := resources.NewMemoryResourceStore()
		_, err := store.Put(
			ctx,
			resources.ResourceKey{Project: projectID, Type: resources.ResourceWorkflow, ID: "build"},
			map[string]any{"id": "build"},
		)
		require.NoError(t, err)
		root := t.TempDir()
		proj := &project.Config{Name: projectID}
		require.NoError(t, proj.SetCWD(root))
		state, err := appstate.NewState(appstate.NewBaseDeps(proj, nil, nil, nil), nil)
		require.NoError(t, err)
		state.SetResourceStore(store)
		r := gin.New()
		r.Use(appstate.StateMiddleware(state))
		r.Use(router.ErrorHandler())
		api := r.Group("/api/v0")
		Register(api)
		req := httptest.NewRequest(http.MethodPost, "/api/v0/workflows/export", http.NoBody)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code)
		require.Contains(t, res.Body.String(), "\"written\":1")
		_, err = os.Stat(filepath.Join(root, "workflows", "build.yaml"))
		require.NoError(t, err)
	})
}
