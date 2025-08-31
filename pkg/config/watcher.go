package config

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher manages file watching for configuration hot-reload.
type Watcher struct {
	watcher   *fsnotify.Watcher
	callbacks []func()
	mu        sync.RWMutex
	// watched keeps track of actively watched absolute file paths.
	// It's used to filter events and to support per-call context cancellation.
	watched   map[string]struct{}
	startOnce sync.Once // Ensures handleEvents goroutine is started only once
	closeOnce sync.Once // Ensures Close is idempotent
}

// NewWatcher creates a new configuration file watcher.
func NewWatcher() (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Watcher{
		watcher:   fsWatcher,
		callbacks: make([]func(), 0),
		watched:   make(map[string]struct{}),
	}, nil
}

// Watch starts watching the specified file for changes.
func (w *Watcher) Watch(ctx context.Context, path string) error {
	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Add the file to the watcher
	if err := w.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to watch file: %w", err)
	}

	// Mark path as being watched
	w.mu.Lock()
	w.watched[absPath] = struct{}{}
	w.mu.Unlock()

	// Stop watching this specific path when the provided context is canceled
	if ctx != nil {
		go func(p string, c context.Context) {
			<-c.Done()
			// Remove from internal registry first to filter any in-flight events
			w.mu.Lock()
			delete(w.watched, p)
			w.mu.Unlock()
			// Best-effort removal from fsnotify watcher; ignore error if already removed/closed
			if err := w.watcher.Remove(p); err != nil {
				// Intentionally ignore error: removal may fail if watcher is already closed
				// or path was already removed. Since this happens during context cancellation,
				// we cannot propagate the error and logging is not yet available.
				// TODO: Add logger injection for proper error logging
				_ = err
			}
		}(absPath, ctx)
	}

	// Start the event handler only once
	w.startOnce.Do(func() {
		go w.handleEvents()
	})

	return nil
}

// OnChange registers a callback to be invoked when the configuration file changes.
func (w *Watcher) OnChange(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, callback)
}

// handleEvents processes file system events until the watcher is closed.
func (w *Watcher) handleEvents() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// Filter out events for paths that are no longer being watched
			w.mu.RLock()
			_, stillWatched := w.watched[event.Name]
			w.mu.RUnlock()
			if !stillWatched {
				continue
			}

			// Handle write and create events only when active
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				w.notifyCallbacks()
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Log the error but continue watching
			if err != nil {
				// TODO: Add logger injection for proper error logging
				// For now, silently continue to avoid fmt.Printf in production
				_ = err
			}
		}
	}
}

// notifyCallbacks invokes all registered callbacks.
func (w *Watcher) notifyCallbacks() {
	w.mu.RLock()
	callbacks := make([]func(), len(w.callbacks))
	copy(callbacks, w.callbacks)
	w.mu.RUnlock()

	// Execute callbacks outside of the lock
	for _, callback := range callbacks {
		if callback != nil {
			callback()
		}
	}
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	var closeErr error

	// Use sync.Once to ensure we only close once
	w.closeOnce.Do(func() {
		if err := w.watcher.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close watcher: %w", err)
		}
	})

	return closeErr
}
