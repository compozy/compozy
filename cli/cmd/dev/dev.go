package dev

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/cli/cmd"
	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// NewDevCommand creates the dev command using the unified command pattern
func NewDevCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the Compozy development server",
		RunE:  executeDevCommand,
	}

	// Add development-specific flags
	cmd.Flags().Bool("watch", false, "Enable file watcher to restart server on change")

	return cmd
}

// executeDevCommand handles the dev command execution using the unified executor pattern
func executeDevCommand(cobraCmd *cobra.Command, args []string) error {
	return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
	}, cmd.ModeHandlers{
		JSON: handleDevJSON,
		TUI:  handleDevTUI,
	}, args)
}

// handleDevJSON handles dev command in JSON mode
func handleDevJSON(ctx context.Context, cobraCmd *cobra.Command, _ *cmd.CommandExecutor, _ []string) error {
	return runDevServer(ctx, cobraCmd)
}

// handleDevTUI handles dev command in TUI mode (same as JSON for dev server)
func handleDevTUI(ctx context.Context, cobraCmd *cobra.Command, _ *cmd.CommandExecutor, _ []string) error {
	return runDevServer(ctx, cobraCmd)
}

// runDevServer runs the development server with the provided configuration
func runDevServer(ctx context.Context, cobraCmd *cobra.Command) error {
	cfg := config.Get()

	// Setup development environment
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.DebugMode)
	}

	// Change to the specified working directory if provided
	if cfg.CLI.CWD != "" {
		if err := os.Chdir(cfg.CLI.CWD); err != nil {
			return fmt.Errorf("failed to change working directory to %s: %w", cfg.CLI.CWD, err)
		}
	}

	log := logger.FromContext(ctx)

	// Get the current working directory (after any --cwd change)
	CWD, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if cfg.CLI.CWD != "" {
		log.Info("Working directory changed", "cwd", cfg.CLI.CWD)
	}

	// Find available port
	availablePort, err := cliutils.FindAvailablePort(cfg.Server.Host, cfg.Server.Port)
	if err != nil {
		return fmt.Errorf("no free port found near %d: %w", cfg.Server.Port, err)
	}

	if availablePort != cfg.Server.Port {
		log.Info("Port unavailable, using alternative port",
			"requested_port", cfg.Server.Port, "available_port", availablePort)
		cfg.Server.Port = availablePort
	}

	// Check if watch mode is enabled
	watch, err := cobraCmd.Flags().GetBool("watch")
	if err != nil {
		return fmt.Errorf("failed to get watch flag: %w", err)
	}

	configFile, err := cobraCmd.Flags().GetString("config")
	if err != nil {
		return fmt.Errorf("failed to get config flag: %w", err)
	}

	// If no config file specified, look for default compozy.yaml in CWD
	if configFile == "" {
		configFile = "compozy.yaml"
	}

	// Get environment file path from flags
	envFilePath, err := cobraCmd.Flags().GetString("env-file")
	if err != nil {
		return fmt.Errorf("failed to get env-file flag: %w", err)
	}
	if envFilePath == "" {
		envFilePath = ".env"
	}

	if watch {
		return RunWithWatcher(ctx, CWD, configFile, envFilePath)
	}

	srv := server.NewServer(ctx, CWD, configFile, envFilePath)
	defer config.Close(ctx) // Ensure global config cleanup on exit
	return srv.Run()
}
