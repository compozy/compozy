package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyRepo struct{}

func (dummyRepo) CreateUser(context.Context, *model.User) error { return nil }
func (dummyRepo) GetUserByID(context.Context, core.ID) (*model.User, error) {
	return &model.User{ID: core.MustNewID(), Role: model.RoleAdmin}, nil
}
func (dummyRepo) GetUserByEmail(context.Context, string) (*model.User, error) { return nil, nil }
func (dummyRepo) ListUsers(context.Context) ([]*model.User, error)            { return nil, nil }
func (dummyRepo) UpdateUser(context.Context, *model.User) error               { return nil }
func (dummyRepo) DeleteUser(context.Context, core.ID) error                   { return nil }
func (dummyRepo) CreateInitialAdminIfNone(context.Context, *model.User) error { return nil }
func (dummyRepo) CreateAPIKey(context.Context, *model.APIKey) error           { return nil }
func (dummyRepo) GetAPIKeyByID(context.Context, core.ID) (*model.APIKey, error) {
	return nil, authuc.ErrAPIKeyNotFound
}
func (dummyRepo) GetAPIKeyByFingerprint(context.Context, []byte) (*model.APIKey, error) {
	return nil, authuc.ErrAPIKeyNotFound
}
func (dummyRepo) ListAPIKeysByUserID(context.Context, core.ID) ([]*model.APIKey, error) {
	return nil, nil
}
func (dummyRepo) UpdateAPIKeyLastUsed(context.Context, core.ID) error { return nil }
func (dummyRepo) DeleteAPIKey(context.Context, core.ID) error         { return nil }

type mockScheduleManager struct{ called bool }

func (m *mockScheduleManager) ReconcileSchedules(_ context.Context, _ []*workflow.Config) error {
	return nil
}
func (m *mockScheduleManager) ListSchedules(_ context.Context) ([]*schedule.Info, error) {
	return []*schedule.Info{}, nil
}
func (m *mockScheduleManager) GetSchedule(_ context.Context, _ string) (*schedule.Info, error) {
	return nil, nil
}

func (m *mockScheduleManager) UpdateSchedule(
	_ context.Context,
	_ string,
	_ schedule.UpdateRequest,
) error {
	return nil
}
func (m *mockScheduleManager) DeleteSchedule(_ context.Context, _ string) error {
	return nil
}
func (m *mockScheduleManager) OnConfigurationReload(_ context.Context, _ []*workflow.Config) error {
	m.called = true
	return nil
}

func (m *mockScheduleManager) StartPeriodicReconciliation(
	_ context.Context,
	_ func() []*workflow.Config,
	_ time.Duration,
) error {
	return nil
}
func (m *mockScheduleManager) StopPeriodicReconciliation() {}

func setupAdminTestRouter(t *testing.T, withAdminUser bool, state *appstate.State) *gin.Engine {
	t.Helper()
	ginmode.EnsureGinTestMode()
	r := gin.New()
	t.Setenv("SERVER_AUTH_ADMIN_KEY", "test_admin_key_123")
	cfgMgr := config.NewManager(t.Context(), config.NewService())
	_, err := cfgMgr.Load(
		t.Context(),
		config.NewDefaultProvider(),
		config.NewCLIProvider(map[string]any{"auth-enabled": true}),
	)
	require.NoError(t, err)
	r.Use(appstate.StateMiddleware(state))
	if withAdminUser {
		r.Use(func(c *gin.Context) {
			u := &model.User{ID: core.MustNewID(), Role: model.RoleAdmin}
			c.Request = c.Request.WithContext(userctx.WithUser(c.Request.Context(), u))
			c.Next()
		})
	}
	apiBase := r.Group(routes.Base())
	factory := authuc.NewFactory(dummyRepo{})
	ctx := config.ContextWithManager(t.Context(), cfgMgr)
	admin := CreateAdminGroup(ctx, apiBase, factory)
	admin.POST("/reload", func(c *gin.Context) { adminReloadHandler(c, &Server{envFilePath: ".env"}) })
	return r
}

