//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Invariant: absolute paths may cross a system-owned root alias on macOS, but
// symlinks below that trusted boundary remain forbidden.
// Owner: fileutil non-following path acquisition.
// Canonical suite: fileutil Unix no-follow filesystem unit tests.
func TestOpenDirectoryHandlesDarwinSystemRootAlias(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("system-owned root aliases are a macOS filesystem contract")
	}

	t.Run("Should open a temporary directory through the system root alias", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("filepath.EvalSymlinks(%q) error = %v", root, err)
		}
		if resolved == root {
			t.Skip("temporary directory does not cross a system root alias")
		}

		directory, err := OpenDirectory(root)
		if err != nil {
			t.Fatalf("OpenDirectory(%q) error = %v", root, err)
		}
		t.Cleanup(func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		})
	})

	t.Run("Should reject a symlink below the system root alias", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("os.Mkdir(target) error = %v", err)
		}
		link := filepath.Join(root, "linked")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("os.Symlink(target, link) error = %v", err)
		}

		_, err := OpenDirectory(link)
		if !errors.Is(err, ErrSymlink) {
			t.Fatalf("OpenDirectory(%q) error = %v, want ErrSymlink", link, err)
		}
	})
}

func TestDirectoryRejectsInvalidChildNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		child string
	}{
		{name: "Should reject an empty child name", child: ""},
		{name: "Should reject the current directory child name", child: "."},
		{name: "Should reject the parent directory child name", child: ".."},
		{name: "Should reject a rooted child name", child: string(filepath.Separator)},
		{name: "Should reject a nested child path", child: filepath.Join("nested", "child")},
		{name: "Should reject a child name containing a NUL byte", child: "child\x00name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			directory, err := OpenDirectory(t.TempDir())
			if err != nil {
				t.Fatalf("OpenDirectory() error = %v", err)
			}
			t.Cleanup(func() {
				if closeErr := directory.Close(); closeErr != nil {
					t.Errorf("Directory.Close() error = %v", closeErr)
				}
			})

			_, err = directory.OpenDirectory(test.child)
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("OpenDirectory(%q) error = %v, want ErrInvalidPath", test.child, err)
			}
		})
	}
}

func TestDirectoryRejectsPortableUnsafeChildNames(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a backslash even on Unix", func(t *testing.T) {
		t.Parallel()

		const name = "agent\\definition"
		directory, err := OpenDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("OpenDirectory(temp directory) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		})

		_, err = directory.OpenRegularFile(name)
		if !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("OpenRegularFile(%q) error = %v, want ErrInvalidPath", name, err)
		}
	})
}

func TestDirectoryRejectsSymlinkAndFIFOChildren(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a regular file where a directory is required", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "regular.txt"), []byte("regular"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(regular file) error = %v", err)
		}
		directory, err := OpenDirectory(root)
		if err != nil {
			t.Fatalf("OpenDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		})

		_, err = directory.OpenDirectory("regular.txt")
		if !errors.Is(err, ErrNotDirectory) {
			t.Fatalf("OpenDirectory(regular.txt) error = %v, want ErrNotDirectory", err)
		}
	})

	t.Run("Should reject a symlinked child directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("os.Mkdir() error = %v", err)
		}
		link := filepath.Join(root, "linked")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("os.Symlink() error = %v", err)
		}

		directory, err := OpenDirectory(root)
		if err != nil {
			t.Fatalf("OpenDirectory() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		})

		_, err = directory.OpenDirectory("linked")
		if !errors.Is(err, ErrSymlink) {
			t.Fatalf("OpenDirectory(linked) error = %v, want ErrSymlink", err)
		}
	})

	t.Run("Should reject a FIFO child without blocking", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		fifoPath := filepath.Join(root, "config.fifo")
		if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
			t.Fatalf("unix.Mkfifo() error = %v", err)
		}

		directory, err := OpenDirectory(root)
		if err != nil {
			t.Fatalf("OpenDirectory() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		})

		result := make(chan error, 1)
		go func() {
			file, openErr := directory.OpenRegularFile("config.fifo")
			if openErr != nil {
				result <- openErr
				return
			}
			result <- file.Close()
		}()

		select {
		case openErr := <-result:
			if !errors.Is(openErr, ErrNotRegular) {
				t.Fatalf("OpenRegularFile(config.fifo) error = %v, want ErrNotRegular", openErr)
			}
		case <-time.After(time.Second):
			t.Fatal("OpenRegularFile(config.fifo) blocked")
		}
	})
}
