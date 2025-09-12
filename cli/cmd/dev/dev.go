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
	setupGinMode(cfg)
	envFilePath, err := resolveEnvFilePath(cobraCmd)
	if err != nil {
		return err
	}
	CWD, err := setupWorkingDirectory(ctx, cfg)
	if err != nil {
		return err
	}
	if err := setupServerPort(cfg); err != nil {
		return err
	}
	watch, configFile, err := getCommandFlags(cobraCmd)
	if err != nil {
		return err
	}
	if watch {
		return RunWithWatcher(ctx, CWD, configFile, envFilePath)
	}
	srv := server.NewServer(ctx, CWD, configFile, envFilePath)
	defer config.ManagerFromContext(ctx).Close(ctx)
	return srv.Run()
}

// setupGinMode configures Gin mode based on debug configuration
func setupGinMode(cfg *config.Config) {
	if os.Getenv("GIN_MODE") == "" {
		if cfg.CLI.Debug {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}
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
	if cfg.CLI.CWD != "" {
		if err := os.Chdir(cfg.CLI.CWD); err != nil {
			return "", fmt.Errorf("failed to change working directory to %s: %w", cfg.CLI.CWD, err)
		}
	}
	log := logger.FromContext(ctx)
	CWD, err := filepath.Abs(".")
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	if cfg.CLI.CWD != "" {
		log.Info("Working directory changed", "cwd", cfg.CLI.CWD)
	}
	return CWD, nil
}

// setupServerPort finds and configures an available port for the server
func setupServerPort(cfg *config.Config) error {
	availablePort, err := cliutils.FindAvailablePort(cfg.Server.Host, cfg.Server.Port)
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
