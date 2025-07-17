package dev

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/compozy/compozy/cli/cmd"
	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// Constants for dev server configuration
const (
	// Server restart delays
	initialRetryDelay = 500 * time.Millisecond
	maxRetryDelay     = 30 * time.Second

	// File watcher debounce delay
	fileChangeDebounceDelay = 200 * time.Millisecond
)

// ignoredDirs contains directories that should be skipped during file watching
var ignoredDirs = map[string]bool{
	".git":          true,
	"node_modules":  true,
	".idea":         true,
	".vscode":       true,
	"vendor":        true,
	"dist":          true,
	"build":         true,
	"tmp":           true,
	"temp":          true,
	".cache":        true,
	"__pycache__":   true,
	".pytest_cache": true,
	".next":         true,
	".nuxt":         true,
	"coverage":      true,
}

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
		RequireAPI:  false,
	}, cmd.ModeHandlers{
		JSON: handleDevJSON,
		TUI:  handleDevTUI,
	}, args)
}

// handleDevJSON handles dev command in JSON mode
func handleDevJSON(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	return runDevServer(ctx, cobraCmd, executor)
}

// handleDevTUI handles dev command in TUI mode (same as JSON for dev server)
func handleDevTUI(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
	return runDevServer(ctx, cobraCmd, executor)
}

// runDevServer runs the development server with the provided configuration
func runDevServer(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor) error {
	cfg := executor.GetConfig()

	// Setup development environment
	gin.SetMode(gin.ReleaseMode)

	// Environment file is loaded globally in SetupGlobalConfig
	// No need to load it again here

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
		return runWithWatcher(ctx, cfg, CWD, configFile, envFilePath)
	}

	srv := server.NewServer(ctx, cfg, CWD, configFile, envFilePath)
	return srv.Run()
}

// runWithWatcher sets up file watching and runs the server with restart capability
func runWithWatcher(ctx context.Context, cfg *config.Config, cwd, configFile, envFilePath string) error {
	watcher, err := setupWatcher(ctx, cwd)
	if err != nil {
		return err
	}
	defer watcher.Close()

	restartChan := make(chan bool, 1)
	go startWatcher(ctx, watcher, restartChan)

	return runAndWatchServer(ctx, cfg, cwd, configFile, envFilePath, restartChan)
}

// setupWatcher creates and configures the file system watcher
func setupWatcher(ctx context.Context, cwd string) (*fsnotify.Watcher, error) {
	log := logger.FromContext(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Close watcher when context is canceled to prevent goroutine leaks
	go func() {
		<-ctx.Done()
		_ = watcher.Close()
	}()

	// For large projects, watch directories instead of individual files
	dirsToWatch := make(map[string]bool)
	fileCount := 0

	if err := filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			baseName := filepath.Base(path)
			if ignoredDirs[baseName] {
				return filepath.SkipDir
			}
		}

		// Count YAML files and track their parent directories
		if !info.IsDir() && (filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml") {
			fileCount++
			dir := filepath.Dir(path)
			dirsToWatch[dir] = true
		}
		return nil
	}); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to walk project directory: %w", err)
	}

	// Watch directories containing YAML files
	for dir := range dirsToWatch {
		if err := watcher.Add(dir); err != nil {
			log.Warn("Failed to watch directory", "path", dir, "error", err)
		}
	}

	log.Info("File watcher initialized",
		"yaml_files", fileCount,
		"watched_directories", len(dirsToWatch))

	return watcher, nil
}

// startWatcher monitors file system events and triggers server restarts
func startWatcher(ctx context.Context, watcher *fsnotify.Watcher, restartChan chan bool) {
	log := logger.FromContext(ctx)

	// Debounce timer to batch multiple file changes
	var debounceTimer *time.Timer
	var pendingRestart bool
	debounceMutex := &sync.Mutex{}

	triggerRestart := func() {
		debounceMutex.Lock()
		defer debounceMutex.Unlock()

		if pendingRestart {
			select {
			case restartChan <- true:
				log.Debug("Sending restart signal after debounce")
			default:
				// Restart already pending
			}
			pendingRestart = false
		}
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("Context canceled, stopping file watcher")
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				// Only react to YAML file changes
				ext := filepath.Ext(event.Name)
				if ext == ".yaml" || ext == ".yml" {
					log.Debug("Detected file change, debouncing...", "file", event.Name)

					debounceMutex.Lock()
					pendingRestart = true

					// Reset the debounce timer
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(fileChangeDebounceDelay, triggerRestart)
					debounceMutex.Unlock()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error("Watcher error", "error", err)
		}
	}
}

// runAndWatchServer runs the server with restart capability
func runAndWatchServer(
	ctx context.Context,
	cfg *config.Config,
	cwd, configFile, envFilePath string,
	restartChan chan bool,
) error {
	log := logger.FromContext(ctx)
	var retryDelay = initialRetryDelay

	for {
		// Find available port on each restart
		availablePort, err := cliutils.FindAvailablePort(cfg.Server.Host, cfg.Server.Port)
		if err != nil {
			return fmt.Errorf("no free port found near %d: %w", cfg.Server.Port, err)
		}
		if availablePort != cfg.Server.Port {
			log.Info("port conflict on restart, using next available port",
				"original_port", cfg.Server.Port,
				"available_port", availablePort)
			cfg.Server.Port = availablePort
		}

		srv := server.NewServer(ctx, cfg, cwd, configFile, envFilePath)
		serverErrChan := make(chan error, 1)
		go func() {
			serverErrChan <- srv.Run()
		}()

		log.Info("Server started. Watching for file changes.")

		select {
		case <-restartChan:
			log.Info("Restart signal received. Shutting down server...")
			srv.Shutdown()
			<-serverErrChan // Wait for shutdown to complete
			log.Info("Server shut down. Restarting...")
			// Reset retry delay on successful file-based restart
			retryDelay = initialRetryDelay
			// Drain the channel in case of multiple file change events
			for len(restartChan) > 0 {
				<-restartChan
			}
			continue // Restart the loop
		case err := <-serverErrChan:
			if err != nil {
				log.Error("Server stopped with error", "error", err)
				// Use exponential back-off to prevent tight restart loops on server failures
				log.Debug("Waiting before retry...", "delay", retryDelay)
				time.Sleep(retryDelay)
				// Double the delay for next retry, up to maximum
				retryDelay *= 2
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
				continue // Retry after back-off
			}
			log.Info("Server stopped.")
			return nil
		}
	}
}
