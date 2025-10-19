package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvProvider_Load(t *testing.T) {
	t.Run("Should return empty map as loading is handled by koanf", func(t *testing.T) {
		// Arrange
		provider := NewEnvProvider()

		// Act
		data, err := provider.Load()

		// Assert
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data)
	})
}

func TestEnvProvider_Type(t *testing.T) {
	t.Run("Should return SourceEnv", func(t *testing.T) {
		provider := NewEnvProvider()
		assert.Equal(t, SourceEnv, provider.Type())
	})
}

func TestEnvProvider_Watch(t *testing.T) {
	t.Run("Should return nil for Watch", func(t *testing.T) {
		provider := NewEnvProvider()
		err := provider.Watch(t.Context(), func() {})
		assert.NoError(t, err)
	})
}

func TestCLIProvider_Load(t *testing.T) {
	t.Run("Should map CLI flags to configuration structure", func(t *testing.T) {
		// Arrange
		flags := map[string]any{
			"host":                          "cli.example.com",
			"port":                          6001,
			"cors":                          true,
			"max-nesting-depth":             20,
			"max-string-length":             2048,
			"max-message-content-length":    8192,
			"dispatcher-heartbeat-interval": 45,
			"async-token-counter-workers":   8,
		}
		provider := NewCLIProvider(flags)

		// Act
		data, err := provider.Load()

		// Assert
		require.NoError(t, err)
		require.NotNil(t, data)

		// Check server mapping
		server, ok := data["server"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "cli.example.com", server["host"])
		assert.Equal(t, 6001, server["port"])
		assert.Equal(t, true, server["cors_enabled"])

		// Check limits mapping
		limits, ok := data["limits"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 20, limits["max_nesting_depth"])
		assert.Equal(t, 2048, limits["max_string_length"])
		assert.Equal(t, 8192, limits["max_message_content"])

		// Check runtime mapping
		runtime, ok := data["runtime"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 45, runtime["dispatcher_heartbeat_interval"])
		assert.Equal(t, 8, runtime["async_token_counter_workers"])
	})

	t.Run("Should handle nil flags gracefully", func(t *testing.T) {
		// Arrange
		provider := NewCLIProvider(nil)

		// Act
		data, err := provider.Load()

		// Assert
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data)
	})

	t.Run("Should handle empty flags map", func(t *testing.T) {
		// Arrange
		provider := NewCLIProvider(map[string]any{})

		// Act
		data, err := provider.Load()

		// Assert
		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data)
	})
}

func TestCLIProvider_Type(t *testing.T) {
	t.Run("Should return SourceCLI", func(t *testing.T) {
		provider := NewCLIProvider(nil)
		assert.Equal(t, SourceCLI, provider.Type())
	})
}

func TestCLIProvider_Watch(t *testing.T) {
	t.Run("Should return nil for Watch", func(t *testing.T) {
		provider := NewCLIProvider(nil)
		err := provider.Watch(t.Context(), func() {})
		assert.NoError(t, err)
	})
}

func TestYAMLProvider_Load(t *testing.T) {
	t.Run("Should return empty map for non-existent file", func(t *testing.T) {
		// Arrange
		provider := NewYAMLProvider("/non/existent/config.yaml")

		// Act
		data, err := provider.Load()

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, data)
		assert.Empty(t, data)
	})
}

func TestYAMLProvider_Type(t *testing.T) {
	t.Run("Should return SourceYAML", func(t *testing.T) {
		provider := NewYAMLProvider("config.yaml")
		assert.Equal(t, SourceYAML, provider.Type())
	})
}

func TestYAMLProvider_Watch(t *testing.T) {
	t.Run("Should setup watcher without error", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		provider := NewYAMLProvider(tmpFile.Name())
		ctx := t.Context()

		err = provider.Watch(ctx, func() {})
		assert.NoError(t, err)
	})
}

