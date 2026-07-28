package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicRemoveFileRemovesFileAndRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()

	t.Run("Should remove a regular file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "target.txt")
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		if err := AtomicRemoveFile(path); err != nil {
			t.Fatalf("AtomicRemoveFile() error = %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(removed file) error = %v, want %v", err, os.ErrNotExist)
		}
	})

	t.Run("Should reject directories", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if err := AtomicRemoveFile(dir); err == nil {
			t.Fatal("AtomicRemoveFile(directory) error = nil, want non-nil")
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("Stat(directory after AtomicRemoveFile) error = %v, want directory to remain", err)
		}
	})

	t.Run("Should reject blank paths", func(t *testing.T) {
		t.Parallel()

		if err := AtomicRemoveFile(" "); err == nil {
			t.Fatal("AtomicRemoveFile(blank) error = nil, want non-nil")
		}
	})
}

func TestRemoveFileOnlyRejectsDirectories(t *testing.T) {
	t.Parallel()

	t.Run("Should reject directories without removing them", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if err := removeFileOnly(dir); err == nil {
			t.Fatal("removeFileOnly(directory) error = nil, want non-nil")
		}
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("Stat(directory after removeFileOnly) error = %v, want directory to remain", err)
		}
	})
}
