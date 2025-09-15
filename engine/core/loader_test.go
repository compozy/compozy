package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ResolvePath(t *testing.T) {
	t.Run("Should resolve relative path using cwd when provided", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "a.yaml")
		require.NoError(t, os.WriteFile(f, []byte("k: v"), 0o644))
		c := &PathCWD{Path: dir}
		p, err := ResolvePath(c, "a.yaml")
		require.NoError(t, err)
		assert.Equal(t, f, p)
	})
	t.Run("Should resolve absolute/relative without cwd", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "b.yaml")
		require.NoError(t, os.WriteFile(f, []byte("k: v"), 0o644))
		p, err := ResolvePath(nil, f)
		require.NoError(t, err)
		pEval, err := filepath.EvalSymlinks(p)
		require.NoError(t, err)
		fEval, err := filepath.EvalSymlinks(f)
		require.NoError(t, err)
		assert.Equal(t, fEval, pEval)
		// relative without cwd should still become absolute
		oldwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(dir))
		t.Cleanup(func() { require.NoError(t, os.Chdir(oldwd)) })
		p2, err := ResolvePath(nil, "b.yaml")
		require.NoError(t, err)
		p2Eval, err := filepath.EvalSymlinks(p2)
		require.NoError(t, err)
		assert.Equal(t, fEval, p2Eval)
	})
}

func Test_MapFromFilePath(t *testing.T) {
	t.Run("Should read YAML file as map", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "c.yaml")
		require.NoError(t, os.WriteFile(p, []byte("x: 1\ny: foo\n"), 0o644))
		m, err := MapFromFilePath(p)
		require.NoError(t, err)
		assert.Equal(t, 1, m["x"])
		assert.Equal(t, "foo", m["y"])
	})
}
