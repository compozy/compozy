package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	csvc "github.com/compozy/compozy/engine/infra/server/config"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/schedule"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// staticSource is a minimal in-memory config source used to override values for a single operation.
// It implements pkg/config.Source.
type staticSource struct {
	data map[string]any
}

func (s *staticSource) Load() (map[string]any, error) {
	return s.data, nil
}

func (s *staticSource) Watch(_ context.Context, _ func()) error {
	return nil
}

func (s *staticSource) Type() config.SourceType {
	return config.SourceCLI
}

func (s *staticSource) Close() error {
	return nil
}

func setupAdminRoutes(
	ctx context.Context,
	apiBase *gin.RouterGroup,
	state *appstate.State,
	server *Server,
) error {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing config in context")
	}
	if state == nil {
		return fmt.Errorf("application state not initialized")
	}
	authRepo := state.Store.NewAuthRepo()
	factory := authuc.NewFactory(authRepo)
	admin := CreateAdminGroup(apiBase, factory, cfg)
	admin.GET("/reload", func(c *gin.Context) {
		adminReloadHandler(c, server)
	})
	return nil
}

// adminReloadHandler handles the admin-triggered configuration reload.
//
//	@Summary      Reload configuration and reconcile schedules
//	@Description  Rebuild compiled workflows from yaml|store and
//	@Description  trigger schedule reconciliation. Admin only.
//	@Tags         admin
//	@Accept       json
//	@Produce      json
//	@Param        source  query  string  false  "yaml|store"  Enums(yaml,store)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Reload completed"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo} "Invalid parameters"
//	@Failure      401  {object}  router.Response{error=router.ErrorInfo} "Unauthorized"
//	@Failure      403  {object}  router.Response{error=router.ErrorInfo} "Forbidden"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo} "Internal server error"
//	@Router       /admin/reload [get]
func adminReloadHandler(c *gin.Context, server *Server) {
	start := time.Now()
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	st := router.GetAppState(c)
	if st == nil {
		return
	}
	desiredMode := resolveSourceMode(strings.TrimSpace(strings.ToLower(c.Query("source"))))
	if desiredMode == "" {
		router.RespondWithError(
			c,
			http.StatusBadRequest,
			router.NewRequestError(http.StatusBadRequest, "invalid source parameter", nil),
		)
		return
	}
	opCtx, err := buildOpContext(ctx, desiredMode)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to prepare operation configuration", err)
		return
	}
	cwd, file, ok := extractProjectPaths(st)
	if !ok {
		router.RespondWithServerError(c, router.ErrInternalCode, "project configuration not available", nil)
		return
	}
	store, ok := getResourceStoreFromState(st)
	if !ok {
		router.RespondWithServerError(c, router.ErrInternalCode, "resource store not available", nil)
		return
	}
	svc := csvc.NewService(server.envFilePath, store)
	_, compiled, _, err := svc.LoadProject(opCtx, cwd, file)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "reload failed", err)
		return
	}
	st.ReplaceWorkflows(compiled)
	if err := reconcileIfPresent(ctx, st, compiled); err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "schedule reconciliation failed", err)
		return
	}
	duration := time.Since(start)
	payload := map[string]any{
		"recompiled":  len(compiled),
		"duration_ms": duration.Milliseconds(),
		"source":      desiredMode,
	}
	log.Info("admin reload completed", "source", desiredMode, "count", len(compiled), "duration", duration)
	router.RespondOK(c, "reload completed", payload)
}

func resolveSourceMode(param string) string {
	switch param {
	case "", "yaml", "repo":
		return "repo"
	case "store", "builder":
		return "builder"
	default:
		return ""
	}
}

func buildOpContext(ctx context.Context, mode string) (context.Context, error) {
	cm := config.NewManager(config.NewService())
	override := &staticSource{data: map[string]any{"server": map[string]any{"source_of_truth": mode}}}
	if _, err := cm.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider(), override); err != nil {
		return nil, err
	}
	return config.ContextWithManager(ctx, cm), nil
}

func extractProjectPaths(st *appstate.State) (string, string, bool) {
	proj := st.ProjectConfig
	if proj == nil || proj.GetCWD() == nil {
		return "", "", false
	}
	return proj.GetCWD().PathStr(), proj.GetFilePath(), true
}

func getResourceStoreFromState(st *appstate.State) (resources.ResourceStore, bool) {
	v, ok := st.ResourceStore()
	if !ok || v == nil {
		return nil, false
	}
	rs, ok := v.(resources.ResourceStore)
	if !ok || rs == nil {
		return nil, false
	}
	return rs, true
}

func reconcileIfPresent(ctx context.Context, st *appstate.State, workflows []*workflow.Config) error {
	v, ok := st.ScheduleManager()
	if !ok || v == nil {
		return nil
	}
	mgr, ok := v.(schedule.Manager)
	if !ok || mgr == nil {
		return nil
	}
	return mgr.OnConfigurationReload(ctx, workflows)
}
