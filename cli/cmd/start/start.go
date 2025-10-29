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

// Deployment mode constants (avoid magic strings)
const (
	modeStandalone  = "standalone"
	modeDistributed = "distributed"
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
	// Deployment mode flag controls global runtime mode for this command invocation.
	// Valid values: standalone, distributed.
	cmd.Flags().String("mode", "", "Deployment mode: standalone or distributed")
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
func handleStartTUI(ctx context.Context, cobraCmd *cobra.Command, _ *cmd.CommandExecutor, _ []string) error {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("configuration missing from context; attach a manager with config.ContextWithManager")
	}
	cfg.Mode = resolveStartMode(cobraCmd, config.ManagerFromContext(ctx).Service, cfg.Mode)
	if m := strings.TrimSpace(cfg.Mode); m != "" && m != modeStandalone && m != modeDistributed {
		return fmt.Errorf("invalid --mode value %q: must be one of [standalone distributed]", m)
	}
	cfg.Runtime.Environment = productionEnvironment
	gin.SetMode(gin.ReleaseMode)
	modeStr := cfg.Mode
	if modeStr == "" {
		modeStr = modeDistributed
	}
	logger.FromContext(ctx).Info("Starting Compozy server", "mode", modeStr)
	logProductionSecurityWarnings(ctx, cfg)
	if !helpers.IsPortAvailable(ctx, cfg.Server.Host, cfg.Server.Port) {
		return fmt.Errorf("port %d is not available on host %s", cfg.Server.Port, cfg.Server.Host)
	}
	if cfg.CLI.CWD == "" {
		if wd, err := os.Getwd(); err == nil {
			cfg.CLI.CWD = wd
		}
	}
	configFile := cfg.CLI.ConfigFile
	envFilePath := cfg.CLI.EnvFile
	srv, err := server.NewServer(ctx, cfg.CLI.CWD, configFile, envFilePath)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	return srv.Run()
}

// handleStartJSON handles start command in JSON mode
func handleStartJSON(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	return handleStartTUI(ctx, nil, executor, nil)
}

// sourceGetter defines the subset of the configuration service needed for mode precedence checks.
type sourceGetter interface {
	GetSource(key string) config.SourceType
}

// resolveStartMode applies the --mode flag if provided while respecting precedence.
// Config file values take precedence over CLI flag for the global deployment mode.
func resolveStartMode(cobraCmd *cobra.Command, svc sourceGetter, current string) string {
	if cobraCmd == nil || svc == nil {
		return current
	}
	flagVal, err := cobraCmd.Flags().GetString("mode")
	if err != nil {
		return current
	}
	mode := strings.TrimSpace(flagVal)
	if mode == "" {
		return current
	}
	src := svc.GetSource("mode")
	if src == config.SourceDefault || src == config.SourceEnv || src == config.SourceCLI {
		return mode
	}
	return current
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
