package version

import "testing"

func BenchmarkCurrent(b *testing.B) {
	for b.Loop() {
		_ = Current()
	}
}

func BenchmarkCurrentParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = Current()
		}
	})
}

func BenchmarkInfoString(b *testing.B) {
	info := Info{
		Version:   "1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-04-03T00:00:00Z",
	}

	for b.Loop() {
		_ = info.String()
	}
}
