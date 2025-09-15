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
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing config in context; ensure config.ContextWithManager is set in root command")
	}
	// Ensure best dev experience: force embedded Temporal dev server in standalone mode
	// This avoids worker skip warnings when no external Temporal is running.
	if cfg.Mode == config.ModeStandalone {
		cfg.Temporal.DevServerEnabled = true
	}
	setupGinMode(cfg)
	envFilePath, err := resolveEnvFilePath(cobraCmd)
	if err != nil {
		return err
	}
	CWD, err := setupWorkingDirectory(ctx, cfg)
	if err != nil {
		return err
	}
	if err := setupServerPort(ctx, cfg); err != nil {
		return err
	}
	watch, configFile, err := getCommandFlags(cobraCmd)
	if err != nil {
		return err
	}
	if watch {
		return RunWithWatcher(ctx, CWD, configFile, envFilePath)
	}
	srv, err := server.NewServer(ctx, CWD, configFile, envFilePath)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	defer config.ManagerFromContext(ctx).Close(ctx)
	return srv.Run()
}

// setupGinMode configures Gin mode based on debug configuration
func setupGinMode(cfg *config.Config) {
	if os.Getenv("GIN_MODE") != "" {
		return
	}
	debug := cfg != nil && cfg.CLI.Debug
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
}

// resolveEnvFilePath gets and resolves the environment file path before directory changes
func resolveEnvFilePath(cobraCmd *cobra.Command) (string, error) {
	envFilePath, err := cobraCmd.Flags().GetString("env-file")
	if err != nil {
		return "", fmt.Errorf("failed to get env-file flag: %w", err)
	}
	if envFilePath == "" {
		envFilePath = ".env"
	}
	if !filepath.IsAbs(envFilePath) {
		originalCWD, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		envFilePath = filepath.Join(originalCWD, envFilePath)
	}
	return envFilePath, nil
}

// setupWorkingDirectory changes to the specified working directory and returns the absolute path
func setupWorkingDirectory(ctx context.Context, cfg *config.Config) (string, error) {
	log := logger.FromContext(ctx)
	if cfg.CLI.CWD == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		cfg.CLI.CWD = wd
	}
	abs, err := filepath.Abs(cfg.CLI.CWD)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}
	if abs != cfg.CLI.CWD {
		cfg.CLI.CWD = abs
	}
	log.Debug("Using working directory", "cwd", cfg.CLI.CWD)
	return cfg.CLI.CWD, nil
}

// setupServerPort finds and configures an available port for the server
func setupServerPort(ctx context.Context, cfg *config.Config) error {
	availablePort, err := cliutils.FindAvailablePort(ctx, cfg.Server.Host, cfg.Server.Port)
	if err != nil {
		return fmt.Errorf("no free port found near %d: %w", cfg.Server.Port, err)
	}
	if availablePort != cfg.Server.Port {
		// Note: We can't use logger here as context isn't available in this helper
		cfg.Server.Port = availablePort
	}
	return nil
}

// getCommandFlags extracts and validates command flags
func getCommandFlags(cobraCmd *cobra.Command) (bool, string, error) {
	watch, err := cobraCmd.Flags().GetBool("watch")
	if err != nil {
		return false, "", fmt.Errorf("failed to get watch flag: %w", err)
	}
	configFile, err := cobraCmd.Flags().GetString("config")
	if err != nil {
		return false, "", fmt.Errorf("failed to get config flag: %w", err)
	}
	if configFile == "" {
		configFile = "compozy.yaml"
	}
	return watch, configFile, nil
}
