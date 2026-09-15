//go:build windows

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Invariant: private I/O uses the held object's owner and ACL, never synthetic POSIX bits.
func TestWindowsPrivateFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, sddl, wantError string
		allowed               bool
	}{
		{name: "Should accept owner-only access", sddl: "D:P(A;;FA;;;%s)", allowed: true},
		{name: "Should accept trusted system administrators", sddl: "D:P(A;;FA;;;%s)(A;;FA;;;SY)(A;;FA;;;BA)", allowed: true},
		{name: "Should reject public read access", sddl: "D:P(A;;FA;;;%s)(A;;FR;;;WD)", wantError: "private file ACL must restrict access"},
		{name: "Should reject public write access", sddl: "D:P(A;;FA;;;%s)(A;;FW;;;WD)", wantError: "private file ACL must restrict access"},
		{name: "Should reject a null DACL", sddl: "D:NO_ACCESS_CONTROL", wantError: "private file requires a non-null DACL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "secret")
			if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
				t.Fatal(err)
			}
			operator, err := windows.GetCurrentProcessToken().GetTokenUser()
			if err != nil {
				t.Fatal(err)
			}
			sddl := tc.sddl
			if strings.Contains(sddl, "%s") {
				sddl = fmt.Sprintf(sddl, operator.User.Sid.String())
			}
			security, err := windows.SecurityDescriptorFromString(sddl)
			if err != nil {
				t.Fatal(err)
			}
			dacl, _, err := security.DACL()
			if err != nil {
				t.Fatal(err)
			}
			if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
				windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
				nil, nil, dacl, nil); err != nil {
				t.Fatal(err)
			}
			got, err := ReadPrivateFile(path)
			if tc.allowed {
				if err != nil || string(got) != "private" {
					t.Fatalf("read = %q, %v", got, err)
				}
				if err := os.Chmod(path, 0o400); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(path, 0o600); err != nil {
						t.Error(err)
					}
				})
				if _, err := ReadPrivateFile(path); err != nil {
					t.Fatalf("read-only attribute changes no ACL: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || got != nil {
					t.Fatalf("unsafe read = %q, %v; want %q", got, err, tc.wantError)
				}
				before, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
					windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
				if err != nil {
					t.Fatal(err)
				}
				if err := AtomicWritePrivateFile(
					path,
					[]byte("replacement"),
				); err == nil ||
					!strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("unsafe replacement = %v; want %q", err, tc.wantError)
				}
				preserved, err := os.ReadFile(path)
				if err != nil || string(preserved) != "private" {
					t.Fatalf("preserved = %q, %v", preserved, err)
				}
				after, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
					windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
				if err != nil {
					t.Fatal(err)
				}
				if before.String() != after.String() {
					t.Fatal("replacement changed existing security descriptor")
				}
			}
		})
	}
	t.Run("Should refuse publication into a public directory without changing existing bytes", func(t *testing.T) {
		t.Parallel()
		directory := t.TempDir()
		path := filepath.Join(directory, "secret")
		if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command("icacls", directory, "/grant", "*S-1-1-0:(OI)(CI)F").
			CombinedOutput(); err != nil {
			t.Fatalf("grant: %v: %s", err, output)
		}
		if err := AtomicWritePrivateFile(
			path,
			[]byte("replacement"),
		); err == nil ||
			!strings.Contains(err.Error(), "private file ACL must restrict access") {
			t.Fatalf("expected private directory ACL rejection, got %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "original" {
			t.Fatalf("preserved = %q, %v", got, err)
		}
	})
}
