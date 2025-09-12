package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// SnapshotTempFiles returns a snapshot of temp files that start with the
// compozy attachment prefix. It filters by prefix to avoid unrelated files.
func SnapshotTempFiles(t *testing.T) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) >= 13 && name[:13] == "compozy-att-" {
			out[filepath.Join(os.TempDir(), name)] = struct{}{}
		}
	}
	return out
}
