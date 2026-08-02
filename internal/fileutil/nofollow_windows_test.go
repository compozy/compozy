//go:build windows

package fileutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsFileModeInformationClass = 16

type windowsFileModeInformation struct {
	Mode uint32
}

// Invariant: every handle used as the source of a handle-bound Windows rename
// carries FILE_WRITE_THROUGH so the rename request cannot silently use the
// ordinary metadata cache path.
// Owner: fileutil Windows capability-rooted publication.
// Canonical suite: fileutil Windows no-follow behavior.
func TestWindowsRenameSourcesUseWriteThrough(t *testing.T) {
	t.Parallel()

	t.Run("Should open a directory move source with write-through", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		if err := os.Mkdir(filepath.Join(rootPath, "source"), 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v", err)
		}
		root, err := OpenDirectoryForMutation(rootPath)
		if err != nil {
			t.Fatalf("OpenDirectoryForMutation(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		source, err := root.OpenDirectoryForMove("source")
		if err != nil {
			t.Fatalf("OpenDirectoryForMove(source) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Directory.Close(source) error = %v", closeErr)
			}
		})

		assertWindowsFileUsesWriteThrough(t, source.file)
	})

	t.Run("Should create an atomic publication source with write-through", func(t *testing.T) {
		t.Parallel()

		root, err := OpenDirectoryForMutation(t.TempDir())
		if err != nil {
			t.Fatalf("OpenDirectoryForMutation(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})
		_, temporary, err := root.createTemporaryFile(0o600)
		if err != nil {
			t.Fatalf("Directory.createTemporaryFile() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := temporary.Close(); closeErr != nil {
				t.Errorf("temporary.Close() error = %v", closeErr)
			}
		})

		assertWindowsFileUsesWriteThrough(t, temporary)
	})
}

func assertWindowsFileUsesWriteThrough(t *testing.T, file *os.File) {
	t.Helper()

	var mode windowsFileModeInformation
	var status windows.IO_STATUS_BLOCK
	if err := windows.NtQueryInformationFile(
		windows.Handle(file.Fd()),
		&status,
		(*byte)(unsafe.Pointer(&mode)),
		uint32(unsafe.Sizeof(mode)),
		windowsFileModeInformationClass,
	); err != nil {
		t.Fatalf("NtQueryInformationFile(FileModeInformation) error = %v", err)
	}
	if mode.Mode&windows.FILE_WRITE_THROUGH == 0 {
		t.Fatalf("file mode = 0x%x, want FILE_WRITE_THROUGH", mode.Mode)
	}
}

func TestDirectoryRejectsWindowsEscapingChildNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		child string
	}{
		{name: "Should reject a current drive root", child: `\`},
		{name: "Should reject a drive relative child", child: `C:`},
		{name: "Should reject an absolute drive child", child: `C:\`},
		{name: "Should reject a nested backslash child", child: `nested\child`},
		{name: "Should reject a nested slash child", child: `nested/child`},
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

func TestDirectoryOpensNormalWindowsChildDirectoriesAndRejectsWrongTypes(t *testing.T) {
	t.Parallel()

	t.Run("Should open a normal direct child directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "child"), 0o700); err != nil {
			t.Fatalf("os.Mkdir(child) error = %v", err)
		}
		directory, err := OpenDirectory(root)
		if err != nil {
			t.Fatalf("OpenDirectory(root) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})

		child, err := directory.OpenDirectory("child")
		if err != nil {
			t.Fatalf("Directory.OpenDirectory(child) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := child.Close(); closeErr != nil {
				t.Errorf("Directory.Close(child) error = %v", closeErr)
			}
		})
	})

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
				t.Errorf("Directory.Close(root) error = %v", closeErr)
			}
		})

		_, err = directory.OpenDirectory("regular.txt")
		if !errors.Is(err, ErrNotDirectory) {
			t.Fatalf("Directory.OpenDirectory(regular.txt) error = %v, want ErrNotDirectory", err)
		}
	})

	t.Run("Should obtain mutation rights for atomic publication", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "published.txt")
		parent, name, err := OpenParentDirectory(path)
		if err != nil {
			t.Fatalf("OpenParentDirectory() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := parent.Close(); closeErr != nil {
				t.Errorf("Directory.Close(parent) error = %v", closeErr)
			}
		})
		if err := parent.AtomicWriteFile(name, []byte("published"), 0o600, false); err != nil {
			t.Fatalf("Directory.AtomicWriteFile() error = %v", err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(published file) error = %v", err)
		}
		if string(contents) != "published" {
			t.Fatalf("published contents = %q, want published", contents)
		}
	})
}

func TestDirectoryRejectsWindowsReparsePointChildren(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a directory junction", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("os.Mkdir() error = %v", err)
		}
		junction := filepath.Join(root, "junction")
		createWindowsJunction(t, junction, target)

		directory, err := OpenDirectory(root)
		if err != nil {
			t.Fatalf("OpenDirectory() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := directory.Close(); closeErr != nil {
				t.Errorf("Directory.Close() error = %v", closeErr)
			}
		})

		_, err = directory.OpenDirectory("junction")
		if !errors.Is(err, ErrSymlink) {
			t.Fatalf("OpenDirectory(junction) error = %v, want ErrSymlink", err)
		}
	})

	t.Run("Should reject a leaf file symlink", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "target.txt")
		if err := os.WriteFile(target, []byte("outside root content"), 0o600); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
		link := filepath.Join(root, "linked.txt")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("os.Symlink() error = %v; configure the Windows test runner with symlink support", err)
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

		_, _, err = directory.ReadRegularFile("linked.txt")
		if !errors.Is(err, ErrSymlink) {
			t.Fatalf("ReadRegularFile(linked.txt) error = %v, want ErrSymlink", err)
		}
	})
}

func createWindowsJunction(t *testing.T, link string, target string) {
	t.Helper()

	command := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J %q %q error = %v, output = %s", link, target, err, output)
	}
}
