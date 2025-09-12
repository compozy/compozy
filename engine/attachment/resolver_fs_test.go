package attachment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
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
		require.Equal(t, f, p)
		require.Equal(t, "image/png", res.MIME())
		rc, err := res.Open()
		require.NoError(t, err)
		rc.Close()
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
}
