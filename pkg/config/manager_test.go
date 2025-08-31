package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Creation(t *testing.T) {
	t.Run("Should create manager with default service", func(t *testing.T) {
		manager := NewManager(nil)
		require.NotNil(t, manager)
		require.NotNil(t, manager.Service)
		assert.Equal(t, 100*time.Millisecond, manager.debounce)
		require.NoError(t, manager.Close(context.Background()))
	})

	t.Run("Should create manager with custom service", func(t *testing.T) {
		service := NewService()
		manager := NewManager(service)
		require.NotNil(t, manager)
		assert.Equal(t, service, manager.Service)
		require.NoError(t, manager.Close(context.Background()))
	})

	t.Run("Should configure debounce duration", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		// Set custom debounce
		manager.SetDebounce(500 * time.Millisecond)
		assert.Equal(t, 500*time.Millisecond, manager.debounce)
	})
}

func TestManager_Load(t *testing.T) {
	t.Run("Should load configuration from sources", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		// Load with default provider
		ctx := context.Background()
		config, err := manager.Load(ctx, NewDefaultProvider())

		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, "0.0.0.0", config.Server.Host)
		assert.Equal(t, 5001, config.Server.Port)
	})

	t.Run("Should store configuration atomically", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		// Initially nil
		assert.Nil(t, manager.Get())

		// Load configuration
		ctx := context.Background()
		config, err := manager.Load(ctx, NewDefaultProvider())
		require.NoError(t, err)

		// Should be accessible via Get
		stored := manager.Get()
		assert.Equal(t, config, stored)
	})

	t.Run("Should handle multiple sources with precedence", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		// Create temp YAML file
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, "config.yaml")
		yamlContent := `
server:
  host: yaml.example.com
  port: 9090
`
		err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644)
		require.NoError(t, err)

		// Load with multiple sources - YAML should override defaults
		ctx := context.Background()
		config, err := manager.Load(ctx,
			NewDefaultProvider(),
			NewYAMLProvider(yamlPath),
		)

		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, "yaml.example.com", config.Server.Host)
		assert.Equal(t, 9090, config.Server.Port)
	})
}

func TestManager_Get(t *testing.T) {
	t.Run("Should return nil before loading", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		assert.Nil(t, manager.Get())
	})

	t.Run("Should return configuration after loading", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		ctx := context.Background()
		loaded, err := manager.Load(ctx, NewDefaultProvider())
		require.NoError(t, err)

		retrieved := manager.Get()
		assert.Equal(t, loaded, retrieved)
	})

	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		ctx := context.Background()
		_, err := manager.Load(ctx, NewDefaultProvider())
		require.NoError(t, err)

		// Concurrent reads
		var wg sync.WaitGroup
		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				config := manager.Get()
				assert.NotNil(t, config)
			}()
		}
		wg.Wait()
	})
}

func TestManager_Reload(t *testing.T) {
	t.Run("Should reload configuration", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		// Initial load
		ctx := context.Background()
		_, err := manager.Load(ctx, NewDefaultProvider())
		require.NoError(t, err)

		// Reload
		err = manager.Reload(ctx)
		assert.NoError(t, err)

		// Configuration should still be valid
		config := manager.Get()
		assert.NotNil(t, config)
	})

	t.Run("Should validate configuration before applying", func(t *testing.T) {
		// Create a mock service that returns invalid config on reload
		service := &mockService{
			loadFunc: func(_ context.Context, _ ...Source) (*Config, error) {
				// Return config with invalid values
				return &Config{
					Server: ServerConfig{
						Host: "", // Invalid - required field
					},
				}, nil
			},
			validateFunc: func(config *Config) error {
				if config.Server.Host == "" {
					return assert.AnError
				}
				return nil
			},
		}

		manager := NewManager(service)
		defer manager.Close(context.Background())

		// Reload should fail validation
		ctx := context.Background()
		err := manager.Reload(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration validation failed")
	})
}

func TestManager_OnChange(t *testing.T) {
	t.Run("Should register and invoke callbacks", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		// Register callback
		var callbackConfig *Config
		manager.OnChange(func(config *Config) {
			callbackConfig = config
		})

		// Load configuration - should trigger callback
		ctx := context.Background()
		loaded, err := manager.Load(ctx, NewDefaultProvider())
		require.NoError(t, err)

		// Callback should have been invoked
		assert.Equal(t, loaded, callbackConfig)
	})

	t.Run("Should handle multiple callbacks", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close(context.Background())

		// Register multiple callbacks
		var count int32
		for range 3 {
			manager.OnChange(func(_ *Config) {
				atomic.AddInt32(&count, 1)
			})
		}

		// Load configuration
		ctx := context.Background()
		_, err := manager.Load(ctx, NewDefaultProvider())
		require.NoError(t, err)

		// All callbacks should be invoked
		assert.Equal(t, int32(3), atomic.LoadInt32(&count))
	})
}

