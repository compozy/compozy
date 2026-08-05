//go:build windows

package fileutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// Invariant: a write never crosses a Windows junction substituted for its parent.
// Owner: fileutil capability-rooted mutation boundary.
// Canonical suite: fileutil Windows no-follow behavior.
func TestAtomicWriteFileRejectsWindowsJunctionParent(t *testing.T) {
	t.Parallel()

	t.Run("Should leave the junction target unchanged", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		external := filepath.Join(root, "external")
		if err := os.Mkdir(external, 0o700); err != nil {
			t.Fatalf("os.Mkdir(external) error = %v", err)
		}
		sentinelPath := filepath.Join(external, "config.json")
		sentinel := []byte("external sentinel")
		if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
			t.Fatalf("os.WriteFile(external sentinel) error = %v", err)
		}
		junction := filepath.Join(root, "junction")
		createWindowsJunction(t, junction, external)

		err := AtomicWriteFile(filepath.Join(junction, "config.json"), []byte("attempted write"), 0o600)
		if !errors.Is(err, ErrSymlink) {
			t.Fatalf("AtomicWriteFile(junction target) error = %v, want ErrSymlink", err)
		}
		got, readErr := os.ReadFile(sentinelPath)
		if readErr != nil {
			t.Fatalf("os.ReadFile(external sentinel) error = %v", readErr)
		}
		if !bytes.Equal(got, sentinel) {
			t.Fatalf("external sentinel = %q, want %q", got, sentinel)
		}
	})
}

// Invariant: Windows directory publication preserves no-replace errors,
// rejects oversized names without panicking, and fully removes bound trees.
// Owner: fileutil Windows capability-rooted directory mutation boundary.
// Canonical suite: fileutil Windows rooted-write behavior.
func TestWindowsDirectoryMoveNoReplaceContract(t *testing.T) {
	t.Parallel()

	t.Run("Should report a typed collision and leave both trees unchanged", func(t *testing.T) {
		t.Parallel()

		rootPath := t.TempDir()
		for name, payload := range map[string]string{"source": "held", "target": "existing"} {
			path := filepath.Join(rootPath, name)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v", name, err)
			}
			if err := os.WriteFile(filepath.Join(path, "payload"), []byte(payload), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q payload) error = %v", name, err)
			}
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

		result, err := source.MoveTo(root, "target", false)
		if !result.Collision() || result.Committed() {
			t.Fatalf("Directory.MoveTo() result = %+v, want collision", result)
		}
		if !errors.Is(err, ErrTargetExists) {
			t.Fatalf("Directory.MoveTo() error = %v, want ErrTargetExists", err)
		}
		assertWindowsRegularFileContents(t, filepath.Join(rootPath, "source", "payload"), "held")
		assertWindowsRegularFileContents(t, filepath.Join(rootPath, "target", "payload"), "existing")
	})

	t.Run("Should return a path error for an oversized target without panicking", func(t *testing.T) {
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

		result, err := source.MoveTo(root, strings.Repeat("a", windows.MAX_LONG_PATH+1), false)
		if result.Committed() || result.Collision() {
			t.Fatalf("Directory.MoveTo() result = %+v, want pre-commit refusal", result)
		}
		if !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("Directory.MoveTo() error = %v, want ErrInvalidPath", err)
		}
	})

	t.Run("Should remove the committed bound tree without a private directory leak", func(t *testing.T) {
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

		result, err := source.MoveTo(root, "target", false)
		if err != nil || !result.Committed() || result.PostCommitErr != nil {
			t.Fatalf("Directory.MoveTo() result = %+v, error = %v", result, err)
		}
		if err := source.RemoveBoundTree(); err != nil {
			t.Fatalf("Directory.RemoveBoundTree() error = %v", err)
		}
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			t.Fatalf("os.ReadDir(root) error = %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("root entries after bound removal = %v, want none", entries)
		}
	})
}

func assertWindowsRegularFileContents(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %q = %q, want %q", path, got, want)
	}
}

