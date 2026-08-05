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

func TestCloseOwnerConsumesOwnershipBeforeClosing(t *testing.T) {
	t.Parallel()

	t.Run("Should close exactly once and preserve primary and close error identities", func(t *testing.T) {
		t.Parallel()

		primaryErr := errors.New("primary failure")
		closeErr := errors.New("close failure")
		closeCalls := 0
		owner := newCloseOwner(func() error {
			closeCalls++
			return closeErr
		})

		firstErr := owner.closeWithError(primaryErr, "fileutil: close owned file")
		secondErr := owner.closeWithError(firstErr, "fileutil: close owned file again")

		if closeCalls != 1 {
			t.Fatalf("close calls = %d, want 1", closeCalls)
		}
		if secondErr != firstErr {
			t.Fatalf("second close error = %v, want original joined error identity %v", secondErr, firstErr)
		}
		if !errors.Is(secondErr, primaryErr) {
			t.Fatalf("close error = %v, want primary error identity %v", secondErr, primaryErr)
		}
		if !errors.Is(secondErr, closeErr) {
			t.Fatalf("close error = %v, want first close error identity %v", secondErr, closeErr)
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

// Invariant: components accepted by atomic publication have one portable
// filename identity, and rejected components create no filesystem artifact.
// Owner: fileutil path grammar and atomic publication.
// Canonical suite: fileutil atomic-write behavior.
func TestAtomicWriteFileRejectsPortableUnsafeComponents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		child string
	}{
		{name: "Should reject a trailing-space component", child: "target.txt "},
		{name: "Should reject a trailing-dot component", child: "target."},
		{name: "Should reject an alternate-data-stream component", child: "target:stream"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), test.child)
			err := AtomicWriteFile(path, []byte("must not publish"), 0o644)
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("AtomicWriteFile(%q) error = %v, want ErrInvalidPath", path, err)
			}
			if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("Lstat(%q) error = %v, want %v", path, statErr, os.ErrNotExist)
			}
		})
	}

	t.Run("Should preserve internal whitespace in a portable component", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "target config.txt")
		contents := []byte("portable internal whitespace")
		if err := AtomicWriteFile(path, contents, 0o644); err != nil {
			t.Fatalf("AtomicWriteFile(%q) error = %v", path, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		if !bytes.Equal(got, contents) {
			t.Fatalf("ReadFile(%q) = %q, want %q", path, got, contents)
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

func TestSyncDirRejectsMissingDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should reject missing directory", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("directory synchronization is a no-op on windows")
		}

		if err := SyncDir(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("SyncDir(missing) error = nil, want non-nil")
		}
	})
}
