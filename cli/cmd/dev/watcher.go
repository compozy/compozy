package dev

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cliutils "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/fsnotify/fsnotify"
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

// RunWithWatcher sets up file watching and runs the server with restart capability
func RunWithWatcher(ctx context.Context, cwd, configFile, envFilePath string) error {
	watcher, err := setupWatcher(ctx, cwd)
	if err != nil {
		return err
	}
	defer watcher.Close()

	restartChan := make(chan bool, 1)
	go startWatcher(ctx, watcher, restartChan)

	// Integrate with config hot-reload via manager in context
	config.ManagerFromContext(ctx).OnChange(func(_ *config.Config) {
		log := logger.FromContext(ctx)
		log.Info("Configuration change detected, triggering restart")
		select {
		case restartChan <- true:
			// enqueued
		default:
			// restart already pending
		}
	})

	return runAndWatchServer(ctx, cwd, configFile, envFilePath, restartChan)
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
		if !info.IsDir() && isYAMLFile(path) {
			fileCount++
			dir := filepath.Dir(path)
			dirsToWatch[dir] = true
		}
		return nil
	}); err != nil {
		if closeErr := watcher.Close(); closeErr != nil {
			log.Warn("Failed to close watcher during error cleanup", "error", closeErr)
		}
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
				if isYAMLFile(event.Name) {
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
	cwd, configFile, envFilePath string,
	restartChan chan bool,
) error {
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing configuration in context")
	}
	var retryDelay = initialRetryDelay

	for {
		// Find available port on each restart
		availablePort, err := cliutils.FindAvailablePort(ctx, cfg.Server.Host, cfg.Server.Port)
		if err != nil {
			return fmt.Errorf("no free port found near %d: %w", cfg.Server.Port, err)
		}
		if availablePort != cfg.Server.Port {
			log.Info("port conflict on restart, using next available port",
				"original_port", cfg.Server.Port,
				"available_port", availablePort)
			cfg.Server.Port = availablePort
		}

		srv, err := server.NewServer(ctx, cwd, configFile, envFilePath)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}
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

// isYAMLFile checks if a file has a YAML extension (case-insensitive)
func isYAMLFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}
