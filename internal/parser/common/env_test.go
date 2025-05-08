package common

import (
	"os"
	"path/filepath"
	"testing"
)

// setupEnvFile creates a temporary .env file with the given content
func setupEnvFile(t *testing.T, content string) string {
	t.Helper()

	// Create temp directory
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	// Write content to file
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test env file: %v", err)
	}

	return envPath
}

func Test_FromFile(t *testing.T) {
	t.Run("Should load environment variables from file successfully", func(t *testing.T) {
		content := "KEY1=value1\nKEY2=value2"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		envPath := setupEnvFile(t, content)
		env := make(EnvMap)
		err := env.FromFile(envPath)

		if err != nil {
			t.Errorf("FromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("FromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})

	t.Run("Should handle empty file correctly", func(t *testing.T) {
		content := ""
		expected := EnvMap{}

		envPath := setupEnvFile(t, content)
		env := make(EnvMap)
		err := env.FromFile(envPath)

		if err != nil {
			t.Errorf("FromFile() error = %v, want nil", err)
			return
		}

		if len(env) != len(expected) {
			t.Errorf("FromFile() env length = %v, want %v", len(env), len(expected))
		}
	})

	t.Run("Should handle comments in file correctly", func(t *testing.T) {
		content := "# Comment\nKEY1=value1\n# Another comment\nKEY2=value2"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		envPath := setupEnvFile(t, content)
		env := make(EnvMap)
		err := env.FromFile(envPath)

		if err != nil {
			t.Errorf("FromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("FromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})

	t.Run("Should handle empty lines in file correctly", func(t *testing.T) {
		content := "\nKEY1=value1\n\nKEY2=value2\n"
		expected := EnvMap{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		envPath := setupEnvFile(t, content)
		env := make(EnvMap)
		err := env.FromFile(envPath)

		if err != nil {
			t.Errorf("FromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("FromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})
}

func Test_FromFileNotFound(t *testing.T) {
	t.Run("Should handle nonexistent file gracefully", func(t *testing.T) {
		env := make(EnvMap)
		err := env.FromFile("nonexistent.env")

		if err != nil {
			t.Errorf("FromFile() error = %v, want nil for nonexistent file", err)
		}

		if len(env) != 0 {
			t.Errorf("FromFile() env = %v, want empty map for nonexistent file", env)
		}
	})
}

func Test_LoadFromFile(t *testing.T) {
	t.Run("Should merge with existing values", func(t *testing.T) {
		initial := EnvMap{
			"EXISTING": "original",
		}
		content := "KEY1=value1\nKEY2=value2"
		expected := EnvMap{
			"EXISTING": "original",
			"KEY1":     "value1",
			"KEY2":     "value2",
		}

		envPath := setupEnvFile(t, content)
		env := initial
		err := env.LoadFromFile(envPath)

		if err != nil {
			t.Errorf("LoadFromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("LoadFromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})

	t.Run("Should override existing values", func(t *testing.T) {
		initial := EnvMap{
			"KEY1": "old",
		}
		content := "KEY1=new\nKEY2=value2"
		expected := EnvMap{
			"KEY1": "new",
			"KEY2": "value2",
		}

		envPath := setupEnvFile(t, content)
		env := initial
		err := env.LoadFromFile(envPath)

		if err != nil {
			t.Errorf("LoadFromFile() error = %v, want nil", err)
			return
		}

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("LoadFromFile() env[%s] = %v, want %v", k, got, v)
			}
		}
	})
}

func Test_LoadFromFileNotFound(t *testing.T) {
	t.Run("Should preserve existing values when file not found", func(t *testing.T) {
		env := EnvMap{"EXISTING": "original"}
		err := env.LoadFromFile("nonexistent.env")

		if err != nil {
			t.Errorf("LoadFromFile() error = %v, want nil for nonexistent file", err)
		}

		if env["EXISTING"] != "original" {
			t.Errorf("LoadFromFile() env = %v, want to preserve existing values", env)
		}
	})
}

func Test_Merge(t *testing.T) {
	t.Run("Should merge with empty map", func(t *testing.T) {
		initial := EnvMap{
			"KEY1": "value1",
		}
		other := EnvMap{}
		expected := EnvMap{
			"KEY1": "value1",
		}

		env := initial
		env.Merge(other)

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
		env.Merge(other)

		for k, v := range expected {
			if got := env[k]; got != v {
				t.Errorf("Merge() env[%s] = %v, want %v", k, got, v)
			}
		}
	})
}
