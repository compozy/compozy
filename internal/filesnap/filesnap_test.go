package filesnap

import (
	"errors"
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
