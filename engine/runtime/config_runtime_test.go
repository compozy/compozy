package runtime_test

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/runtime"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestFromAppConfig(t *testing.T) {
	t.Run("Should create runtime config from app config", func(t *testing.T) {
		appCfg := &appconfig.RuntimeConfig{
			Environment:          "production",
			ToolExecutionTimeout: 30 * time.Second,
		}

		config := runtime.FromAppConfig(appCfg)

		assert.Equal(t, "production", config.Environment)
		assert.Equal(t, 30*time.Second, config.ToolExecutionTimeout)
		// Should preserve defaults for other fields
		assert.Equal(t, runtime.RuntimeTypeBun, config.RuntimeType)
		assert.NotEmpty(t, config.BunPermissions)
	})

	t.Run("Should return default config when app config is nil", func(t *testing.T) {
		config := runtime.FromAppConfig(nil)
		defaultConfig := runtime.DefaultConfig()

		assert.Equal(t, defaultConfig.Environment, config.Environment)
		assert.Equal(t, defaultConfig.ToolExecutionTimeout, config.ToolExecutionTimeout)
		assert.Equal(t, defaultConfig.RuntimeType, config.RuntimeType)
	})

	t.Run("Should use defaults for missing fields in app config", func(t *testing.T) {
		appCfg := &appconfig.RuntimeConfig{
			Environment: "staging",
			// ToolExecutionTimeout not set - should use default
		}

		config := runtime.FromAppConfig(appCfg)

		assert.Equal(t, "staging", config.Environment)
		assert.Equal(t, runtime.DefaultConfig().ToolExecutionTimeout, config.ToolExecutionTimeout)
	})

	t.Run("Should ignore zero timeout in app config", func(t *testing.T) {
		appCfg := &appconfig.RuntimeConfig{
			Environment:          "test",
			ToolExecutionTimeout: 0, // Should be ignored
		}

		config := runtime.FromAppConfig(appCfg)

		assert.Equal(t, "test", config.Environment)
		assert.Equal(t, runtime.DefaultConfig().ToolExecutionTimeout, config.ToolExecutionTimeout)
	})
}

func TestDirectConfigUsage(t *testing.T) {
	t.Run("Should create config with direct field assignment", func(t *testing.T) {
		config := &runtime.Config{
			RuntimeType:          runtime.RuntimeTypeNode,
			EntrypointPath:       "/custom/path.js",
			BunPermissions:       []string{"--allow-all"},
			NodeOptions:          []string{"--experimental-modules"},
			Environment:          "production",
			ToolExecutionTimeout: 45 * time.Second,
		}

		assert.Equal(t, runtime.RuntimeTypeNode, config.RuntimeType)
		assert.Equal(t, "/custom/path.js", config.EntrypointPath)
		assert.Equal(t, []string{"--allow-all"}, config.BunPermissions)
		assert.Equal(t, []string{"--experimental-modules"}, config.NodeOptions)
		assert.Equal(t, "production", config.Environment)
		assert.Equal(t, 45*time.Second, config.ToolExecutionTimeout)
	})
}

func TestDefaultConfigRuntimeFields(t *testing.T) {
	t.Run("Should have Bun as default runtime type", func(t *testing.T) {
		config := runtime.DefaultConfig()

		assert.Equal(t, runtime.RuntimeTypeBun, config.RuntimeType)
		assert.NotEmpty(t, config.BunPermissions)
		assert.Contains(t, config.BunPermissions, "--allow-read")
		assert.Equal(t, "development", config.Environment)
	})
}

func TestTestConfigRuntimeFields(t *testing.T) {
	t.Run("Should have Bun as default runtime type for tests", func(t *testing.T) {
		config := runtime.TestConfig()

		assert.Equal(t, runtime.RuntimeTypeBun, config.RuntimeType)
		assert.NotEmpty(t, config.BunPermissions)
		assert.Contains(t, config.BunPermissions, "--allow-read")
		assert.Equal(t, "testing", config.Environment)
	})
}
