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

const defaultConfigFile = "compozy.yaml"

// NewDevCommand creates the dev command using the unified command pattern
func NewDevCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the Compozy development server",
		RunE:  executeDevCommand,
	}
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
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing config in context; ensure config.ContextWithManager is set in root command")
	}
	manager := config.ManagerFromContext(ctx)
	if manager == nil {
		return fmt.Errorf("configuration manager missing from context")
	}
	ctxManager, ok := ctx.Value(config.ManagerCtxKey).(*config.Manager)
	owned := ok && ctxManager == manager
	defer func() {
		if !owned {
			return
		}
		if err := manager.Close(ctx); err != nil {
			log.Warn("failed to close config manager", "error", err)
		}
	}()
	// NOTE: Embedded Temporal dev server is no longer supported; require an external Temporal endpoint.
	setupGinMode(cfg)
	CWD, err := setupWorkingDirectory(ctx, cfg)
	if err != nil {
		return err
	}
	envFilePath := resolveEnvFilePath(cobraCmd, CWD)
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
	if runErr := srv.Run(); runErr != nil {
		return runErr
	}
	return nil
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

// resolveEnvFilePath resolves the environment file path against baseDir when relative
func resolveEnvFilePath(cobraCmd *cobra.Command, baseDir string) string {
	var envFilePath string
	if flag := cobraCmd.Flags().Lookup("env-file"); flag != nil {
		envFilePath = flag.Value.String()
	} else if flag := cobraCmd.InheritedFlags().Lookup("env-file"); flag != nil {
		envFilePath = flag.Value.String()
	}
	if envFilePath == "" {
		envFilePath = ".env"
	}
	if filepath.IsAbs(envFilePath) {
		return envFilePath
	}
	candidate := filepath.Join(baseDir, envFilePath)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if wd, err := os.Getwd(); err == nil {
		fallback := filepath.Join(wd, envFilePath)
		if _, statErr := os.Stat(fallback); statErr == nil {
			return fallback
		}
	}
	return candidate
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

// setupServerPort verifies the configured server port is available before starting
func setupServerPort(ctx context.Context, cfg *config.Config) error {
	if err := cliutils.EnsurePortAvailable(ctx, cfg.Server.Host, cfg.Server.Port); err != nil {
		return fmt.Errorf("development server port unavailable: %w", err)
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
		configFile = defaultConfigFile
	}
	return watch, configFile, nil
}
