package config

import (
	"context"
	"fmt"
	"sync"
)

// Global instance of Manager (unexported to force access via functions).
var (
	GlobalManager *Manager
	initOnce      sync.Once
	closeOnce     sync.Once
)

// Initialize sets up the global config manager exactly once.
// Call this early in your application (e.g., in main or init funcs).
// - ctx: Context for loading/watching.
// - service: Optional custom Service (defaults to NewService()).
// - sources: Configuration sources (e.g., NewYAMLProvider("config.yaml")).
// Returns error if loading fails.
func Initialize(ctx context.Context, service Service, sources ...Source) error {
	var initErr error
	initOnce.Do(func() {
		if service == nil {
			service = NewService()
		}
		GlobalManager = NewManager(service)

		// Load config and start watching (as per Manager.Load).
		_, err := GlobalManager.Load(ctx, sources...)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize global config: %w", err)
			// Reset globalManager on failure to allow retry (optional; otherwise, app should exit).
			GlobalManager = nil
			return
		}
	})
	return initErr
}

// Get returns the current Config atomically.
// Panics if not initialized (alternative: return nil, error).
func Get() *Config {
	if GlobalManager == nil {
		panic("config not initialized; call config.Initialize first")
	}
	return GlobalManager.Get()
}

// OnChange registers a callback for config changes (thread-safe).
// Panics if not initialized.
func OnChange(callback func(*Config)) {
	if GlobalManager == nil {
		panic("config not initialized; call config.Initialize first")
	}
	GlobalManager.OnChange(callback)
}

// Reload forces a reload from sources (thread-safe via Manager's mutex).
// Panics if not initialized.
func Reload(ctx context.Context) error {
	if GlobalManager == nil {
		panic("config not initialized; call config.Initialize first")
	}
	return GlobalManager.Reload(ctx)
}

// Close shuts down the global manager (stops watchers, closes sources).
// Idempotent (can be called multiple times safely).
func Close(ctx context.Context) error {
	var closeErr error
	closeOnce.Do(func() {
		if GlobalManager != nil {
			closeErr = GlobalManager.Close(ctx)
			GlobalManager = nil // Allow re-init if needed (e.g., in long-running apps).
		}
	})
	return closeErr
}

// For testing only: Reset the singleton state (unexported).
func resetForTest() {
	initOnce = sync.Once{}
	closeOnce = sync.Once{}
	GlobalManager = nil
}
