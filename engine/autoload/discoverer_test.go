package autoload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileDiscoverer_Discover(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"workflows/user.yaml",
		"workflows/admin.yaml",
		"workflows/test/test.yaml",
		"tasks/email.yaml",
		"tasks/webhook.yaml",
		"tasks/.#temp.yaml", // Emacs temp file
		"agents/chatbot.yaml",
		"agents/reviewer.yaml~", // Backup file
		"tools/format.yaml",
		"tools/validate.yaml.bak", // Backup file
		"test/fixture.yaml",
		"nested/deep/config.yaml",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("test"), 0644)
		require.NoError(t, err)
	}

	discoverer := NewFileDiscoverer(tempDir)

	t.Run("Should discover files with basic patterns", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{"workflows/*.yaml"}, nil)
		assert.NoError(t, err)
		assert.Len(t, files, 2)

		// Convert to relative paths for easier comparison
		relFiles := make([]string, len(files))
		for i, f := range files {
			rel, _ := filepath.Rel(tempDir, f)
			relFiles[i] = filepath.ToSlash(rel)
		}

		assert.Contains(t, relFiles, "workflows/user.yaml")
		assert.Contains(t, relFiles, "workflows/admin.yaml")
	})

	t.Run("Should discover files with ** patterns", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{"workflows/**/*.yaml"}, nil)
		assert.NoError(t, err)
		assert.Len(t, files, 3)

		relFiles := make([]string, len(files))
		for i, f := range files {
			rel, _ := filepath.Rel(tempDir, f)
			relFiles[i] = filepath.ToSlash(rel)
		}

		assert.Contains(t, relFiles, "workflows/user.yaml")
		assert.Contains(t, relFiles, "workflows/admin.yaml")
		assert.Contains(t, relFiles, "workflows/test/test.yaml")
	})

	t.Run("Should handle multiple include patterns", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{"workflows/*.yaml", "tasks/*.yaml"}, nil)
		assert.NoError(t, err)
		assert.Len(t, files, 4) // 2 workflows + 2 tasks (temp file excluded by default)
	})

	t.Run("Should exclude default temporary files", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{"**/*.yaml"}, nil)
		assert.NoError(t, err)

		// Check that temp files are excluded
		for _, file := range files {
			assert.NotContains(t, file, ".#")
			assert.NotContains(t, file, "~")
			assert.NotContains(t, file, ".bak")
		}
	})

	t.Run("Should apply custom exclude patterns", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover(
			[]string{"**/*.yaml"},
			[]string{"**/test/**", "workflows/*"},
		)
		assert.NoError(t, err)

		relFiles := make([]string, len(files))
		for i, f := range files {
			rel, _ := filepath.Rel(tempDir, f)
			relFiles[i] = filepath.ToSlash(rel)
		}

		// Should exclude test directory and workflows
		assert.NotContains(t, relFiles, "workflows/user.yaml")
		assert.NotContains(t, relFiles, "workflows/test/test.yaml")
		assert.NotContains(t, relFiles, "test/fixture.yaml")

		// Should include others
		assert.Contains(t, relFiles, "tasks/email.yaml")
		assert.Contains(t, relFiles, "agents/chatbot.yaml")
	})

	t.Run("Should handle empty includes", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{}, nil)
		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("Should deduplicate files", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover(
			[]string{"tasks/*.yaml", "tasks/email.yaml"},
			nil,
		)
		assert.NoError(t, err)

		// Count occurrences of email.yaml
		emailCount := 0
		for _, file := range files {
			if strings.HasSuffix(file, "email.yaml") {
				emailCount++
			}
		}
		assert.Equal(t, 1, emailCount)
	})
}

func TestFileDiscoverer_Security(t *testing.T) {
	tempDir := t.TempDir()
	discoverer := NewFileDiscoverer(tempDir)

	t.Run("Should reject absolute paths", func(t *testing.T) {
		t.Parallel()
		_, err := discoverer.Discover([]string{"/etc/passwd"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "INVALID_PATTERN")
		assert.Contains(t, err.Error(), "absolute paths not allowed")
	})

	t.Run("Should reject parent directory references", func(t *testing.T) {
		t.Parallel()
		_, err := discoverer.Discover([]string{"../../../etc/passwd"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "INVALID_PATTERN")
		assert.Contains(t, err.Error(), "parent directory references not allowed")
	})

	t.Run("Should reject patterns with .. in the middle", func(t *testing.T) {
		t.Parallel()
		_, err := discoverer.Discover([]string{"workflows/../../../etc/passwd"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "INVALID_PATTERN")
	})
}

func TestFileDiscoverer_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()

	// Create files with edge case names
	edgeCaseFiles := []string{
		"config.yaml",
		"dir1/config.yaml",
		"dir2/config.yaml",
		"space name.yaml",
		"special-chars!@#.yaml",
	}

	for _, file := range edgeCaseFiles {
		fullPath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("test"), 0644)
		require.NoError(t, err)
	}

	discoverer := NewFileDiscoverer(tempDir)

	t.Run("Should handle duplicate basenames in different directories", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{"**/config.yaml"}, nil)
		assert.NoError(t, err)
		assert.Len(t, files, 3) // root, dir1, dir2
	})

	t.Run("Should handle files with spaces", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{"*.yaml"}, nil)
		assert.NoError(t, err)

		found := false
		for _, file := range files {
			if strings.Contains(file, "space name.yaml") {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("Should handle files with special characters", func(t *testing.T) {
		t.Parallel()
		files, err := discoverer.Discover([]string{"special-*.yaml"}, nil)
		assert.NoError(t, err)
		assert.Len(t, files, 1)
	})

	t.Run("Should handle invalid glob patterns", func(t *testing.T) {
		t.Parallel()
		_, err := discoverer.Discover([]string{"[invalid"}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid glob pattern")
	})
}
