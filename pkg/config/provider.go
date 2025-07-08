package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// envProvider is a placeholder for backward compatibility.
// The actual environment loading is now handled by koanf's native env provider.
type envProvider struct{}

// NewEnvProvider creates a new environment variable configuration source.
// Note: This is kept for backward compatibility, but the actual loading
// is handled by koanf's native env provider in loader.go
func NewEnvProvider() Source {
	return &envProvider{}
}

// Load returns empty map as environment loading is handled natively by koanf.
func (e *envProvider) Load() (map[string]any, error) {
	// Environment loading is now handled by koanf's native env provider
	return make(map[string]any), nil
}

// Watch is not implemented for environment variables as they don't change at runtime.
func (e *envProvider) Watch(_ context.Context, _ func()) error {
	return nil
}

// Type returns the source type identifier.
func (e *envProvider) Type() SourceType {
	return SourceEnv
}

// Close releases any resources held by the source.
func (e *envProvider) Close() error {
	return nil
}

// cliProvider implements Source interface for CLI flags.
type cliProvider struct {
	flags map[string]any
}

// NewCLIProvider creates a new CLI flags configuration source.
func NewCLIProvider(flags map[string]any) Source {
	return &cliProvider{
		flags: flags,
	}
}

// Load returns the CLI flags as configuration data.
func (c *cliProvider) Load() (map[string]any, error) {
	if c.flags == nil {
		return make(map[string]any), nil
	}

	// Define CLI flag to configuration path mappings
	// These mappings translate command-line flags to their corresponding
	// configuration structure paths using dot notation.
	var flagToPath = map[string]string{
		"host":                            "server.host",
		"port":                            "server.port",
		"cors":                            "server.cors_enabled",
		"db-host":                         "database.host",
		"db-port":                         "database.port",
		"db-user":                         "database.user",
		"db-password":                     "database.password",
		"db-name":                         "database.name",
		"db-ssl-mode":                     "database.ssl_mode",
		"db-conn-string":                  "database.conn_string",
		"max-nesting-depth":               "limits.max_nesting_depth",
		"max-string-length":               "limits.max_string_length",
		"max-message-content-length":      "limits.max_message_content",
		"max-total-content-size":          "limits.max_total_content_size",
		"dispatcher-heartbeat-interval":   "runtime.dispatcher_heartbeat_interval",
		"dispatcher-heartbeat-ttl":        "runtime.dispatcher_heartbeat_ttl",
		"dispatcher-stale-threshold":      "runtime.dispatcher_stale_threshold",
		"async-token-counter-workers":     "runtime.async_token_counter_workers",
		"async-token-counter-buffer-size": "runtime.async_token_counter_buffer_size",
	}

	// Convert flat CLI flags to nested structure
	config := make(map[string]any)

	for key, value := range c.flags {
		if path, ok := flagToPath[key]; ok {
			if err := setNested(config, path, value); err != nil {
				return nil, fmt.Errorf("failed to set CLI flag %s: %w", key, err)
			}
		}
		// Ignore unknown flags
	}

	return config, nil
}

// Watch is not implemented for CLI flags as they don't change at runtime.
func (c *cliProvider) Watch(_ context.Context, _ func()) error {
	// CLI flags don't support watching
	return nil
}

// Type returns the source type identifier.
func (c *cliProvider) Type() SourceType {
	return SourceCLI
}

// Close releases any resources held by the source.
func (c *cliProvider) Close() error {
	return nil
}

// setNested sets a value in a nested map structure using dot notation.
// It returns an error if a path conflict is encountered.
func setNested(m map[string]any, path string, value any) error {
	if path == "" {
		return nil // Don't set anything for empty path
	}

	parts := strings.Split(path, ".")
	current := m

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}

		next, ok := current[part].(map[string]any)
		if !ok {
			// Structure conflict, cannot set value
			return fmt.Errorf("configuration conflict: key %q is not a map", strings.Join(parts[:i+1], "."))
		}
		current = next
	}

	if len(parts) > 0 {
		current[parts[len(parts)-1]] = value
	}
	return nil
}

// yamlProvider implements Source interface for YAML files.
type yamlProvider struct {
	path       string
	watcher    *Watcher
	watcherMu  sync.Mutex
	isWatching bool
	watchOnce  sync.Once
	closeOnce  sync.Once
}

// NewYAMLProvider creates a new YAML file configuration source.
func NewYAMLProvider(path string) Source {
	return &yamlProvider{
		path: path,
	}
}

// Load reads configuration from a YAML file.
func (y *yamlProvider) Load() (map[string]any, error) {
	data, err := os.ReadFile(y.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty map when file doesn't exist to prevent overriding environment variables
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file: %w", err)
	}

	// Filter out nil values to prevent overriding existing configs
	// This ensures that missing sections in YAML don't reset environment variables
	filtered := filterNilValues(config)

	return filtered, nil
}

