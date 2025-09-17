package server

import (
	"context"

	authrouter "github.com/compozy/compozy/engine/auth/router"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

func setupAuthSystem(
	ctx context.Context,
	apiBase *gin.RouterGroup,
	state *appstate.State,
	cfg *config.Config,
	server *Server,
) error {
	authRepo := state.Store.NewAuthRepo()
	authFactory := authuc.NewFactory(authRepo)
	authManager := authmw.NewManager(authFactory, cfg)
	if cfg.Server.Auth.Enabled {
		apiBase.Use(authManager.Middleware())
		apiBase.Use(authManager.RequireAuth())
	}
	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		authrouter.RegisterRoutesWithMetrics(ctx, apiBase, authFactory, cfg, server.monitoring.Meter())
	} else {
		authrouter.RegisterRoutes(apiBase, authFactory, cfg)
	}
	return nil
}
