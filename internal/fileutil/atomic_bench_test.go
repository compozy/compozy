package fileutil

import (
	"bytes"
	"path/filepath"
	"testing"
)

func BenchmarkAtomicWriteFile1KiB(b *testing.B) {
	benchmarkAtomicWriteFile(b, 1<<10)
}

func BenchmarkAtomicWriteFile64KiB(b *testing.B) {
	benchmarkAtomicWriteFile(b, 64<<10)
}

func benchmarkAtomicWriteFile(b *testing.B, size int) {
	b.Helper()

	path := filepath.Join(b.TempDir(), "target.bin")
	payload := bytes.Repeat([]byte("a"), size)

	if err := AtomicWriteFile(path, payload, 0o600); err != nil {
		b.Fatalf("AtomicWriteFile() warmup error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := AtomicWriteFile(path, payload, 0o600); err != nil {
			b.Fatalf("AtomicWriteFile() error = %v", err)
		}
	}
}
