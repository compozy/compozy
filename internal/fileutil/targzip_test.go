package fileutil

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTarGzipDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should produce deterministic output and report accepted resources", func(t *testing.T) {
		t.Parallel()

		root := writeTarGzipFixture(t)
		first, firstStats := writeTarGzipFixtureArchive(t, root, TarGzipLimits{})
		second, secondStats := writeTarGzipFixtureArchive(t, root, TarGzipLimits{})
		if !bytes.Equal(first, second) {
			t.Fatal("WriteTarGzipDirectory() output is not deterministic")
		}
		if firstStats != secondStats || firstStats.FileCount != 3 {
			t.Fatalf("WriteTarGzipDirectory() stats = %#v and %#v, want 3 entries", firstStats, secondStats)
		}
		if firstStats.CompressedSize != int64(len(first)) || firstStats.UncompressedSize <= 0 {
			t.Fatalf("WriteTarGzipDirectory() stats = %#v, archive bytes = %d", firstStats, len(first))
		}
	})

	t.Run("Should stop when the file count budget is exhausted", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer
		_, err := WriteTarGzipDirectory(
			t.Context(),
			&archive,
			writeTarGzipFixture(t),
			nil,
			TarGzipLimits{MaxFileCount: 2},
		)
		if !errors.Is(err, ErrTarGzipFileCountLimit) {
			t.Fatalf("WriteTarGzipDirectory() error = %v, want ErrTarGzipFileCountLimit", err)
		}
	})

	t.Run("Should stop while writing the uncompressed tar stream", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer
		stats, err := WriteTarGzipDirectory(
			t.Context(),
			&archive,
			writeTarGzipFixture(t),
			nil,
			TarGzipLimits{MaxUncompressedSize: 128},
		)
		if !errors.Is(err, ErrTarGzipUncompressedLimit) {
			t.Fatalf("WriteTarGzipDirectory() error = %v, want ErrTarGzipUncompressedLimit", err)
		}
		if stats.UncompressedSize > 128 {
			t.Fatalf("UncompressedSize = %d, want at most 128", stats.UncompressedSize)
		}
	})

	t.Run("Should stop while writing the compressed stream", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer
		stats, err := WriteTarGzipDirectory(
			t.Context(),
			&archive,
			writeTarGzipFixture(t),
			nil,
			TarGzipLimits{MaxCompressedSize: 16},
		)
		if !errors.Is(err, ErrTarGzipCompressedLimit) {
			t.Fatalf("WriteTarGzipDirectory() error = %v, want ErrTarGzipCompressedLimit", err)
		}
		if stats.CompressedSize > 16 || archive.Len() > 16 {
			t.Fatalf("compressed bytes = %d, stats = %#v, want at most 16", archive.Len(), stats)
		}
	})

	t.Run("Should honor cancellation before traversing the directory", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var archive bytes.Buffer
		_, err := WriteTarGzipDirectory(ctx, &archive, writeTarGzipFixture(t), nil, TarGzipLimits{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WriteTarGzipDirectory() error = %v, want context.Canceled", err)
		}
	})
}

func writeTarGzipFixtureArchive(t *testing.T, root string, limits TarGzipLimits) ([]byte, TarGzipStats) {
	t.Helper()

	var archive bytes.Buffer
	stats, err := WriteTarGzipDirectory(t.Context(), &archive, root, nil, limits)
	if err != nil {
		t.Fatalf("WriteTarGzipDirectory() error = %v", err)
	}
	return archive.Bytes(), stats
}

func writeTarGzipFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatalf("os.Mkdir(child) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(a.txt) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(child, "b.txt"), []byte("beta"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(b.txt) error = %v", err)
	}
	return root
}
