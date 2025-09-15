package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CWDFromPath_And_Methods(t *testing.T) {
	t.Run("Should return current dir when empty path", func(t *testing.T) {
		cwd, err := CWDFromPath("")
		require.NoError(t, err)
		wd, _ := os.Getwd()
		assert.Equal(t, wd, cwd.Path)
	})
	t.Run("Should normalize relative path and file path", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "a.txt")
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
		c1, err := CWDFromPath(dir)
		require.NoError(t, err)
		assert.Equal(t, dir, c1.Path)
		c2, err := CWDFromPath(file)
		require.NoError(t, err)
		assert.Equal(t, dir, c2.Path)
	})
	t.Run("Should Set, PathStr, Validate, Clone and JoinAndCheck", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "b.txt")
		require.NoError(t, os.WriteFile(file, []byte("y"), 0o644))
		var p *PathCWD
		err := p.Set("whatever")
		assert.ErrorContains(t, err, "CWD is nil")
		c := &PathCWD{}
		require.NoError(t, c.Set(dir))
		assert.Equal(t, dir, c.PathStr())
		assert.NoError(t, c.Validate())
		got, err := c.JoinAndCheck("b.txt")
		require.NoError(t, err)
		assert.Equal(t, file, got)
		clone, err := c.Clone()
		require.NoError(t, err)
		assert.Equal(t, c.Path, clone.Path)
		// missing file on a valid CWD yields not found
		_, err = c.JoinAndCheck("missing2")
		assert.ErrorContains(t, err, "file not found or inaccessible")
		// empty CWD yields not-set error
		c2 := &PathCWD{}
		_, err = c2.JoinAndCheck("missing")
		assert.ErrorContains(t, err, "CWD is not set")
	})
	t.Run("Should read file via ReadFile", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "c.txt")
		require.NoError(t, os.WriteFile(p, []byte("hello"), 0o644))
		b, err := ReadFile(p)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), b)
	})
}
