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

	t.Run("ShouldFallbackToWorkingDirWhenBaseDirMissing", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("env-file", "", "")
		require.NoError(t, cmd.Flags().Set("env-file", ".env"))

		baseDir := filepath.Join(t.TempDir(), "example")
		require.NoError(t, os.MkdirAll(baseDir, 0o755))

		fallbackDir := t.TempDir()
		fallbackEnv := filepath.Join(fallbackDir, ".env")
		require.NoError(t, os.WriteFile(fallbackEnv, []byte("KEY=fallback"), 0o600))

		originalWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(fallbackDir))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(originalWD))
		})

		resolved := resolveEnvFilePath(cmd, baseDir)
		resolvedEval, err := filepath.EvalSymlinks(resolved)
		require.NoError(t, err)
		expectedEval, err := filepath.EvalSymlinks(fallbackEnv)
		require.NoError(t, err)
		require.Equal(t, expectedEval, resolvedEval)
	})

	t.Run("ShouldReturnBaseDirPathEvenWhenFileMissing", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("env-file", "", "")
		require.NoError(t, cmd.Flags().Set("env-file", ".env"))
		baseDir := filepath.Join(t.TempDir(), "example")
		require.NoError(t, os.MkdirAll(baseDir, 0o755))
		resolved := resolveEnvFilePath(cmd, baseDir)
		require.Equal(t, filepath.Join(baseDir, ".env"), resolved)
	})
}
