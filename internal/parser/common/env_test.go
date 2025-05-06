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

func TestFromFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected EnvMap
		wantErr  bool
	}{
		{
			name:    "successful load",
			content: "KEY1=value1\nKEY2=value2",
			expected: EnvMap{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:     "empty file",
			content:  "",
			expected: EnvMap{},
			wantErr:  false,
		},
		{
			name:    "with comments",
			content: "# Comment\nKEY1=value1\n# Another comment\nKEY2=value2",
			expected: EnvMap{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
		{
			name:    "with empty lines",
			content: "\nKEY1=value1\n\nKEY2=value2\n",
			expected: EnvMap{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envPath := setupEnvFile(t, tt.content)

			env := make(EnvMap)
			err := env.FromFile(envPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("FromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for k, v := range tt.expected {
					if got := env[k]; got != v {
						t.Errorf("FromFile() env[%s] = %v, want %v", k, got, v)
					}
				}
			}
		})
	}
}

func TestFromFileNotFound(t *testing.T) {
	env := make(EnvMap)
	err := env.FromFile("nonexistent.env")

	if err != nil {
		t.Errorf("FromFile() error = %v, want nil for nonexistent file", err)
	}

	if len(env) != 0 {
		t.Errorf("FromFile() env = %v, want empty map for nonexistent file", env)
	}
}

func TestLoadFromFile(t *testing.T) {
	tests := []struct {
		name     string
		initial  EnvMap
		content  string
		expected EnvMap
		wantErr  bool
	}{
		{
			name: "merge with existing values",
			initial: EnvMap{
				"EXISTING": "original",
			},
			content: "KEY1=value1\nKEY2=value2",
			expected: EnvMap{
				"EXISTING": "original",
				"KEY1":     "value1",
				"KEY2":     "value2",
			},
			wantErr: false,
		},
		{
			name: "override existing values",
			initial: EnvMap{
				"KEY1": "old",
			},
			content: "KEY1=new\nKEY2=value2",
			expected: EnvMap{
				"KEY1": "new",
				"KEY2": "value2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envPath := setupEnvFile(t, tt.content)

			env := tt.initial
			err := env.LoadFromFile(envPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for k, v := range tt.expected {
					if got := env[k]; got != v {
						t.Errorf("LoadFromFile() env[%s] = %v, want %v", k, got, v)
					}
				}
			}
		})
	}
}

func TestLoadFromFileNotFound(t *testing.T) {
	env := EnvMap{"EXISTING": "original"}
	err := env.LoadFromFile("nonexistent.env")

	if err != nil {
		t.Errorf("LoadFromFile() error = %v, want nil for nonexistent file", err)
	}

	if env["EXISTING"] != "original" {
		t.Errorf("LoadFromFile() env = %v, want to preserve existing values", env)
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		initial  EnvMap
		other    EnvMap
		expected EnvMap
	}{
		{
			name: "merge with empty",
			initial: EnvMap{
				"KEY1": "value1",
			},
			other: EnvMap{},
			expected: EnvMap{
				"KEY1": "value1",
			},
		},
		{
			name: "merge with values",
			initial: EnvMap{
				"KEY1": "value1",
				"KEY2": "old",
			},
			other: EnvMap{
				"KEY2": "new",
				"KEY3": "value3",
			},
			expected: EnvMap{
				"KEY1": "value1",
				"KEY2": "new",
				"KEY3": "value3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := tt.initial
			env.Merge(tt.other)

			for k, v := range tt.expected {
				if got := env[k]; got != v {
					t.Errorf("Merge() env[%s] = %v, want %v", k, got, v)
				}
			}
		})
	}
}
