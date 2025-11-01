package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var repoRoot = detectRepoRoot()

func detectRepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("testutil: failed to detect repository root via runtime.Caller")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Clean(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("testutil: repository root not found")
		}
		dir = parent
	}
}

// TestDataPath returns an absolute path rooted at sdk/testdata for the provided segments.
func TestDataPath(t *testing.T, segments ...string) string {
	t.Helper()
	parts := append([]string{repoRoot, "sdk", "testdata"}, segments...)
	return filepath.Join(parts...)
}

// ReadTestData reads a file from sdk/testdata using the provided path segments and fails the test on error.
func ReadTestData(t *testing.T, segments ...string) []byte {
	t.Helper()
	path := TestDataPath(t, segments...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read testdata %s: %v", path, err)
	}
	return data
}