func TestAdminReload_Unauthorized(t *testing.T) {
	t.Run("Should return 403 when no admin user", func(t *testing.T) {
		state := &appstate.State{BaseDeps: appstate.BaseDeps{ProjectConfig: &project.Config{Name: "t"}}}
		r := setupAdminTestRouter(t, false, state)
		req := httptest.NewRequest(http.MethodPost, routes.Base()+"/admin/reload", http.NoBody)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		assert.Equal(t, http.StatusForbidden, res.Code)
	})
}

func TestAdminReload_BadParam(t *testing.T) {
	t.Run("Should return 400 for invalid source", func(t *testing.T) {
		state := &appstate.State{BaseDeps: appstate.BaseDeps{ProjectConfig: &project.Config{Name: "t"}}}
		r := setupAdminTestRouter(t, true, state)
		req := httptest.NewRequest(http.MethodPost, routes.Base()+"/admin/reload?source=invalid", http.NoBody)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		assert.Equal(t, http.StatusBadRequest, res.Code)
	})
}

func TestAdminReload_YAMLAndStore(t *testing.T) {
	t.Run("Should reload from YAML and store and trigger schedule hook", func(t *testing.T) {
		tmp := t.TempDir()
		wfContent := "id: test-wf\nversion: '1.0.0'\ndescription: minimal wf\ntasks: []\nagents: []\ntools: []\n"
		wfPath := filepath.Join(tmp, "workflow.yaml")
		require.NoError(t, os.WriteFile(wfPath, []byte(wfContent), 0o644))
		projYAML := "name: test\nworkflows:\n  - source: " + wfPath + "\n"
		err := os.WriteFile(filepath.Join(tmp, "compozy.yaml"), []byte(projYAML), 0o644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmp, "tools.ts"), []byte(""), 0o644)
		require.NoError(t, err)
		proj := &project.Config{Name: "test"}
		require.NoError(t, proj.SetCWD(tmp))
		proj.SetFilePath("compozy.yaml")
		state := &appstate.State{
			BaseDeps:   appstate.BaseDeps{ProjectConfig: proj, Workflows: []*workflow.Config{}},
			Extensions: map[appstate.ExtensionKey]any{},
		}
		state.SetResourceStore(resources.NewMemoryResourceStore())
		mockMgr := &mockScheduleManager{}
		state.SetScheduleManager(mockMgr)
		r := setupAdminTestRouter(t, true, state)
		// YAML mode
		req := httptest.NewRequest(http.MethodPost, routes.Base()+"/admin/reload?source=yaml", http.NoBody)
		req = req.WithContext(logger.ContextWithLogger(req.Context(), logger.NewForTests()))
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("unexpected status %d body=%s", res.Code, res.Body.String())
		}
		var body struct {
			Data    map[string]any `json:"data"`
			Message string         `json:"message"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		require.GreaterOrEqual(t, int(body.Data["recompiled"].(float64)), 1)
		assert.NotEmpty(t, body.Message)
		// Store mode (empty store is ok)
		req2 := httptest.NewRequest(http.MethodPost, routes.Base()+"/admin/reload?source=store", http.NoBody)
		req2 = req2.WithContext(logger.ContextWithLogger(req2.Context(), logger.NewForTests()))
		res2 := httptest.NewRecorder()
		r.ServeHTTP(res2, req2)
		require.Equal(t, http.StatusOK, res2.Code)
		var body2 struct {
			Data    map[string]any `json:"data"`
			Message string         `json:"message"`
		}
		require.NoError(t, json.Unmarshal(res2.Body.Bytes(), &body2))
		require.GreaterOrEqual(t, int(body2.Data["recompiled"].(float64)), 0)
		assert.NotEmpty(t, body2.Message)
		require.True(t, mockMgr.called)
	})
}
