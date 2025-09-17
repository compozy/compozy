package server

import (
	"context"

	"github.com/compozy/compozy/docs"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func setupBasicConfiguration(ctx context.Context) (string, string) {
	version := core.GetVersion()
	prefixURL := routes.Base()
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if cfg == nil {
		log.Warn("config manager not found in context; cannot check admin bootstrap key")
		return version, prefixURL
	}
	if cfg.Server.Auth.AdminKey.Value() != "" {
		log.Info("Admin bootstrap key is configured")
	} else {
		log.Info("No admin bootstrap key configured")
	}
	return version, prefixURL
}

func logRegistrationComplete(ctx context.Context, state *appstate.State) {
	log := logger.FromContext(ctx)
	authEnabled := false
	if cfg := config.FromContext(ctx); cfg != nil {
		authEnabled = cfg.Server.Auth.Enabled
	} else {
		log.Warn("config manager not found in context; auth_enabled will default to false")
	}
	log.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
		"swagger_base_path", docs.SwaggerInfo.BasePath,
		"auth_enabled", authEnabled,
	)
}
