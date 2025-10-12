package helpers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const updateGoldenEnv = "UPDATE_GOLDEN"

func GoldenFilePath(t *testing.T, relPath string) string {
	t.Helper()
	root, err := FindProjectRoot()
	require.NoError(t, err)
	return filepath.Join(root, relPath)
}

func LoadGolden(t *testing.T, relPath string) []byte {
	t.Helper()
	path := GoldenFilePath(t, relPath)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

func CompareWithGolden(t *testing.T, actual []byte, relPath string) {
	t.Helper()
	path := GoldenFilePath(t, relPath)
	if os.Getenv(updateGoldenEnv) == "1" {
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(path, actual, 0o600)
		require.NoError(t, err)
	}
	expected := LoadGolden(t, relPath)
	require.Equal(t, string(expected), string(actual))
}
