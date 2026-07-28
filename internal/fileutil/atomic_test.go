package fileutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAtomicWriteFileWritesContentAndPermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should write content with requested permissions", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "meta.json")
		content := []byte("hello\n")
		const perm = 0o640

		if err := AtomicWriteFile(path, content, perm); err != nil {
			t.Fatalf("AtomicWriteFile() error = %v", err)
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
		if got, want := info.Mode().Perm(), os.FileMode(perm); got != want {
			t.Fatalf("file permissions = %o, want %o", got, want)
		}
	})
}

func TestAtomicWriteFileHandlesBoundaryLengthBasename(t *testing.T) {
	t.Parallel()

	t.Run("Should write a valid maximum length basename", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("Windows path-length behavior depends on volume and long-path policy")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, strings.Repeat("a", 255))
		if err := os.WriteFile(path, []byte("seed"), 0o644); err != nil {
			t.Skipf("filesystem does not accept 255-byte basename fixture: %v", err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("Remove(seed boundary file) error = %v", err)
		}

		content := []byte("boundary content")
		if err := AtomicWriteFile(path, content, 0o644); err != nil {
			t.Fatalf("AtomicWriteFile(boundary basename) error = %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(boundary basename) error = %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Fatalf("ReadFile(boundary basename) = %q, want %q", string(got), string(content))
		}
	})
}

func TestAtomicWriteFileDoesNotCorruptTargetOnFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should leave original target unchanged when temp write fails", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("directory permission failure semantics are platform-specific on windows")
		}

		dir := t.TempDir()
		path := filepath.Join(dir, "target.txt")
		original := []byte("original")
		if err := os.WriteFile(path, original, 0o644); err != nil {
			t.Fatalf("WriteFile(original) error = %v", err)
		}

		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatalf("Chmod(read-only dir) error = %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(dir, 0o755); err != nil {
				t.Errorf("Chmod(restore dir) error = %v", err)
			}
		})

		err := AtomicWriteFile(path, []byte("updated"), 0o644)
		if err == nil {
			t.Fatal("AtomicWriteFile() error = nil, want failure in read-only directory")
		}

		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("ReadFile(original target) error = %v", readErr)
		}
		if !bytes.Equal(got, original) {
			t.Fatalf("target contents after failure = %q, want %q", string(got), string(original))
		}
	})
}

func TestAtomicWriteFileRejectsBlankPath(t *testing.T) {
	t.Parallel()

	t.Run("Should reject blank paths with invalid path sentinel", func(t *testing.T) {
		t.Parallel()

		err := AtomicWriteFile("   ", []byte("content"), 0o644)
		if !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("AtomicWriteFile(blank path) error = %v, want ErrInvalidPath", err)
		}
	})
}

func TestAtomicWriteFileRejectsNullBytePath(t *testing.T) {
	t.Parallel()

	t.Run("Should reject NUL bytes before creating filesystem artifacts", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		invalidName := "leak" + "\x00" + ".txt"
		err := AtomicWriteFile(filepath.Join(dir, invalidName), []byte("content"), 0o644)
		if !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("AtomicWriteFile(NUL path) error = %v, want ErrInvalidPath", err)
		}

		matches, globErr := filepath.Glob(filepath.Join(dir, "leak*"))
		if globErr != nil {
			t.Fatalf("filepath.Glob(leak*) error = %v", globErr)
		}
		if len(matches) != 0 {
			t.Fatalf("leak artifacts = %#v, want none", matches)
		}
	})
}

func TestAtomicWriteFilePreservesLiteralWhitespaceInPath(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve literal whitespace in path", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("trailing-space filenames are normalized by Win32 APIs")
		}

		path := filepath.Join(t.TempDir(), "target.txt ")
		trimmedPath := strings.TrimSpace(path)
		if path == trimmedPath {
			t.Fatal("test fixture path did not retain trailing whitespace")
		}

		content := []byte("content with trailing-space filename")
		if err := AtomicWriteFile(path, content, 0o644); err != nil {
			t.Fatalf("AtomicWriteFile() error = %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(literal path) error = %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Fatalf("ReadFile(literal path) = %q, want %q", string(got), string(content))
		}

		if _, err := os.Stat(trimmedPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(trimmed path) error = %v, want %v", err, os.ErrNotExist)
		}
	})
}

func TestAtomicWriteFileFailsWhenParentDirectoryIsMissing(t *testing.T) {
	t.Parallel()

	t.Run("Should fail when parent directory is missing", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "missing", "target.txt")
		if err := AtomicWriteFile(path, []byte("content"), 0o644); err == nil {
			t.Fatal("AtomicWriteFile(missing dir) error = nil, want non-nil")
		}
	})
}

func TestAtomicWriteFileFailsWhenTargetIsDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should fail when target is a directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "target")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("Mkdir(target dir) error = %v", err)
		}

		if err := AtomicWriteFile(path, []byte("content"), 0o644); err == nil {
			t.Fatal("AtomicWriteFile(target dir) error = nil, want non-nil")
		}
	})
}

func TestWriteTempFileReturnsErrorForClosedFile(t *testing.T) {
	t.Parallel()

	t.Run("Should return error for closed file", func(t *testing.T) {
		t.Parallel()

		file, err := os.CreateTemp(t.TempDir(), "closed-*")
		if err != nil {
			t.Fatalf("CreateTemp() error = %v", err)
		}
		path := file.Name()
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		if err := writeTempFile(file, path, []byte("content"), 0o644); err == nil {
			t.Fatal("writeTempFile(closed file) error = nil, want non-nil")
		}
	})
}

func TestSyncDirRejectsMissingDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should reject missing directory", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("syncDir is a no-op on windows")
		}

		if err := syncDir(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("syncDir(missing) error = nil, want non-nil")
		}
	})
}
