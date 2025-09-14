package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SnapshotTempFiles returns a snapshot of temp files that start with the
// compozy attachment prefix. It filters by prefix to avoid unrelated files.
func SnapshotTempFiles(t *testing.T) map[string]struct{} {
	const attPrefix = "compozy-att-"

	t.Helper()
	out := map[string]struct{}{}
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, attPrefix) {
			out[filepath.Join(os.TempDir(), name)] = struct{}{}
		}
	}
	return out
}
