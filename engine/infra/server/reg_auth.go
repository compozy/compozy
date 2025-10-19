package server

import (
	"context"
	"fmt"

	authrouter "github.com/compozy/compozy/engine/auth/router"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

func setupAuthSystem(
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
	baseAuth := state.Store.NewAuthRepo()
	if baseAuth == nil {
		return fmt.Errorf("auth repository not initialized")
	}
	authRepoDriver := driverPostgres
	authRepo := baseAuth
	if server != nil {
		repo, cacheDriver := server.buildAuthRepo(cfg, baseAuth)
		authRepo = repo
		server.authRepoDriverLabel = authRepoDriver
		server.authCacheDriverLabel = cacheDriver
		logger.FromContext(ctx).Info(
			"auth repository configured",
			"auth_repo_driver", authRepoDriver,
			"auth_cache_driver", cacheDriver,
		)
	}
	authFactory := authuc.NewFactory(authRepo)
	authManager := authmw.NewManager(authFactory, cfg)
	if cfg.Server.Auth.Enabled {
		apiBase.Use(authManager.Middleware())
		apiBase.Use(authManager.RequireAuth())
	}
	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		authrouter.RegisterRoutesWithMetrics(ctx, apiBase, authFactory, cfg, server.monitoring.Meter())
	} else {
		authrouter.RegisterRoutesWithMetrics(ctx, apiBase, authFactory, cfg, nil)
	}
	return nil
}
