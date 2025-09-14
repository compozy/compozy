package attachment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
)

func Test_Resolve_Factory_Routing(t *testing.T) {
	t.Run("Should resolve an image URL without fetching", func(t *testing.T) {
		a := &ImageAttachment{URL: "https://example.com/x.png", Source: SourceURL}
		res, err := Resolve(context.Background(), a, nil)
		require.NoError(t, err)
		defer res.Cleanup()
		u, ok := res.AsURL()
		require.True(t, ok)
		require.Equal(t, a.URL, u)
	})
	t.Run("Should resolve a PDF local path to a file", func(t *testing.T) {
		// Use a tiny local HTTP server
		// Covered in http test; here only exercise path-based resolution
		dir := t.TempDir()
		// Create fake pdf header (%PDF-1.)
		pdf := []byte{'%', 'P', 'D', 'F', '-', '1', '.'}
		p := writeTempFile(t, dir, "a.pdf", pdf)
		expectedPath, err := filepath.EvalSymlinks(p)
		require.NoError(t, err)
		cwd, err := core.CWDFromPath(dir)
		require.NoError(t, err)
		a := &PDFAttachment{Path: "a.pdf", Source: SourcePath}
		res, err := Resolve(context.Background(), a, cwd)
		require.NoError(t, err)
		defer res.Cleanup()
		fp, ok := res.AsFilePath()
		require.True(t, ok)
		require.Equal(t, expectedPath, fp)
	})
}

func writeTempFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