func TestManager_WatchIntegration(t *testing.T) {
	t.Run("Should reload on file change", func(t *testing.T) {
		// Create temp YAML file
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, "config.yaml")
		initialContent := `
server:
  host: initial.example.com
  port: 5001
`
		err := os.WriteFile(yamlPath, []byte(initialContent), 0o644)
		require.NoError(t, err)

		manager := NewManager(nil)
		// Reduce debounce to make test faster and more reliable
		manager.SetDebounce(10 * time.Millisecond)
		defer manager.Close(context.Background())

		// Track reloads
		var reloadCount int32
		manager.OnChange(func(_ *Config) {
			atomic.AddInt32(&reloadCount, 1)
		})

		// Load with YAML provider
		ctx := context.Background()
		config, err := manager.Load(ctx, NewYAMLProvider(yamlPath))
		require.NoError(t, err)
		assert.Equal(t, "initial.example.com", config.Server.Host)

		// Wait for watcher to start by giving it time to initialize
		// fsnotify needs time to set up file watching on the OS level
		time.Sleep(200 * time.Millisecond)

		// Modify file using non-atomic write to ensure fsnotify detects the change
		// os.WriteFile uses atomic writes on macOS which can bypass fsnotify
		updatedContent := `
server:
  host: updated.example.com
  port: 9090
`
		file, err := os.OpenFile(yamlPath, os.O_WRONLY|os.O_TRUNC, 0o644)
		require.NoError(t, err)
		_, err = file.WriteString(updatedContent)
		require.NoError(t, err)
		// Ensure data hits disk before close so fsnotify reliably observes the change
		err = file.Sync()
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		// Give fsnotify time to process the file change event
		time.Sleep(100 * time.Millisecond)

		// Wait for configuration to be reloaded
		require.Eventually(t, func() bool {
			newConfig := manager.Get()
			return newConfig.Server.Host == "updated.example.com"
		}, 2*time.Second, 50*time.Millisecond, "configuration reload timeout")

		// Verify configuration was reloaded correctly
		newConfig := manager.Get()
		assert.Equal(t, "updated.example.com", newConfig.Server.Host)
		assert.Equal(t, 9090, newConfig.Server.Port)

		// Callback should have been invoked (initial load + reload)
		assert.GreaterOrEqual(t, atomic.LoadInt32(&reloadCount), int32(2))
	})
}

func TestManager_Close(t *testing.T) {
	t.Run("Should close gracefully", func(t *testing.T) {
		manager := NewManager(nil)

		// Start watching
		ctx := context.Background()
		_, err := manager.Load(ctx, NewDefaultProvider())
		require.NoError(t, err)

		// Close should not hang
		done := make(chan bool)
		go func() {
			err := manager.Close(context.Background())
			assert.NoError(t, err)
			done <- true
		}()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for close")
		}
	})
}

// mockService implements Service interface for testing
type mockService struct {
	loadFunc      func(context.Context, ...Source) (*Config, error)
	watchFunc     func(context.Context, func(*Config)) error
	validateFunc  func(*Config) error
	getSourceFunc func(string) SourceType
}

func (m *mockService) Load(ctx context.Context, sources ...Source) (*Config, error) {
	if m.loadFunc != nil {
		return m.loadFunc(ctx, sources...)
	}
	return Default(), nil
}

func (m *mockService) Watch(ctx context.Context, callback func(*Config)) error {
	if m.watchFunc != nil {
		return m.watchFunc(ctx, callback)
	}
	return nil
}

func (m *mockService) Validate(config *Config) error {
	if m.validateFunc != nil {
		return m.validateFunc(config)
	}
	return nil
}

func (m *mockService) GetSource(key string) SourceType {
	if m.getSourceFunc != nil {
		return m.getSourceFunc(key)
	}
	return SourceDefault
}

func TestConfigEqual(t *testing.T) {
	t.Run("Should return true for identical configurations", func(t *testing.T) {
		config1 := &Config{
			Server: ServerConfig{
				Host:        "localhost",
				Port:        5001,
				CORSEnabled: true,
				Timeout:     30 * time.Second,
			},
			Database: DatabaseConfig{
				Host:   "db.example.com",
				Port:   "5432",
				User:   "testuser",
				DBName: "testdb",
			},
		}

		config2 := &Config{
			Server: ServerConfig{
				Host:        "localhost",
				Port:        5001,
				CORSEnabled: true,
				Timeout:     30 * time.Second,
			},
			Database: DatabaseConfig{
				Host:   "db.example.com",
				Port:   "5432",
				User:   "testuser",
				DBName: "testdb",
			},
		}

		assert.True(t, configEqual(config1, config2))
	})

	t.Run("Should return false for different configurations", func(t *testing.T) {
		config1 := &Config{
			Server: ServerConfig{
				Host: "localhost",
				Port: 5001,
			},
		}

		config2 := &Config{
			Server: ServerConfig{
				Host: "different.host.com",
				Port: 5001,
			},
		}

		assert.False(t, configEqual(config1, config2))
	})

	t.Run("Should handle nil configurations", func(t *testing.T) {
		config := &Config{}

		assert.True(t, configEqual(nil, nil))
		assert.False(t, configEqual(config, nil))
		assert.False(t, configEqual(nil, config))
	})

	t.Run("Should detect database configuration differences", func(t *testing.T) {
		config1 := &Config{
			Database: DatabaseConfig{
				Host: "db1.example.com",
				Port: "5432",
			},
		}

		config2 := &Config{
			Database: DatabaseConfig{
				Host: "db2.example.com",
				Port: "5432",
			},
		}

		assert.False(t, configEqual(config1, config2))
	})
}
