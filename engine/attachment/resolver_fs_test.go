package attachment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePNG(path string) error {
	data := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	return os.WriteFile(path, data, 0o644)
}

func Test_resolveLocalFile(t *testing.T) {
	t.Run("Should resolve and detect MIME for local file within CWD", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "a.png")
		require.NoError(t, writePNG(f))
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		res, err := resolveLocalFile(context.Background(), cwd, "a.png", TypeImage)
		require.NoError(t, err)
		defer res.Cleanup()
		p, ok := res.AsFilePath()
		require.True(t, ok)
		// Resolve symlinks for macOS temp directories
		expectedPath, err := filepath.EvalSymlinks(f)
		require.NoError(t, err)
		require.Equal(t, expectedPath, p)
		require.Equal(t, "image/png", res.MIME())
		rc, err := res.Open()
		require.NoError(t, err)
		require.NoError(t, rc.Close())
	})

	t.Run("Should reject path traversal outside CWD", func(t *testing.T) {
		dir := t.TempDir()
		outer := filepath.Dir(dir)
		f := filepath.Join(outer, "x.png")
		require.NoError(t, writePNG(f))
		t.Cleanup(func() { _ = os.Remove(f) })
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		_, err = resolveLocalFile(context.Background(), cwd, "../x.png", TypeImage)
		require.Error(t, err)
	})

	t.Run("Should reject absolute path outside CWD (absolute)", func(t *testing.T) {
		dir := t.TempDir()
		outer := filepath.Dir(dir)
		f := filepath.Join(outer, "y.png")
		require.NoError(t, writePNG(f))
		t.Cleanup(func() { _ = os.Remove(f) })
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		_, err = resolveLocalFile(context.Background(), cwd, f, TypeImage)
		require.Error(t, err)
	})

	t.Run("Should reject symlink pointing outside CWD", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(subDir, 0o755))

		// Create file outside CWD
		outer := filepath.Dir(dir)
		outerFile := filepath.Join(outer, "outside.png")
		require.NoError(t, writePNG(outerFile))
		t.Cleanup(func() { _ = os.Remove(outerFile) })

		// Create symlink inside subDir pointing to file outside CWD
		linkPath := filepath.Join(subDir, "evil_link.png")
		require.NoError(t, os.Symlink(outerFile, linkPath))
		t.Cleanup(func() { _ = os.Remove(linkPath) })

		cwd, err := core.CWDFromPath(subDir)
		require.NoError(t, err)

		// Should reject symlink pointing outside CWD
		_, err = resolveLocalFile(context.Background(), cwd, "evil_link.png", TypeImage)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path outside CWD")
	})

	t.Run("Should allow symlink pointing inside CWD", func(t *testing.T) {
		dir := t.TempDir()

		// Create real file inside CWD
		realFile := filepath.Join(dir, "real.png")
		require.NoError(t, writePNG(realFile))

		// Create symlink inside CWD pointing to another file inside CWD
		linkPath := filepath.Join(dir, "valid_link.png")
		require.NoError(t, os.Symlink(realFile, linkPath))
		t.Cleanup(func() { _ = os.Remove(linkPath) })

		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)

		// Should allow symlink pointing inside CWD
		res, err := resolveLocalFile(context.Background(), cwd, "valid_link.png", TypeImage)
		require.NoError(t, err)
		defer res.Cleanup()

		// The resolved path should point to the real file (may be resolved through symlinks)
		p, ok := res.AsFilePath()
		require.True(t, ok)
		resolvedRealFile, err := filepath.EvalSymlinks(realFile)
		require.NoError(t, err)
		require.Equal(t, resolvedRealFile, p)
	})

	t.Run("Should reject broken symlink", func(t *testing.T) {
		dir := t.TempDir()

		// Create symlink pointing to non-existent file
		linkPath := filepath.Join(dir, "broken_link.png")
		require.NoError(t, os.Symlink("/non/existent/file.png", linkPath))
		t.Cleanup(func() { _ = os.Remove(linkPath) })

		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)

		// Should reject broken symlink (EvalSymlinks fails)
		_, err = resolveLocalFile(context.Background(), cwd, "broken_link.png", TypeImage)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resolve path symlinks")
	})
}

func Test_pathWithin(t *testing.T) {
	t.Run("Should allow paths within root", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(subDir, 0o755))
		file := filepath.Join(subDir, "test.txt")
		require.NoError(t, os.WriteFile(file, []byte("test"), 0o644))

		within, err := pathWithin(dir, file)
		require.NoError(t, err)
		assert.True(t, within)
	})

	t.Run("Should reject paths outside root", func(t *testing.T) {
		dir := t.TempDir()
		outer := filepath.Dir(dir)
		outerFile := filepath.Join(outer, "outside.txt")
		require.NoError(t, os.WriteFile(outerFile, []byte("test"), 0o644))
		t.Cleanup(func() { _ = os.Remove(outerFile) })

		within, err := pathWithin(dir, outerFile)
		require.NoError(t, err)
		assert.False(t, within)
	})

	t.Run("Should reject symlink pointing outside root", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(subDir, 0o755))

		// Create file outside root
		outer := filepath.Dir(dir)
		outerFile := filepath.Join(outer, "outside.txt")
		require.NoError(t, os.WriteFile(outerFile, []byte("test"), 0o644))
		t.Cleanup(func() { _ = os.Remove(outerFile) })

		// Create symlink inside subDir pointing outside
		linkPath := filepath.Join(subDir, "evil_link.txt")
		require.NoError(t, os.Symlink(outerFile, linkPath))
		t.Cleanup(func() { _ = os.Remove(linkPath) })

		within, err := pathWithin(dir, linkPath)
		require.NoError(t, err)
		assert.False(t, within)
	})

	t.Run("Should allow symlink pointing inside root", func(t *testing.T) {
		dir := t.TempDir()

		// Create real file inside root
		realFile := filepath.Join(dir, "real.txt")
		require.NoError(t, os.WriteFile(realFile, []byte("test"), 0o644))

		// Create symlink inside root pointing to real file
		linkPath := filepath.Join(dir, "valid_link.txt")
		require.NoError(t, os.Symlink(realFile, linkPath))
		t.Cleanup(func() { _ = os.Remove(linkPath) })

		within, err := pathWithin(dir, linkPath)
		require.NoError(t, err)
		assert.True(t, within)
	})

	t.Run("Should reject broken symlink", func(t *testing.T) {
		dir := t.TempDir()

		// Create broken symlink
		linkPath := filepath.Join(dir, "broken_link.txt")
		require.NoError(t, os.Symlink("/non/existent/file.txt", linkPath))
		t.Cleanup(func() { _ = os.Remove(linkPath) })

		within, err := pathWithin(dir, linkPath)
		require.Error(t, err)
		// The error message changed to be more specific about non-existent targets
		assert.Contains(t, err.Error(), "target path does not exist")
		assert.False(t, within)
	})

	t.Run("Should handle current directory", func(t *testing.T) {
		dir := t.TempDir()

		within, err := pathWithin(dir, dir)
		require.NoError(t, err)
		assert.True(t, within)
	})

	t.Run("Should handle relative paths", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(subDir, 0o755))
		otherFile := filepath.Join(dir, "other")
		require.NoError(t, os.WriteFile(otherFile, []byte("test"), 0o644))

		// Test with relative path
		within, err := pathWithin(dir, filepath.Join(dir, "sub", "..", "other"))
		require.NoError(t, err)
		assert.True(t, within)
	})
}
