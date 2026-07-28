package daytona

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func BenchmarkIOCopyLimitSlidingWindow(b *testing.B) {
	sourceData := bytes.Repeat([]byte("abcdefgh01234567"), 1<<17)
	limit := 1 << 20
	var mu sync.Mutex

	b.ReportAllocs()
	b.SetBytes(int64(limit))

	for b.Loop() {
		var dst bytes.Buffer
		session := newBenchmarkSession(sourceData)
		if err := ioCopyLimit(&dst, session, limit, &mu); err != nil {
			b.Fatalf("ioCopyLimit() error = %v", err)
		}
		if got := dst.Len(); got != limit {
			b.Fatalf("ioCopyLimit() len = %d, want %d", got, limit)
		}
	}
}

func BenchmarkWriteTarWorkspaceTree(b *testing.B) {
	root := b.TempDir()
	expectedFiles := createBenchmarkWorkspaceTree(b, root)
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		stats, err := writeTar(ctx, root, io.Discard, []string{"node_modules"})
		if err != nil {
			b.Fatalf("writeTar() error = %v", err)
		}
		if got := stats.Files; got != expectedFiles {
			b.Fatalf("writeTar() files = %d, want %d", got, expectedFiles)
		}
	}
}

func BenchmarkExtractTarWorkspaceTree(b *testing.B) {
	root := b.TempDir()
	expectedFiles := createBenchmarkWorkspaceTree(b, root)
	var archive bytes.Buffer
	stats, err := writeTar(context.Background(), root, &archive, nil)
	if err != nil {
		b.Fatalf("writeTar() setup error = %v", err)
	}
	if stats.Files != expectedFiles {
		b.Fatalf("writeTar() setup files = %d, want %d", stats.Files, expectedFiles)
	}

	baseDest := b.TempDir()
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		b.StopTimer()
		dest := filepath.Join(baseDest, fmt.Sprintf("run-%d", i))
		if err := os.MkdirAll(dest, 0o755); err != nil {
			b.Fatalf("MkdirAll(%q) error = %v", dest, err)
		}
		b.StartTimer()
		extracted, err := extractTar(dest, bytes.NewReader(archive.Bytes()))
		if err != nil {
			b.Fatalf("extractTar() error = %v", err)
		}
		if extracted.Files != expectedFiles {
			b.Fatalf("extractTar() files = %d, want %d", extracted.Files, expectedFiles)
		}
		b.StopTimer()
		if err := os.RemoveAll(dest); err != nil {
			b.Fatalf("RemoveAll(%q) error = %v", dest, err)
		}
	}
}

func createBenchmarkWorkspaceTree(tb testing.TB, root string) int {
	tb.Helper()

	dirs := []string{
		filepath.Join(root, "cmd"),
		filepath.Join(root, "configs"),
		filepath.Join(root, "docs"),
		filepath.Join(root, "nested", "level1"),
		filepath.Join(root, "nested", "level1", "level2"),
		filepath.Join(root, "node_modules", "ignored"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	entries := []struct {
		relative string
		content  []byte
	}{
		{relative: "README.md", content: bytes.Repeat([]byte("readme\n"), 64)},
		{relative: "cmd/agent.sh", content: bytes.Repeat([]byte("#!/bin/sh\necho hi\n"), 32)},
		{relative: "configs/runtime.toml", content: bytes.Repeat([]byte("mode=\"daytona\"\n"), 48)},
		{relative: "docs/notes.txt", content: bytes.Repeat([]byte("notes\n"), 96)},
		{relative: "nested/level1/app.log", content: bytes.Repeat([]byte("log-entry\n"), 128)},
		{relative: "nested/level1/level2/data.json", content: bytes.Repeat([]byte("{\"ok\":true}\n"), 128)},
		{relative: "node_modules/ignored/index.js", content: bytes.Repeat([]byte("console.log('skip')\n"), 64)},
	}

	count := 0
	for _, entry := range entries {
		path := filepath.Join(root, filepath.FromSlash(entry.relative))
		if err := os.WriteFile(path, entry.content, 0o600); err != nil {
			tb.Fatalf("WriteFile(%q) error = %v", path, err)
		}
		if entry.relative != "node_modules/ignored/index.js" {
			count++
		}
	}

	linkTarget := filepath.Join("..", "..", "README.md")
	linkPath := filepath.Join(root, "nested", "level1", "readme-link")
	if err := os.Symlink(linkTarget, linkPath); err != nil {
		tb.Fatalf("Symlink(%q -> %q) error = %v", linkPath, linkTarget, err)
	}

	return count
}

type benchmarkSession struct {
	reader *bytes.Reader
	done   chan struct{}
}

func newBenchmarkSession(data []byte) *benchmarkSession {
	done := make(chan struct{})
	close(done)
	return &benchmarkSession{
		reader: bytes.NewReader(data),
		done:   done,
	}
}

func (s *benchmarkSession) Read(p []byte) (int, error) { return s.reader.Read(p) }

func (*benchmarkSession) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func (*benchmarkSession) Close() error { return nil }

func (*benchmarkSession) CloseWrite() error { return nil }

func (s *benchmarkSession) Done() <-chan struct{} { return s.done }

func (*benchmarkSession) Wait() error { return nil }

func (*benchmarkSession) Stop(context.Context) error { return nil }

func (*benchmarkSession) Stderr() string { return "" }
