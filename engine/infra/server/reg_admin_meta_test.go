package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/auth/model"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminMetaEndpoints(t *testing.T) {
	ginmode.EnsureGinTestMode()
	r := gin.New()
	prj := &project.Config{Name: "p"}
	require.NoError(t, prj.SetCWD("."))
	st, err := appstate.NewState(appstate.NewBaseDeps(prj, nil, nil, nil), nil)
	require.NoError(t, err)
	store := resources.NewMemoryResourceStore()
	st.SetResourceStore(store)
	r.Use(appstate.StateMiddleware(st))
	// inject admin user before group is created so RequireAdmin sees it
	r.Use(func(c *gin.Context) {
		u := &model.User{ID: core.MustNewID(), Role: model.RoleAdmin}
		c.Request = c.Request.WithContext(userctx.WithUser(c.Request.Context(), u))
		c.Next()
	})
	api := r.Group(routes.Base())
	factory := authuc.NewFactory(dummyRepo{})
	// build context with config manager
	cfgMgr := config.NewManager(t.Context(), config.NewService())
	t.Setenv("SERVER_AUTH_ADMIN_KEY", "0123456789abcdef")
	_, err = cfgMgr.Load(
		t.Context(),
		config.NewDefaultProvider(),
		config.NewCLIProvider(map[string]any{"auth-enabled": true}),
	)
	require.NoError(t, err)
	ctx := config.ContextWithManager(t.Context(), cfgMgr)
	admin := CreateAdminGroup(ctx, api, factory)
	registerMetaRoutes(admin)
	// seed a meta entry
	_, _ = store.Put(
		t.Context(),
		resources.ResourceKey{Project: "p", Type: resources.ResourceMeta, ID: "p:agent:a"},
		map[string]any{"source": "api", "updated_at": "2024-01-01T00:00:00Z", "updated_by": "u"},
	)
	// get specific
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routes.Base()+"/admin/meta/agent/a", http.NoBody)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	// add more meta items
	for i := range 5 {
		id := fmt.Sprintf("p:agent:a%d", i)
		_, _ = store.Put(
			t.Context(),
			resources.ResourceKey{Project: "p", Type: resources.ResourceMeta, ID: id},
			map[string]any{
				"project": "p", "type": "agent", "id": fmt.Sprintf("a%d", i), "source": "api", "updated_at": "2024-01-01T00:00:00Z", "updated_by": "u",
			},
		)
	}
	// list changes with limit
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, routes.Base()+"/admin/meta/changes?limit=3", http.NoBody)
	r.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	// list changes with offset
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, routes.Base()+"/admin/meta/changes?limit=2&offset=2", http.NoBody)
	r.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)
}

// configForTests returns enabled-auth config
// (no-op) removed: configForTests; tests now build ctx with config manager inline
