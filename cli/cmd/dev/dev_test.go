package dev

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestResolveEnvFilePath(t *testing.T) {
	t.Run("ShouldReturnAbsoluteEnvFilePath", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("env-file", "", "")
		absFile := filepath.Join(t.TempDir(), "custom.env")
		require.NoError(t, os.WriteFile(absFile, []byte("key=value"), 0o600))
		require.NoError(t, cmd.Flags().Set("env-file", absFile))
		resolved := resolveEnvFilePath(cmd, t.TempDir())
		require.Equal(t, absFile, resolved)
	})

	t.Run("ShouldPreferBaseDirEnvFileWhenPresent", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("env-file", "", "")
		require.NoError(t, cmd.Flags().Set("env-file", ".env"))
		baseDir := t.TempDir()
		expected := filepath.Join(baseDir, ".env")
		require.NoError(t, os.WriteFile(expected, []byte("KEY=base"), 0o600))
		resolved := resolveEnvFilePath(cmd, baseDir)
		require.Equal(t, expected, resolved)
	})

	t.Run("ShouldFallbackToOriginalWorkingDirectoryWhenBaseDirMissing", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("env-file", "", "")
		require.NoError(t, cmd.Flags().Set("env-file", ".env"))
		originalWD, err := os.Getwd()
		require.NoError(t, err)
		rootDir := t.TempDir()
		require.NoError(t, os.Chdir(rootDir))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(originalWD))
		})
		fallback := filepath.Join(rootDir, ".env")
		require.NoError(t, os.WriteFile(fallback, []byte("KEY=root"), 0o600))
		baseDir := filepath.Join(rootDir, "example")
		require.NoError(t, os.MkdirAll(baseDir, 0o755))
		resolved := resolveEnvFilePath(cmd, baseDir)
		resolvedEval, err := filepath.EvalSymlinks(resolved)
		require.NoError(t, err)
		expectedEval, err := filepath.EvalSymlinks(fallback)
		require.NoError(t, err)
		require.Equal(t, expectedEval, resolvedEval)
	})
}