// Invariant: a Windows directory move that already changed the canonical name
// reports every subsequent parent-sync failure without reclassifying the move.
// Owner: fileutil capability-rooted directory publication.
// Canonical suite: fileutil Windows rooted-write behavior.
func TestWindowsDirectoryMoveReportsPostCommitSyncFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve committed state and join both sync failures", func(t *testing.T) {
		t.Parallel()

		base := t.TempDir()
		sourceParentPath := filepath.Join(base, "staging")
		targetParentPath := filepath.Join(base, "installed")
		for _, path := range []string{sourceParentPath, targetParentPath} {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v", path, err)
			}
		}
		sourcePath := filepath.Join(sourceParentPath, "candidate")
		if err := os.Mkdir(sourcePath, 0o700); err != nil {
			t.Fatalf("os.Mkdir(candidate) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourcePath, "payload"), []byte("committed"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(payload) error = %v", err)
		}

		sourceParent, err := OpenDirectoryForMutation(sourceParentPath)
		if err != nil {
			t.Fatalf("OpenDirectoryForMutation(source parent) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := sourceParent.Close(); closeErr != nil {
				t.Errorf("Directory.Close(source parent) error = %v", closeErr)
			}
		})
		targetParent, err := OpenDirectoryForMutation(targetParentPath)
		if err != nil {
			t.Fatalf("OpenDirectoryForMutation(target parent) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := targetParent.Close(); closeErr != nil {
				t.Errorf("Directory.Close(target parent) error = %v", closeErr)
			}
		})
		source, err := sourceParent.OpenDirectoryForMove("candidate")
		if err != nil {
			t.Fatalf("OpenDirectoryForMove(candidate) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Directory.Close(candidate) error = %v", closeErr)
			}
		})

		sourceSyncErr := errors.New("source parent sync failed")
		targetSyncErr := errors.New("target parent sync failed")
		calls := 0
		result, err := moveBoundDirectoryWithSync(
			source,
			targetParent,
			"target",
			false,
			func(directory *Directory) error {
				calls++
				switch directory {
				case sourceParent:
					return sourceSyncErr
				case targetParent:
					return targetSyncErr
				default:
					t.Fatalf("sync directory = %p, want a move parent", directory)
					return nil
				}
			},
		)

		if err != nil {
			t.Fatalf("moveBoundDirectoryWithSync() error = %v", err)
		}
		if !result.Committed() || result.Collision() {
			t.Fatalf("moveBoundDirectoryWithSync() = %+v, want committed", result)
		}
		if !errors.Is(result.PostCommitErr, sourceSyncErr) {
			t.Fatalf("PostCommitErr = %v, want source sync failure", result.PostCommitErr)
		}
		if !errors.Is(result.PostCommitErr, targetSyncErr) {
			t.Fatalf("PostCommitErr = %v, want target sync failure", result.PostCommitErr)
		}
		if calls != 2 {
			t.Fatalf("sync calls = %d, want 2", calls)
		}
		payload, readErr := os.ReadFile(filepath.Join(targetParentPath, "target", "payload"))
		if readErr != nil {
			t.Fatalf("os.ReadFile(committed payload) error = %v", readErr)
		}
		if got := string(payload); got != "committed" {
			t.Fatalf("committed payload = %q, want committed", got)
		}
		if _, statErr := os.Stat(sourcePath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(source after move) error = %v, want os.ErrNotExist", statErr)
		}
	})
}

// Invariant: a rejected Windows directory-buffer flush is distinguishable
// from the already committed metadata operation and retains its OS cause.
// Owner: fileutil Windows directory durability boundary.
// Canonical suite: fileutil Windows rooted-write behavior.
func TestWindowsDirectorySyncReportsUnconfirmedDurability(t *testing.T) {
	t.Parallel()

	t.Run("Should retain the injected flush failure under the typed durability error", func(t *testing.T) {
		t.Parallel()

		flushErr := errors.New("flush rejected")
		calls := 0
		err := flushWindowsDirectoryHandle(windows.InvalidHandle, func(handle windows.Handle) error {
			calls++
			if handle != windows.InvalidHandle {
				t.Fatalf("flush handle = %v, want InvalidHandle", handle)
			}
			return flushErr
		})

		if !errors.Is(err, ErrDirectoryDurabilityUnconfirmed) {
			t.Fatalf("flushWindowsDirectoryHandle() error = %v, want ErrDirectoryDurabilityUnconfirmed", err)
		}
		if !errors.Is(err, flushErr) {
			t.Fatalf("flushWindowsDirectoryHandle() error = %v, want injected failure", err)
		}
		if calls != 1 {
			t.Fatalf("flush calls = %d, want 1", calls)
		}
	})

	t.Run("Should classify a real rejected flush instead of returning success", func(t *testing.T) {
		t.Parallel()

		directory, err := OpenDirectoryForMutation(t.TempDir())
		if err != nil {
			t.Fatalf("OpenDirectoryForMutation() error = %v", err)
		}
		closedFile := directory.file
		if err := directory.Close(); err != nil {
			t.Fatalf("Directory.Close() error = %v", err)
		}

		err = syncDirectoryHandle(closedFile)
		if !errors.Is(err, ErrDirectoryDurabilityUnconfirmed) {
			t.Fatalf("syncDirectoryHandle(closed file) error = %v, want ErrDirectoryDurabilityUnconfirmed", err)
		}
		if !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
			t.Fatalf("syncDirectoryHandle(closed file) error = %v, want ERROR_INVALID_HANDLE", err)
		}
	})
}
