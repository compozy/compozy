package dev

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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
	var watchedDirs sync.Map
	watcher, err := setupWatcher(ctx, cwd, &watchedDirs)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := watcher.Close(); closeErr != nil && !errors.Is(closeErr, fs.ErrClosed) {
			log := logger.FromContext(ctx)
			log.Warn("Failed to close watcher", "error", closeErr)
		}
	}()

	restartChan := make(chan bool, 1)
	go startWatcher(ctx, watcher, restartChan, &watchedDirs)

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
func setupWatcher(ctx context.Context, cwd string, watchedDirs *sync.Map) (*fsnotify.Watcher, error) {
	log := logger.FromContext(ctx)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	fileCount := 0
	dirCount := 0

	if err := filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			if isIgnoredDir(path) {
				return filepath.SkipDir
			}
			if watchDirectory(log, watcher, watchedDirs, path) {
				dirCount++
			}
		}

		// Count YAML files and track their parent directories
		if !info.IsDir() && isYAMLFile(path) {
			fileCount++
		}
		return nil
	}); err != nil {
		if closeErr := watcher.Close(); closeErr != nil {
			log.Warn("Failed to close watcher during error cleanup", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to walk project directory: %w", err)
	}

	log.Info("File watcher initialized",
		"yaml_files", fileCount,
		"watched_directories", dirCount)

	return watcher, nil
}

// startWatcher monitors file system events and triggers server restarts
func startWatcher(ctx context.Context, watcher *fsnotify.Watcher, restartChan chan bool, watchedDirs *sync.Map) {
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
			if handleDirectoryEvent(log, watcher, watchedDirs, event) {
				continue
			}
			if shouldRestartForEvent(event) {
				log.Debug("Detected file change, debouncing...", "file", event.Name, "op", event.Op.String())

				debounceMutex.Lock()
				pendingRestart = true

				// Reset the debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(fileChangeDebounceDelay, triggerRestart)
				debounceMutex.Unlock()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error("Watcher error", "error", err)
		}
	}
}

func handleDirectoryEvent(
	log logger.Logger,
	watcher *fsnotify.Watcher,
	watchedDirs *sync.Map,
	event fsnotify.Event,
) bool {
	if event.Name == "" {
		return false
	}
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			if isIgnoredDir(event.Name) {
				return true
			}
			watchDirectory(log, watcher, watchedDirs, event.Name)
			return true
		}
	}
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		if removeWatchedDirectory(log, watcher, watchedDirs, event.Name) {
			return true
		}
	}
	return false
}

func shouldRestartForEvent(event fsnotify.Event) bool {
	if event.Name == "" {
		return false
	}
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) &&
		!event.Has(fsnotify.Rename) {
		return false
	}
	return isYAMLFile(event.Name)
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
		if err := ctx.Err(); err != nil {
			log.Info("Context canceled before server start, stopping watcher loop")
			return err
		}
		if err := cliutils.EnsurePortAvailable(ctx, cfg.Server.Host, cfg.Server.Port); err != nil {
			return fmt.Errorf("development server port unavailable: %w", err)
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
			shutdownErr := <-serverErrChan // Wait for shutdown to complete
			if shutdownErr != nil {
				log.Warn("Server returned error while shutting down", "error", shutdownErr)
			}
			log.Info("Server shut down. Restarting...")
			// Reset retry delay on successful file-based restart
			retryDelay = initialRetryDelay
			// Drain the channel in case of multiple file change events
			for len(restartChan) > 0 {
				<-restartChan
			}
			continue // Restart the loop
		case <-ctx.Done():
			log.Info("Context canceled, initiating shutdown")
			srv.Shutdown()
			shutdownErr := <-serverErrChan
			if shutdownErr != nil {
				log.Warn("Server returned error while shutting down", "error", shutdownErr)
			}
			return ctx.Err()
		case err := <-serverErrChan:
			if err != nil {
				if isAddressInUse(err) {
					log.Error("Port already in use; stop the conflicting process or configure a new port",
						"host", cfg.Server.Host,
						"port", cfg.Server.Port,
						"error", err,
					)
					return fmt.Errorf(
						"development server failed to bind to %s:%d: %w",
						cfg.Server.Host,
						cfg.Server.Port,
						err,
					)
				}
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

func isAddressInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	lowered := strings.ToLower(err.Error())
	return strings.Contains(lowered, "address already in use")
}

func watchDirectory(log logger.Logger, watcher *fsnotify.Watcher, watchedDirs *sync.Map, dir string) bool {
	if _, exists := watchedDirs.LoadOrStore(dir, struct{}{}); exists {
		return false
	}
	if err := watcher.Add(dir); err != nil {
		watchedDirs.Delete(dir)
		log.Warn("Failed to watch directory", "path", dir, "error", err)
		return false
	}
	log.Debug("Watching directory", "path", dir)
	return true
}

func removeWatchedDirectory(log logger.Logger, watcher *fsnotify.Watcher, watchedDirs *sync.Map, path string) bool {
	if _, exists := watchedDirs.Load(path); !exists {
		return false
	}
	watchedDirs.Delete(path)
	if err := watcher.Remove(path); err != nil {
		log.Debug("Failed to remove directory watch", "path", path, "error", err)
	} else {
		log.Debug("Stopped watching directory", "path", path)
	}
	return true
}

func isIgnoredDir(path string) bool {
	baseName := filepath.Base(path)
	return ignoredDirs[baseName]
}

// isYAMLFile checks if a file has a YAML extension (case-insensitive)
func isYAMLFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yaml" || ext == ".yml"
}
