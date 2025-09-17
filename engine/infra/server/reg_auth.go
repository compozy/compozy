package server

import (
	"context"
	"fmt"

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
	server *Server,
) error {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing config in context")
	}
	authRepo := state.Store.NewAuthRepo()
	authFactory := authuc.NewFactory(authRepo)
	authManager := authmw.NewManager(authFactory, cfg)
	if cfg.Server.Auth.Enabled {
		apiBase.Use(authManager.Middleware())
		apiBase.Use(authManager.RequireAuth())
	}
	// Always pass runtime context; avoid context.Background() wrappers
	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		authrouter.RegisterRoutesWithMetrics(ctx, apiBase, authFactory, cfg, server.monitoring.Meter())
	} else {
		authrouter.RegisterRoutesWithMetrics(ctx, apiBase, authFactory, cfg, nil)
	}
	return nil
}