func TestSetNested(t *testing.T) {
	t.Run("Should set value in nested map structure", func(t *testing.T) {
		// Arrange
		m := make(map[string]any)

		// Act
		err1 := setNested(m, "server.host", "test.example.com")
		err2 := setNested(m, "server.port", 5001)
		err3 := setNested(m, "database.connection.host", "db.example.com")

		// Assert
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)

		server, ok := m["server"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test.example.com", server["host"])
		assert.Equal(t, 5001, server["port"])

		database, ok := m["database"].(map[string]any)
		require.True(t, ok)
		connection, ok := database["connection"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "db.example.com", connection["host"])
	})

	t.Run("Should return error on structure conflicts", func(t *testing.T) {
		// Arrange
		m := map[string]any{
			"server": "not-a-map", // Structure conflict
		}

		// Act
		err := setNested(m, "server.host", "should-not-be-set")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration conflict: key \"server\" is not a map")
		// Original value should remain unchanged
		assert.Equal(t, "not-a-map", m["server"])
	})

	t.Run("Should handle empty path", func(t *testing.T) {
		// Arrange
		m := make(map[string]any)

		// Act
		err := setNested(m, "", "value")

		// Assert
		assert.NoError(t, err)
		assert.Empty(t, m)
	})
}

func TestYAMLProvider_LoadActual(t *testing.T) {
	t.Run("Should load configuration from YAML file", func(t *testing.T) {
		// Create temp YAML file
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, "config.yaml")
		yamlContent := `
server:
  host: yaml.example.com
  port: 9090
  cors_enabled: true
database:
  host: db.yaml.com
  user: testuser
`
		err := os.WriteFile(yamlPath, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Create provider and load
		provider := NewYAMLProvider(yamlPath)
		data, err := provider.Load()

		// Assert
		require.NoError(t, err)
		require.NotNil(t, data)

		// Check server config
		server, ok := data["server"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "yaml.example.com", server["host"])
		assert.Equal(t, 9090, server["port"])
		assert.Equal(t, true, server["cors_enabled"])

		// Check database config
		db, ok := data["database"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "db.yaml.com", db["host"])
		assert.Equal(t, "testuser", db["user"])
	})

	t.Run("Should return empty config for non-existent file", func(t *testing.T) {
		provider := NewYAMLProvider("/non/existent/path.yaml")
		data, err := provider.Load()

		require.NoError(t, err)
		require.NotNil(t, data)
		assert.Empty(t, data)
	})

	t.Run("Should return error for invalid YAML", func(t *testing.T) {
		// Create temp file with invalid YAML
		tmpFile, err := os.CreateTemp("", "invalid-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString("invalid: yaml: content: [")
		require.NoError(t, err)
		tmpFile.Close()

		provider := NewYAMLProvider(tmpFile.Name())
		data, err := provider.Load()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse YAML file")
		assert.Nil(t, data)
	})
}

func TestYAMLProvider_WatchActual(t *testing.T) {
	t.Run("Should watch YAML file for changes", func(t *testing.T) {
		// Create temp YAML file
		tmpFile, err := os.CreateTemp("", "watch-test-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		// Create provider
		provider := NewYAMLProvider(tmpFile.Name())

		// Setup watch
		ctx := t.Context()

		// Use channel to safely track callback invocation
		callbackCh := make(chan struct{}, 1)
		err = provider.Watch(ctx, func() {
			select {
			case callbackCh <- struct{}{}:
			default:
			}
		})
		require.NoError(t, err)

		// Give watcher time to start
		time.Sleep(100 * time.Millisecond)

		// Modify file
		err = os.WriteFile(tmpFile.Name(), []byte("test: value"), 0644)
		require.NoError(t, err)

		// Wait for callback
		select {
		case <-callbackCh:
			// Success - callback was invoked
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for callback")
		}
	})
}

func TestDefaultProvider(t *testing.T) {
	t.Run("Should load default configuration", func(t *testing.T) {
		provider := NewDefaultProvider()
		data, err := provider.Load()

		require.NoError(t, err)
		require.NotNil(t, data)

		// Check structure exists
		server, ok := data["server"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "0.0.0.0", server["host"])
		assert.Equal(t, 5001, server["port"])
		assert.Equal(t, true, server["cors_enabled"])

		database, ok := data["database"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "localhost", database["host"])
		assert.Equal(t, "5432", database["port"])

		temporal, ok := data["temporal"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "localhost:7233", temporal["host_port"])

		runtime, ok := data["runtime"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "development", runtime["environment"])

		limits, ok := data["limits"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 20, limits["max_nesting_depth"])
	})

	t.Run("Should return SourceDefault type", func(t *testing.T) {
		provider := NewDefaultProvider()
		assert.Equal(t, SourceDefault, provider.Type())
	})

	t.Run("Should not support watching", func(t *testing.T) {
		provider := NewDefaultProvider()
		err := provider.Watch(t.Context(), func() {})
		assert.NoError(t, err)
	})
}
