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

	// Start the event handler only once
	w.startOnce.Do(func() {
		go w.handleEvents(ctx)
	})

	return nil
}

// OnChange registers a callback to be invoked when the configuration file changes.
func (w *Watcher) OnChange(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, callback)
}

// handleEvents processes file system events.
func (w *Watcher) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Handle write and create events
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
