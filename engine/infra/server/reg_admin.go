package server

import (
	"context"
	"errors"
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
	if state.Store == nil {
		return fmt.Errorf("auth store not initialized")
	}
	authRepo := state.Store.NewAuthRepo()
	factory := authuc.NewFactory(authRepo)
	admin := CreateAdminGroup(ctx, apiBase, factory)
	admin.POST("/reload", func(c *gin.Context) {
		adminReloadHandler(c, server)
	})
	registerMetaRoutes(admin)
	return nil
}

// adminReloadHandler handles the admin-triggered configuration reload.
//
//	@Summary      Reload configuration and reconcile schedules
//	@Description  Rebuild compiled workflows from repo|builder and trigger schedule reconciliation. Admin only.
//	@Description  Aliases: yaml -> repo, store -> builder.
//	@Tags         admin
//	@Accept       json
//	@Produce      json
//	@Param source query string false "Reload source. yaml->repo, store->builder. Default repo." Enums(repo,builder)
//	@Success      200  {object}  router.Response{data=map[string]any}  "Reload completed"
//	@Failure      400  {object}  router.Response{error=router.ErrorInfo} "Invalid parameters"
//	@Failure      401  {object}  router.Response{error=router.ErrorInfo} "Unauthorized"
//	@Failure      403  {object}  router.Response{error=router.ErrorInfo} "Forbidden"
//	@Failure      500  {object}  router.Response{error=router.ErrorInfo} "Internal server error"
//	@Router       /admin/reload [post]
func adminReloadHandler(c *gin.Context, server *Server) {
	start := time.Now()
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	st := router.GetAppState(c)
	if st == nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "application state not initialized", nil)
		return
	}
	desiredMode, ok := parseReloadSource(c)
	if !ok {
		return
	}
	opCtx, cleanup, err := buildOpContext(ctx, desiredMode)
	if err != nil {
		respondReloadServerError(c, newReloadError("failed to prepare operation configuration", err))
		return
	}
	defer func() {
		if err := cleanup(); err != nil {
			log.Warn("failed to release admin reload context", "error", err)
		}
	}()
	compiled, err := server.loadAndReplaceWorkflows(opCtx, st)
	if err != nil {
		respondReloadServerError(c, err)
		return
	}
	if err := reconcileIfPresent(ctx, st, compiled); err != nil {
		respondReloadServerError(c, newReloadError("schedule reconciliation failed", err))
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

type reloadQuery struct {
	Source string `form:"source" binding:"omitempty,oneof=repo builder yaml store"`
}

// parseReloadSource resolves the desired reload mode and handles validation errors.
func parseReloadSource(c *gin.Context) (string, bool) {
	var q reloadQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		router.RespondWithError(
			c,
			http.StatusBadRequest,
			router.NewRequestError(
				http.StatusBadRequest,
				"invalid source parameter (allowed values: repo, builder)",
				fmt.Errorf("validation failed: %w", err),
			),
		)
		return "", false
	}
	desiredMode := resolveSourceMode(strings.TrimSpace(strings.ToLower(q.Source)))
	if desiredMode == "" {
		router.RespondWithError(
			c,
			http.StatusBadRequest,
			router.NewRequestError(
				http.StatusBadRequest,
				"invalid source parameter (allowed values: repo, builder)",
				nil,
			),
		)
		return "", false
	}
	return desiredMode, true
}

// respondReloadServerError sends a standardized server error response for admin reload failures.
func respondReloadServerError(c *gin.Context, err error) {
	var detail *reloadError
	if errors.As(err, &detail) {
		router.RespondWithServerError(c, router.ErrInternalCode, detail.message, detail.cause)
		return
	}
	router.RespondWithServerError(c, router.ErrInternalCode, "reload failed", err)
}

// loadAndReplaceWorkflows reloads project workflows and updates the application state.
func (s *Server) loadAndReplaceWorkflows(
	opCtx context.Context,
	st *appstate.State,
) ([]*workflow.Config, error) {
	cwd, file, ok := extractProjectPaths(st)
	if !ok {
		return nil, newReloadError("project configuration not available", nil)
	}
	store, ok := getResourceStoreFromState(st)
	if !ok {
		return nil, newReloadError("resource store not available", nil)
	}
	svc := csvc.NewService(s.envFilePath, store)
	_, compiled, _, err := svc.LoadProject(opCtx, cwd, file)
	if err != nil {
		return nil, newReloadError("reload failed", err)
	}
	st.ReplaceWorkflows(compiled)
	return compiled, nil
}

// reloadError normalizes admin reload failures with a user-facing message and optional cause.
type reloadError struct {
	message string
	cause   error
}

// Error implements the error interface.
func (r *reloadError) Error() string {
	if r.cause == nil {
		return r.message
	}
	return fmt.Sprintf("%s: %v", r.message, r.cause)
}

// Unwrap returns the underlying cause.
func (r *reloadError) Unwrap() error {
	return r.cause
}

// newReloadError builds a reloadError with the provided message and cause.
func newReloadError(message string, cause error) error {
	return &reloadError{message: message, cause: cause}
}

func resolveSourceMode(param string) string {
	switch param {
	case "", "yaml", sourceRepo:
		return sourceRepo
	case "store", sourceBuilder:
		return sourceBuilder
	default:
		return ""
	}
}

func buildOpContext(ctx context.Context, mode string) (context.Context, func() error, error) {
	base := config.ManagerFromContext(ctx)
	baseCreated := false
	if base == nil {
		base = config.NewManager(ctx, config.NewService())
		baseCreated = true
		if _, err := base.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
			_ = base.Close(ctx)
			return nil, func() error { return nil }, err
		}
	}
	override := &staticSource{data: map[string]any{"server": map[string]any{"source_of_truth": mode}}}
	baseSources := base.Sources()
	if len(baseSources) == 0 {
		baseSources = []config.Source{config.NewDefaultProvider(), config.NewEnvProvider()}
	}
	sources := append(make([]config.Source, 0, len(baseSources)+1), baseSources...)
	sources = append(sources, override)
	cm := config.NewManager(ctx, base.Service)
	if _, err := cm.Load(ctx, sources...); err != nil {
		_ = cm.Close(ctx)
		if baseCreated {
			_ = base.Close(ctx)
		}
		return nil, func() error { return nil }, err
	}
	cleanup := func() error {
		var errs []error
		if err := cm.Close(ctx); err != nil {
			errs = append(errs, err)
		}
		if baseCreated {
			if err := base.Close(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
	return config.ContextWithManager(ctx, cm), cleanup, nil
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