// filterNilValues recursively removes nil values from a map
// This prevents koanf from overriding existing values with nil
func filterNilValues(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		if v == nil {
			continue
		}
		// Recursively filter nested maps
		if nestedMap, ok := v.(map[string]any); ok {
			filtered := filterNilValues(nestedMap)
			// Only include non-empty maps
			if len(filtered) > 0 {
				result[k] = filtered
			}
		} else {
			result[k] = v
		}
	}
	return result
}

// Watch monitors the YAML file for changes.
func (y *yamlProvider) Watch(ctx context.Context, callback func()) error {
	var watchErr error

	// Use sync.Once to ensure we only create and start the watcher once
	y.watchOnce.Do(func() {
		y.watcherMu.Lock()
		defer y.watcherMu.Unlock()

		// Create a new watcher
		watcher, err := NewWatcher()
		if err != nil {
			watchErr = fmt.Errorf("failed to create watcher: %w", err)
			return
		}
		y.watcher = watcher

		// Start watching the file
		if err := y.watcher.Watch(ctx, y.path); err != nil {
			watchErr = fmt.Errorf("failed to watch YAML file: %w", err)
			return
		}
		y.isWatching = true
	})

	if watchErr != nil {
		return watchErr
	}

	// Register the callback (this can be called multiple times safely)
	y.watcherMu.Lock()
	defer y.watcherMu.Unlock()
	if y.watcher != nil {
		y.watcher.OnChange(callback)
	}

	return nil
}

// Type returns the source type identifier.
func (y *yamlProvider) Type() SourceType {
	return SourceYAML
}

// Close releases any resources held by the source.
func (y *yamlProvider) Close() error {
	var closeErr error

	// Use sync.Once to ensure we only close once
	y.closeOnce.Do(func() {
		y.watcherMu.Lock()
		defer y.watcherMu.Unlock()

		if y.watcher != nil {
			if err := y.watcher.Close(); err != nil {
				closeErr = fmt.Errorf("failed to close watcher: %w", err)
				return
			}
			y.watcher = nil
			y.isWatching = false
		}

		// Reset watchOnce to allow re-watching after close
		y.watchOnce = sync.Once{}
	})

	return closeErr
}

// defaultProvider implements Source interface for default configuration values.
type defaultProvider struct {
	defaults map[string]any
}

// NewDefaultProvider creates a new default configuration source.
func NewDefaultProvider() Source {
	return &defaultProvider{
		defaults: getDefaultConfig(),
	}
}

// Load returns the default configuration values.
func (d *defaultProvider) Load() (map[string]any, error) {
	return d.defaults, nil
}

// Watch is not implemented for defaults as they don't change at runtime.
func (d *defaultProvider) Watch(_ context.Context, _ func()) error {
	return nil
}

// Type returns the source type identifier.
func (d *defaultProvider) Type() SourceType {
	return SourceDefault
}

// Close releases any resources held by the source.
func (d *defaultProvider) Close() error {
	return nil
}

// getDefaultConfig returns the default configuration values.
func getDefaultConfig() map[string]any {
	return map[string]any{
		"server": map[string]any{
			"host":         "localhost",
			"port":         8080,
			"cors_enabled": false,
			"timeout":      "30s",
		},
		"database": map[string]any{
			"host":     "localhost",
			"port":     "5432",
			"user":     "postgres",
			"password": "",
			"name":     "compozy",
			"ssl_mode": "disable",
		},
		"temporal": map[string]any{
			"host_port":  "localhost:7233",
			"namespace":  "default",
			"task_queue": "compozy",
		},
		"runtime": map[string]any{
			"environment":                     "development",
			"log_level":                       "info",
			"dispatcher_heartbeat_interval":   "30s",
			"dispatcher_heartbeat_ttl":        "2m",
			"dispatcher_stale_threshold":      "5m",
			"async_token_counter_workers":     10,
			"async_token_counter_buffer_size": 1000,
		},
		"limits": map[string]any{
			"max_nesting_depth":        20,
			"max_string_length":        10485760,
			"max_message_content":      10240,
			"max_total_content_size":   102400,
			"max_task_context_depth":   5,
			"parent_update_batch_size": 100,
		},
		"openai": map[string]any{
			"api_key": "",
		},
		"memory": map[string]any{
			"url":      "",
			"api_key":  "",
			"org_id":   "",
			"agent_id": "",
		},
		"llm": map[string]any{
			"proxy_url":   "",
			"admin_token": "",
		},
	}
}
