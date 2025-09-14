package attachment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SnapshotTempFiles returns a snapshot of temp files that start with the
// compozy attachment prefix. It filters by prefix to avoid unrelated files.
func SnapshotTempFiles(t *testing.T) map[string]struct{} {
	t.Helper()
	tmp := os.TempDir()
	out := map[string]struct{}{}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Logf("SnapshotTempFiles: ReadDir(%q) failed: %v", tmp, err)
		return out
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, TempFilePrefix) {
			out[filepath.Join(tmp, name)] = struct{}{}
		}
	}
	return out
}
