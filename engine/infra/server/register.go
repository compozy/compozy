package server

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes orchestrates the complete setup of all HTTP routes
func RegisterRoutes(ctx context.Context, router *gin.Engine, state *appstate.State, server *Server) error {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing config in context; ensure config.ContextWithManager is set before server init")
	}
	version, prefixURL := setupBasicConfiguration(ctx, cfg)
	apiBase := router.Group(prefixURL)
	if err := setupWebhookSystem(ctx, state, router, server, cfg); err != nil {
		return err
	}
	setupSwaggerAndDocs(router, prefixURL)
	setupDiagnosticEndpoints(router, version, prefixURL, server)
	if err := setupAuthSystem(ctx, apiBase, state, cfg, server); err != nil {
		return err
	}
	setupComponentRoutes(apiBase)
	logRegistrationComplete(ctx, state, cfg)
	return nil
}
