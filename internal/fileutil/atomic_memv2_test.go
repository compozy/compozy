package fileutil

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	t.Run("Should write content atomically with default file permissions", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "MEMORY.md")
		content := []byte("curated memory\n")

		if err := AtomicWrite(path, content); err != nil {
			t.Fatalf("AtomicWrite() error = %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Fatalf("ReadFile() = %q, want %q", string(got), string(content))
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o644); got != want {
			t.Fatalf("mode = %o, want %o", got, want)
		}
	})
}
