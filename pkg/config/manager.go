package config

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/pkg/logger"
)

// Manager handles configuration with atomic updates and hot-reload support.
type Manager struct {
	Service     Service
	current     atomic.Value // stores *Config
	sources     []Source
	callbacks   []func(*Config)
	callbackMu  sync.RWMutex
	reloadMu    sync.Mutex
	watchCtx    context.Context
	watchCancel context.CancelFunc
	watchWg     sync.WaitGroup
	closeOnce   sync.Once
	debounce    time.Duration // configurable debounce duration for file watching
}

// NewManager creates a new configuration manager.
func NewManager(service Service) *Manager {
	if service == nil {
		service = NewService()
	}
	return &Manager{
		Service:   service,
		callbacks: make([]func(*Config), 0),
		debounce:  100 * time.Millisecond, // default debounce
	}
}

// Load loads configuration from sources and starts watching for changes.
func (m *Manager) Load(ctx context.Context, sources ...Source) (*Config, error) {
	// Store sources for reload (copy to avoid caller mutation)
	m.reloadMu.Lock()
	m.sources = append([]Source(nil), sources...)
	m.reloadMu.Unlock()

	// Initial load
	config, err := m.Service.Load(ctx, sources...)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply configuration atomically and notify callbacks
	m.applyConfig(config)

	// Rebind internal watch context to a non-canceling derivative of the caller's context
	if ctx != nil {
		// Cancel any existing watcher ctx to prevent leaks
		if m.watchCancel != nil {
			m.watchCancel()
		}
		base := context.WithoutCancel(ctx)
		m.watchCtx, m.watchCancel = context.WithCancel(base)
	}

	// Start watching sources that support it
	m.startWatching(sources)

	return config, nil
}

// Sources returns a copy of the currently configured sources.
func (m *Manager) Sources() []Source {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	if len(m.sources) == 0 {
		return []Source{}
	}
	out := make([]Source, len(m.sources))
	copy(out, m.sources)
	return out
}

// Get returns the current configuration atomically.
func (m *Manager) Get() *Config {
	val := m.current.Load()
	if val == nil {
		return nil
	}
	config, ok := val.(*Config)
	if !ok {
		return nil
	}
	return config
}

// Reload forces a configuration reload from all sources.
func (m *Manager) Reload(ctx context.Context) error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	// Load configuration from sources
	newConfig, err := m.Service.Load(ctx, m.sources...)
	if err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	// Validate the new configuration before applying
	if err := m.Service.Validate(newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Apply the new configuration atomically
	m.applyConfig(newConfig)

	return nil
}

// SetDebounce sets the debounce duration for file watching.
// Must be called before Load() to take effect.
func (m *Manager) SetDebounce(duration time.Duration) {
	m.debounce = duration
}

// OnChange registers a callback to be invoked when configuration changes.
func (m *Manager) OnChange(callback func(*Config)) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// Close stops watching and releases resources.
func (m *Manager) Close(ctx context.Context) error {
	// Use sync.Once to ensure we only close once
	m.closeOnce.Do(func() {
		// Cancel watch context
		if m.watchCancel != nil {
			m.watchCancel()
		}

		// Wait for all watchers to finish
		m.watchWg.Wait()

		m.reloadMu.Lock()
		sourcesCopy := append([]Source(nil), m.sources...)
		m.reloadMu.Unlock()
		// Close all sources using a copy to avoid holding locks during Close()
		for _, source := range sourcesCopy {
			if source != nil {
				if err := source.Close(); err != nil {
					logger.FromContext(ctx).Error("failed to close configuration source", "error", err)
				}
			}
		}
	})

	return nil
}

// startWatching sets up file watching for sources that support it.
func (m *Manager) startWatching(sources []Source) {
	for _, source := range sources {
		if source == nil {
			continue
		}
		src := source
		m.watchWg.Add(1)
		go func() {
			defer m.watchWg.Done()
			ctx := m.watchCtx
			if ctx == nil {
				ctx = context.Background()
			}
			err := src.Watch(ctx, func() {
				if m.debounce > 0 {
					time.Sleep(m.debounce)
				}
				if err := m.Reload(ctx); err != nil {
					logger.FromContext(ctx).Error("failed to reload configuration", "error", err)
				}
			})

			if err != nil {
				logger.FromContext(ctx).Debug("source does not support watching", "error", err)
			}
		}()
	}
}

// applyConfig applies a new configuration atomically and notifies callbacks.
func (m *Manager) applyConfig(config *Config) {
	// Store new configuration atomically
	oldConfig := m.Get()
	m.current.Store(config)

	// Skip callbacks if configuration hasn't changed
	if oldConfig != nil && configEqual(oldConfig, config) {
		return
	}

	// Get callbacks under lock
	m.callbackMu.RLock()
	callbacks := make([]func(*Config), len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.callbackMu.RUnlock()

	// Invoke callbacks outside of lock
	for _, callback := range callbacks {
		if callback != nil {
			callback(config)
		}
	}
}

// configEqual performs a deep equality check on configurations.
// It compares all fields to determine if configurations are functionally equivalent.
func configEqual(a, b *Config) bool {
	return reflect.DeepEqual(a, b)
}
