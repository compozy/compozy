package filesnap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkFromPath(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "snapshot.txt")
	if err := os.WriteFile(path, []byte("benchmark payload"), 0o644); err != nil {
		b.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := FromPath(path); err != nil {
			b.Fatalf("FromPath(%q) error = %v", path, err)
		}
	}
}

func BenchmarkEqual(b *testing.B) {
	left := benchmarkSnapshots(32)
	right := Clone(left)

	b.ReportAllocs()

	for b.Loop() {
		if !Equal(left, right) {
			b.Fatal("Equal(left, right) = false, want true")
		}
	}
}

func BenchmarkClone(b *testing.B) {
	src := benchmarkSnapshots(32)

	b.ReportAllocs()

	for b.Loop() {
		cloned := Clone(src)
		if len(cloned) != len(src) {
			b.Fatalf("len(Clone(src)) = %d, want %d", len(cloned), len(src))
		}
	}
}

func benchmarkSnapshots(count int) map[string]Snapshot {
	baseTime := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	snapshots := make(map[string]Snapshot, count)
	for i := range count {
		snapshots[fmt.Sprintf("path-%02d", i)] = Snapshot{
			ModTime: baseTime.Add(time.Duration(i) * time.Second),
			Size:    int64(i + 1),
		}
	}
	return snapshots
}
