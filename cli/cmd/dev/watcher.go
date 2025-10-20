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
	config.ManagerFromContext(ctx).OnChange(func(_ *config.Config) {
		log := logger.FromContext(ctx)
		log.Info("Configuration change detected, triggering restart")
		select {
		case restartChan <- true:
		default:
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

		if info.IsDir() {
			if isIgnoredDir(path) {
				return filepath.SkipDir
			}
			if watchDirectory(log, watcher, watchedDirs, path) {
				dirCount++
			}
		}

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
	debouncer := newRestartDebouncer(log, restartChan)
	for {
		select {
		case <-ctx.Done():
			log.Info("Context canceled, stopping file watcher")
			debouncer.stop()
			return
		case event, ok := <-watcher.Events:
			if !ok {
				debouncer.stop()
				return
			}
			processWatcherEvent(log, watcher, watchedDirs, event, debouncer)
		case err, ok := <-watcher.Errors:
			if !ok {
				debouncer.stop()
				return
			}
			log.Error("Watcher error", "error", err)
		}
	}
}

type restartDebouncer struct {
	mu          sync.Mutex
	timer       *time.Timer
	log         logger.Logger
	restartChan chan bool
	pending     bool
}

func newRestartDebouncer(log logger.Logger, restartChan chan bool) *restartDebouncer {
	return &restartDebouncer{log: log, restartChan: restartChan}
}

func (d *restartDebouncer) schedule() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending = true
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(fileChangeDebounceDelay, d.flush)
}

func (d *restartDebouncer) flush() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.pending {
		return
	}
	select {
	case d.restartChan <- true:
		d.log.Debug("Sending restart signal after debounce")
	default:
	}
	d.pending = false
}

func (d *restartDebouncer) stop() {
	d.mu.Lock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.pending = false
	d.mu.Unlock()
}

func processWatcherEvent(
	log logger.Logger,
	watcher *fsnotify.Watcher,
	watchedDirs *sync.Map,
	event fsnotify.Event,
	debouncer *restartDebouncer,
) {
	if handleDirectoryEvent(log, watcher, watchedDirs, event) {
		return
	}
	if shouldRestartForEvent(event) {
		log.Debug("Detected file change, debouncing...", "file", event.Name, "op", event.Op.String())
		debouncer.schedule()
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
	retryDelay := initialRetryDelay
	for {
		if err := ensureServerReady(ctx, log, cfg); err != nil {
			return err
		}
		srv, serverErrChan, err := startDevServer(ctx, cwd, configFile, envFilePath)
		if err != nil {
			return err
		}

		log.Info("Server started. Watching for file changes.")

		decision, err := waitForServerEvent(ctx, srv, restartChan, serverErrChan, log, cfg)
		switch decision {
		case serverDecisionRestart:
			retryDelay = initialRetryDelay
			drainRestartSignals(restartChan)
			continue
		case serverDecisionRetry:
			log.Debug("Waiting before retry...", "delay", retryDelay)
			time.Sleep(retryDelay)
			retryDelay = nextRetryDelay(retryDelay)
			continue
		case serverDecisionStop:
			return err
		}
	}
}

type serverDecision int

const (
	serverDecisionRestart serverDecision = iota
	serverDecisionRetry
	serverDecisionStop
)

func ensureServerReady(ctx context.Context, log logger.Logger, cfg *config.Config) error {
	if err := ctx.Err(); err != nil {
		log.Info("Context canceled before server start, stopping watcher loop")
		return err
	}
	if err := cliutils.EnsurePortAvailable(ctx, cfg.Server.Host, cfg.Server.Port); err != nil {
		return fmt.Errorf("development server port unavailable: %w", err)
	}
	return nil
}

func startDevServer(ctx context.Context, cwd, configFile, envFilePath string) (*server.Server, <-chan error, error) {
	srv, err := server.NewServer(ctx, cwd, configFile, envFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create server: %w", err)
	}
	serverErrChan := make(chan error, 1)
	go func() {
		serverErrChan <- srv.Run()
	}()
	return srv, serverErrChan, nil
}

func waitForServerEvent(
	ctx context.Context,
	srv *server.Server,
	restartChan chan bool,
	serverErrChan <-chan error,
	log logger.Logger,
	cfg *config.Config,
) (serverDecision, error) {
	select {
	case <-restartChan:
		log.Info("Restart signal received. Shutting down server...")
		shutdownServer(log, srv, serverErrChan)
		log.Info("Server shut down. Restarting...")
		return serverDecisionRestart, nil
	case <-ctx.Done():
		log.Info("Context canceled, initiating shutdown")
		shutdownServer(log, srv, serverErrChan)
		return serverDecisionStop, ctx.Err()
	case err := <-serverErrChan:
		if err != nil {
			if isAddressInUse(err) {
				log.Error("Port already in use; stop the conflicting process or configure a new port",
					"host", cfg.Server.Host,
					"port", cfg.Server.Port,
					"error", err,
				)
				return serverDecisionStop, fmt.Errorf(
					"development server failed to bind to %s:%d: %w",
					cfg.Server.Host,
					cfg.Server.Port,
					err,
				)
			}
			log.Error("Server stopped with error", "error", err)
			return serverDecisionRetry, err
		}
		log.Info("Server stopped.")
		return serverDecisionStop, nil
	}
}

func shutdownServer(log logger.Logger, srv *server.Server, serverErrChan <-chan error) {
	srv.Shutdown()
	if shutdownErr := <-serverErrChan; shutdownErr != nil {
		log.Warn("Server returned error while shutting down", "error", shutdownErr)
	}
}

func drainRestartSignals(restartChan chan bool) {
	for len(restartChan) > 0 {
		<-restartChan
	}
}

func nextRetryDelay(current time.Duration) time.Duration {
	next := current * 2
	if next > maxRetryDelay {
		return maxRetryDelay
	}
	return next
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
