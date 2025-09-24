package toolrouter

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
	"github.com/compozy/compozy/engine/tool"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestToolRouter_Import(t *testing.T) {
	t.Parallel()
	t.Run("Should import tools and echo strategy", func(t *testing.T) {
		t.Parallel()
		ginmode.EnsureGinTestMode()
		ctx := context.Background()
		projectID := "demo"
		store := resources.NewMemoryResourceStore()
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "tools"), 0o755))
		toolCfg := tool.Config{ID: "printer", Description: "prints"}
		toolBytes, err := yaml.Marshal(toolCfg)
		require.NoError(t, err)
		require.NoError(
			t,
			os.WriteFile(
				filepath.Join(root, "tools", "printer.yaml"),
				toolBytes,
				0o600,
			),
		)
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
		req := httptest.NewRequest(http.MethodPost, "/api/v0/tools/import?strategy=overwrite_conflicts", http.NoBody)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code)
		require.Contains(t, res.Body.String(), "\"strategy\":\"overwrite_conflicts\"")
		_, _, err = store.Get(
			ctx,
			resources.ResourceKey{Project: projectID, Type: resources.ResourceTool, ID: "printer"},
		)
		require.NoError(t, err)
	})
}
