package core

import (
	"os"
	"path/filepath"
	"testing"
)

// setupEnvFile creates a temporary .env file with the given content
func setupEnvFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	// Write content to file
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test env file: %v", err)
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
		env, err := NewEnvFromFile(cwd)
		if err != nil {
			t.Errorf("NewEnvFromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("NewEnvFromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})

	t.Run("Should handle empty file correctly", func(t *testing.T) {
		content := ""
		expected := EnvMap{}

		cwd := setupEnvFile(t, content)
		env, err := NewEnvFromFile(cwd)
		if err != nil {
			t.Errorf("NewEnvFromFile() error = %v, want nil", err)
			return
		}

		if len(env) != len(expected) {
			t.Errorf("NewEnvFromFile() env length = %v, want %v", len(env), len(expected))
		}
	})

	t.Run("Should handle comments in file correctly", func(t *testing.T) {
		content := "# Comment\nKEY1=value1\n# Another comment\nKEY2=value2"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		cwd := setupEnvFile(t, content)
		env, err := NewEnvFromFile(cwd)
		if err != nil {
			t.Errorf("NewEnvFromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("NewEnvFromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})

	t.Run("Should handle empty lines in file correctly", func(t *testing.T) {
		content := "\nKEY1=value1\n\nKEY2=value2\n"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		cwd := setupEnvFile(t, content)
		env, err := NewEnvFromFile(cwd)
		if err != nil {
			t.Errorf("NewEnvFromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("NewEnvFromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})

	t.Run("Should handle nonexistent file gracefully", func(t *testing.T) {
		// Create a temporary directory without an .env file
		tmpDir := t.TempDir()
		env, err := NewEnvFromFile(tmpDir)
		if err != nil {
			t.Errorf("NewEnvFromFile() error = %v, want nil for nonexistent file", err)
		}

		if len(env) != 0 {
			t.Errorf("NewEnvFromFile() env = %v, want empty map for nonexistent file", env)
		}
	})
}

func Test_Merge(t *testing.T) {
	t.Run("Should merge with empty map on dst", func(t *testing.T) {
		initial := EnvMap{}
		other := EnvMap{"KEY1": "value1"}
		expected := EnvMap{"KEY1": "value1"}
		env := initial
		env, err := env.Merge(other)
		if err != nil {
			t.Errorf("Merge() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("Merge() env[%s] = %v, want %v", k, got, v)
			}
		}
	})

	t.Run("Should merge with empty map on src", func(t *testing.T) {
		initial := EnvMap{"KEY1": "value1"}
		other := EnvMap{}
		expected := EnvMap{"KEY1": "value1"}

		env := initial
		env, err := env.Merge(other)
		if err != nil {
			t.Errorf("Merge() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("Merge() env[%s] = %v, want %v", k, got, v)
			}
		}
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
		if err != nil {
			t.Errorf("Merge() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("Merge() env[%s] = %v, want %v", k, got, v)
			}
		}
	})
}
