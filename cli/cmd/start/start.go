package start

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// Constants for production start command
const (
	productionEnvironment = "production"
	disableSSLMode        = "disable"
	localhost             = "localhost"
)

// NewStartCommand creates the start command for production server
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"server"},
		Short:   "Start Compozy production server",
		Long:    "Start the Compozy server optimized for production use",
		RunE:    executeStartCommand,
	}
	return cmd
}

// executeStartCommand handles the start command execution using the unified executor pattern
func executeStartCommand(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
	}, cmd.ModeHandlers{
		JSON: handleStartJSON,
		TUI:  handleStartTUI,
	}, args)
}

// handleStartTUI handles start command in TUI mode
func handleStartTUI(ctx context.Context, _ *cobra.Command, _ *cmd.CommandExecutor, _ []string) error {
	cfg := config.FromContext(ctx)
	cfg.Runtime.Environment = productionEnvironment
	gin.SetMode(gin.ReleaseMode)
	logProductionSecurityWarnings(ctx, cfg)
	if !helpers.IsPortAvailable(cfg.Server.Host, cfg.Server.Port) {
		return fmt.Errorf("port %d is not available on host %s", cfg.Server.Port, cfg.Server.Host)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	configFile := cfg.CLI.ConfigFile
	envFilePath := cfg.CLI.EnvFile
	srv := server.NewServer(ctx, cwd, configFile, envFilePath)
	return srv.Run()
}

// handleStartJSON handles start command in JSON mode
func handleStartJSON(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	return handleStartTUI(ctx, nil, executor, nil)
}

// logProductionSecurityWarnings warns about disabled security features in production
func logProductionSecurityWarnings(ctx context.Context, cfg *config.Config) {
	log := logger.FromContext(ctx)
	if !cfg.Server.Auth.Enabled {
		log.Warn("🚨 SECURITY WARNING: Authentication is disabled in production!")
		log.Warn("   Consider enabling authentication: server.auth.enabled=true")
	}
	if cfg.Database.SSLMode == disableSSLMode {
		log.Warn("🔒 SECURITY WARNING: Database SSL is disabled in production!")
		log.Warn("   Consider enabling SSL: database.ssl_mode=require")
	}
	for _, origin := range cfg.Server.CORS.AllowedOrigins {
		if strings.Contains(origin, localhost) {
			log.Warn("🌐 SECURITY WARNING: CORS allows localhost origins in production!")
			log.Warn("   Consider reviewing CORS configuration for production")
			break
		}
	}
	if cfg.RateLimit.GlobalRate.Limit == 0 {
		log.Warn("⚡ INFO: Rate limiting is disabled")
		log.Warn("   Consider enabling rate limiting for production protection")
	}
}
