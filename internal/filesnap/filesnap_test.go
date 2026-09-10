package filesnap

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFromPath(t *testing.T) {
	t.Parallel()

	t.Run("Should read a valid file snapshot", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "demo.txt")
		if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}

		snapshot, err := FromPath(path)
		if err != nil {
			t.Fatalf("FromPath() error = %v", err)
		}
		if snapshot.Size != int64(len("hello")) {
			t.Fatalf("FromPath().Size = %d, want %d", snapshot.Size, len("hello"))
		}
		if snapshot.ModTime.IsZero() {
			t.Fatal("FromPath().ModTime = zero, want populated")
		}
	})

	t.Run("Should return os.ErrNotExist for a missing file", func(t *testing.T) {
		t.Parallel()

		_, err := FromPath(filepath.Join(t.TempDir(), "missing.txt"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("FromPath(missing) error = %v, want os.ErrNotExist", err)
		}
	})
}

func TestEqual(t *testing.T) {
	t.Parallel()

	modTime := time.Date(2026, 4, 6, 22, 30, 0, 0, time.UTC)

	t.Run("Should report equal maps as equal", func(t *testing.T) {
		t.Parallel()

		left := equalTestSnapshots(modTime)
		if !Equal(left, Clone(left)) {
			t.Fatal("Equal(clone) = false, want true")
		}
	})

	t.Run("Should reject maps with different sizes", func(t *testing.T) {
		t.Parallel()

		left := equalTestSnapshots(modTime)
		if Equal(left, map[string]Snapshot{"a": left["a"]}) {
			t.Fatal("Equal(different sizes) = true, want false")
		}
	})

	t.Run("Should reject maps with different keys", func(t *testing.T) {
		t.Parallel()

		left := equalTestSnapshots(modTime)
		right := map[string]Snapshot{
			"b": left["b"],
			"c": left["a"],
		}
		if Equal(left, right) {
			t.Fatal("Equal(different keys) = true, want false")
		}
	})

	t.Run("Should reject maps with different snapshot values", func(t *testing.T) {
		t.Parallel()

		left := equalTestSnapshots(modTime)
		right := Clone(left)
		right["b"] = Snapshot{ModTime: modTime.Add(2 * time.Second), Size: 2}
		if Equal(left, right) {
			t.Fatal("Equal(different values) = true, want false")
		}
	})
}

func TestFromInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should fingerprint file contents when change metadata is unavailable", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "definition.md")
		if err := os.WriteFile(path, []byte("alpha"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		metadata := withoutSystemMetadata{info}
		before := FromInfo(path, metadata)
		if !before.Equal(FromInfo(path, metadata)) {
			t.Fatal("unchanged readable content must produce equal snapshots")
		}
		if err := os.WriteFile(path, []byte("bravo"), info.Mode()); err != nil {
			t.Fatal(err)
		}
		if before.Equal(FromInfo(path, metadata)) {
			t.Fatal("different file bytes with identical supplied metadata must invalidate the snapshot")
		}
	})

	for _, directory := range []bool{false, true} {
		name := "Should fingerprint directory entry names when change metadata is unavailable"
		if directory {
			name = "Should fingerprint directory entry types when change metadata is unavailable"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			child := filepath.Join(root, "original")
			if err := os.WriteFile(child, []byte("content"), 0o644); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata := withoutSystemMetadata{info}
			before := FromInfo(root, metadata)
			if !before.Equal(FromInfo(root, metadata)) {
				t.Fatal("unchanged directory entries must produce equal snapshots")
			}
			if directory {
				if err := os.Remove(child); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(child, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Rename(child, filepath.Join(root, "renamed")); err != nil {
				t.Fatal(err)
			}
			if before.Equal(FromInfo(root, metadata)) {
				t.Fatal("changed directory membership or type must invalidate the snapshot")
			}
		})
	}

	t.Run("Should fingerprint symbolic link targets when change metadata is unavailable", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "linked")
		if err := os.Symlink("original", path); err != nil {
			t.Skipf("Symlink unavailable: %v", err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		metadata := withoutSystemMetadata{info}
		before := FromInfo(path, metadata)
		if !before.Equal(FromInfo(path, metadata)) {
			t.Fatal("unchanged symbolic link target must produce equal snapshots")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("retarget", path); err != nil {
			t.Fatal(err)
		}
		if before.Equal(FromInfo(path, metadata)) {
			t.Fatal("a different symbolic link target must invalidate the snapshot")
		}
	})

	t.Run("Should keep unreadable fallback content uncacheable", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "definition.md")
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		snapshot := FromInfo(path, withoutSystemMetadata{info})
		if snapshot.Equal(snapshot) {
			t.Fatal("a failed fallback read must never authorize snapshot reuse")
		}
	})
}

type withoutSystemMetadata struct {
	fs.FileInfo
}

func (withoutSystemMetadata) Sys() any {
	return nil
}

func TestCloneReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	modTime := time.Date(2026, 4, 6, 22, 45, 0, 0, time.UTC)

	t.Run("Should return an independent copy", func(t *testing.T) {
		t.Parallel()

		original := map[string]Snapshot{
			"skill.md": {ModTime: modTime, Size: 10},
		}

		cloned := Clone(original)
		cloned["skill.md"] = Snapshot{ModTime: modTime.Add(time.Minute), Size: 42}

		if original["skill.md"].Size != 10 {
			t.Fatalf("original snapshot size = %d, want 10", original["skill.md"].Size)
		}
		if original["skill.md"].ModTime != modTime {
			t.Fatalf("original snapshot mod time = %v, want %v", original["skill.md"].ModTime, modTime)
		}
	})

	t.Run("Should return an empty map for nil input", func(t *testing.T) {
		t.Parallel()

		if got := Clone(nil); got == nil || len(got) != 0 {
			t.Fatalf("Clone(nil) = %#v, want empty map", got)
		}
	})
}

func equalTestSnapshots(modTime time.Time) map[string]Snapshot {
	return map[string]Snapshot{
		"a": {ModTime: modTime, Size: 1},
		"b": {ModTime: modTime.Add(time.Second), Size: 2},
	}
}
