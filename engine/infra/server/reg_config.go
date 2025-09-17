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

func setupBasicConfiguration(ctx context.Context, cfg *config.Config) (string, string) {
	version := core.GetVersion()
	prefixURL := routes.Base()
	log := logger.FromContext(ctx)
	if cfg.Server.Auth.AdminKey.Value() != "" {
		log.Info("Admin bootstrap key is configured")
	} else {
		log.Info("No admin bootstrap key configured")
	}
	return version, prefixURL
}

func logRegistrationComplete(ctx context.Context, state *appstate.State, cfg *config.Config) {
	log := logger.FromContext(ctx)
	log.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
		"swagger_base_path", docs.SwaggerInfo.BasePath,
		"auth_enabled", cfg.Server.Auth.Enabled,
	)
}
