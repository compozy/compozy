package config

import (
	"context"
	"errors"
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
	// We keep the per-path context so we can ignore events immediately when the context is canceled.
	watched   map[string]context.Context
	stopCh    chan struct{} // Signals all per-path goroutines to stop
	startOnce sync.Once     // Ensures handleEvents goroutine is started only once
	closeOnce sync.Once     // Ensures Close is idempotent
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
		watched:   make(map[string]context.Context),
		stopCh:    make(chan struct{}),
	}, nil
}

// Watch starts watching the specified file for changes.
func (w *Watcher) Watch(ctx context.Context, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	if err := w.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to watch file: %w", err)
	}
	w.mu.Lock()
	w.watched[absPath] = ctx
	w.mu.Unlock()
	if done := ctx.Done(); done != nil {
		go func(p string, done <-chan struct{}) {
			select {
			case <-done:
			case <-w.stopCh:
			}
			w.mu.Lock()
			delete(w.watched, p)
			w.mu.Unlock()
			if err := w.watcher.Remove(p); err != nil {
				if !errors.Is(err, fsnotify.ErrClosed) {
					_ = err
				}
			}
		}(absPath, done)
	}
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
			w.mu.RLock()
			pathCtx, stillWatched := w.watched[event.Name]
			w.mu.RUnlock()
			if !stillWatched {
				continue
			}
			if pathCtx != nil && pathCtx.Err() != nil {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				w.notifyCallbacks()
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			if err != nil {
				// TODO: Add logger injection for proper error logging
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
	for _, callback := range callbacks {
		if callback != nil {
			callback()
		}
	}
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	var closeErr error
	w.closeOnce.Do(func() {
		close(w.stopCh)
		if err := w.watcher.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close watcher: %w", err)
		}
	})
	return closeErr
}
