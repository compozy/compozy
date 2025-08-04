package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupEnvFile creates a temporary .env file with the given content
func setupEnvFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	// Write content to file
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		require.NoError(t, err, "Failed to create test env file")
	}

	return tmpDir // Return the directory containing .env
}

func Test_NewEnvFromFile(t *testing.T) {
	t.Run("Should load environment variables from file successfully", func(t *testing.T) {
		content := "KEY1=value1\nKEY2=value2"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		cwd := setupEnvFile(t, content)
		env, err := NewEnvFromFile(cwd, "")
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})

	t.Run("Should handle empty file correctly", func(t *testing.T) {
		content := ""
		expected := EnvMap{}

		cwd := setupEnvFile(t, content)
		env, err := NewEnvFromFile(cwd, "")
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})

	t.Run("Should handle comments in file correctly", func(t *testing.T) {
		content := "# Comment\nKEY1=value1\n# Another comment\nKEY2=value2"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		cwd := setupEnvFile(t, content)
		env, err := NewEnvFromFile(cwd, "")
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})

	t.Run("Should handle empty lines in file correctly", func(t *testing.T) {
		content := "\nKEY1=value1\n\nKEY2=value2\n"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		cwd := setupEnvFile(t, content)
		env, err := NewEnvFromFile(cwd, "")
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})

	t.Run("Should handle nonexistent file gracefully", func(t *testing.T) {
		// Create a temporary directory without an .env file
		tmpDir := t.TempDir()
		env, err := NewEnvFromFile(tmpDir, "")
		require.NoError(t, err)
		assert.Empty(t, env)
	})
}

func Test_Merge(t *testing.T) {
	t.Run("Should merge with empty map on dst", func(t *testing.T) {
		initial := EnvMap{}
		other := EnvMap{"KEY1": "value1"}
		expected := EnvMap{"KEY1": "value1"}
		env := initial
		env, err := env.Merge(other)
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})

	t.Run("Should merge with empty map on src", func(t *testing.T) {
		initial := EnvMap{"KEY1": "value1"}
		other := EnvMap{}
		expected := EnvMap{"KEY1": "value1"}

		env := initial
		env, err := env.Merge(other)
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})

	t.Run("Should merge and override values", func(t *testing.T) {
		initial := EnvMap{
			"KEY1": "value1",
			"KEY2": "old",
		}
		other := EnvMap{
			"KEY2": "new",
			"KEY3": "value3",
		}
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "new",
			"KEY3": "value3",
		}

		env := initial
		env, err := env.Merge(other)
		require.NoError(t, err)
		assert.Equal(t, expected, env)
	})
}

func Test_EnvMap_Prop(t *testing.T) {
	t.Run("Should return value for existing key", func(t *testing.T) {
		env := EnvMap{"KEY1": "value1", "KEY2": "value2"}
		result := env.Prop("KEY1")
		assert.Equal(t, "value1", result)
	})

	t.Run("Should return empty string for non-existing key", func(t *testing.T) {
		env := EnvMap{"KEY1": "value1"}
		result := env.Prop("NONEXISTENT")
		assert.Equal(t, "", result)
	})

	t.Run("Should return empty string for nil EnvMap", func(t *testing.T) {
		var env EnvMap
		result := env.Prop("KEY1")
		assert.Equal(t, "", result)
	})
}

func Test_EnvMap_Set(t *testing.T) {
	t.Run("Should set key-value pair in EnvMap", func(t *testing.T) {
		env := make(EnvMap)
		env.Set("KEY1", "value1")
		assert.Equal(t, "value1", env["KEY1"])
	})

	t.Run("Should override existing key", func(t *testing.T) {
		env := EnvMap{"KEY1": "old_value"}
		env.Set("KEY1", "new_value")
		assert.Equal(t, "new_value", env["KEY1"])
	})

	t.Run("Should handle nil EnvMap gracefully", func(t *testing.T) {
		var env *EnvMap
		assert.NotPanics(t, func() {
			env.Set("KEY1", "value1")
		})
	})
}

func Test_EnvMap_AsMap(t *testing.T) {
	t.Run("Should convert EnvMap to map[string]any", func(t *testing.T) {
		env := EnvMap{"KEY1": "value1", "KEY2": "value2"}
		result := env.AsMap()
		expected := map[string]any{"KEY1": "value1", "KEY2": "value2"}
		assert.Equal(t, expected, result)
	})

	t.Run("Should return empty map for nil EnvMap", func(t *testing.T) {
		var env *EnvMap
		result := env.AsMap()
		assert.Equal(t, map[string]any{}, result)
	})
}

func Test_EnvMerger_Merge(t *testing.T) {
	t.Run("Should merge multiple environment maps", func(t *testing.T) {
		merger := &EnvMerger{}
		env1 := EnvMap{"KEY1": "value1"}
		env2 := EnvMap{"KEY2": "value2"}
		env3 := EnvMap{"KEY1": "overridden", "KEY3": "value3"}

		result, err := merger.Merge(env1, env2, env3)
		require.NoError(t, err)

		expected := EnvMap{
			"KEY1": "overridden",
			"KEY2": "value2",
			"KEY3": "value3",
		}
		assert.Equal(t, expected, result)
	})

	t.Run("Should handle empty input", func(t *testing.T) {
		merger := &EnvMerger{}
		result, err := merger.Merge()
		require.NoError(t, err)
		assert.Equal(t, EnvMap{}, result)
	})

	t.Run("Should skip nil maps", func(t *testing.T) {
		merger := &EnvMerger{}
		env1 := EnvMap{"KEY1": "value1"}
		result, err := merger.Merge(env1, nil, EnvMap{"KEY2": "value2"})
		require.NoError(t, err)

		expected := EnvMap{"KEY1": "value1", "KEY2": "value2"}
		assert.Equal(t, expected, result)
	})
}

func Test_EnvMerger_MergeWithDefaults(t *testing.T) {
	t.Run("Should merge environments with nil handling", func(t *testing.T) {
		merger := &EnvMerger{}
		env1 := EnvMap{"KEY1": "value1"}
		result, err := merger.MergeWithDefaults(env1, nil, EnvMap{"KEY2": "value2"})
		require.NoError(t, err)

		expected := EnvMap{"KEY1": "value1", "KEY2": "value2"}
		assert.Equal(t, expected, result)
	})

	t.Run("Should handle all nil environments", func(t *testing.T) {
		merger := &EnvMerger{}
		result, err := merger.MergeWithDefaults(nil, nil)
		require.NoError(t, err)
		assert.Equal(t, EnvMap{}, result)
	})
}
