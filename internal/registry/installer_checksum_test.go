package registry

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestComputeInstallChecksumRejectsLinksAndInvalidRoots(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a blank checksum root", func(t *testing.T) {
		t.Parallel()

		if _, err := computeInstallChecksumDirectory(nil); !errors.Is(err, ErrArchiveRootRequired) {
			t.Fatalf("computeInstallChecksumDirectory(nil) error = %v, want %v", err, ErrArchiveRootRequired)
		}
	})

	t.Run("Should reject a symlink in the checksum tree", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires platform-specific privileges on windows")
		}

		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "payload.txt"), strings.Repeat("first", 4096))
		if err := os.Symlink("payload.txt", filepath.Join(root, "current")); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}

		_, err := computeInstallChecksumDirectory(openArchiveTestRoot(t, root))
		if !errors.Is(err, ErrPathTraversesSymlink) {
			t.Fatalf("computeInstallChecksumDirectory(symlink) error = %v, want %v", err, ErrPathTraversesSymlink)
		}
	})
}

func TestComputeInstallChecksumChangesWhenRegularFileChanges(t *testing.T) {
	t.Parallel()

	t.Run("Should change when regular file content changes", func(t *testing.T) {
		t.Parallel()
		testComputeInstallChecksumChangesWhenRegularFileChanges(t)
	})
}

func testComputeInstallChecksumChangesWhenRegularFileChanges(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	payloadPath := filepath.Join(root, "payload.txt")
	writeTestFile(t, payloadPath, strings.Repeat("payload-", 8192))

	first, err := computeInstallChecksumDirectory(openArchiveTestRoot(t, root))
	if err != nil {
		t.Fatalf("computeInstallChecksum(first) error = %v", err)
	}

	writeTestFile(t, payloadPath, strings.Repeat("updated-", 8192))

	second, err := computeInstallChecksumDirectory(openArchiveTestRoot(t, root))
	if err != nil {
		t.Fatalf("computeInstallChecksum(second) error = %v", err)
	}
	if first == second {
		t.Fatalf("computeInstallChecksum() = %q after content change, want different checksum", second)
	}
}

func TestComputeInstallChecksumStableAcrossCreationOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should remain stable across creation order", func(t *testing.T) {
		t.Parallel()

		firstRoot := t.TempDir()
		secondRoot := t.TempDir()

		populate := func(root string, names []string) {
			t.Helper()
			for _, name := range names {
				writeTestFile(t, filepath.Join(root, name), "payload-"+name)
			}
		}

		populate(firstRoot, []string{
			filepath.Join("zeta", "three.txt"),
			filepath.Join("alpha", "one.txt"),
			filepath.Join("beta", "two.txt"),
		})
		populate(secondRoot, []string{
			filepath.Join("beta", "two.txt"),
			filepath.Join("zeta", "three.txt"),
			filepath.Join("alpha", "one.txt"),
		})

		first, err := computeInstallChecksumDirectory(openArchiveTestRoot(t, firstRoot))
		if err != nil {
			t.Fatalf("computeInstallChecksum(firstRoot) error = %v", err)
		}
		second, err := computeInstallChecksumDirectory(openArchiveTestRoot(t, secondRoot))
		if err != nil {
			t.Fatalf("computeInstallChecksum(secondRoot) error = %v", err)
		}
		if first != second {
			t.Fatalf(
				"computeInstallChecksum() = %q and %q for identical trees with different creation order, want stable checksum",
				first,
				second,
			)
		}
	})
}
