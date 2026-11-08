package fileutil

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteTarDirectory(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a different root identity before writing any archive bytes", func(t *testing.T) {
		t.Parallel()
		root := writeTarGzipFixture(t)
		directory, err := OpenDirectory(root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := directory.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := os.Rename(root, root+"-held"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(root + "-held"); err != nil {
				t.Error(err)
			}
		})
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
		var archive bytes.Buffer
		stats, err := directory.WriteMaterializedTar(t.Context(), &archive, root, nil, TarLimits{})
		if !errors.Is(err, ErrSymlink) || archive.Len() != 0 || stats != (TarStats{}) {
			t.Fatalf("replaced root = %+v, %v, bytes=%d", stats, err, archive.Len())
		}
	})
	t.Run("Should share canonical package bytes with gzip archives across timestamp changes", func(t *testing.T) {
		t.Parallel()
		root := writeTarGzipFixture(t)
		compressed, compressedStats := writeTarGzipFixtureArchive(t, root, TarGzipLimits{})
		second, secondStats := writeTarGzipFixtureArchive(t, root, TarGzipLimits{})
		if !bytes.Equal(compressed, second) {
			t.Fatal("WriteTarGzipDirectory() output is not deterministic")
		}
		if compressedStats != secondStats || compressedStats.FileCount != 3 {
			t.Fatalf("WriteTarGzipDirectory() stats = %#v and %#v, want 3 entries", compressedStats, secondStats)
		}
		if compressedStats.CompressedSize != int64(len(compressed)) || compressedStats.UncompressedSize <= 0 {
			t.Fatalf("WriteTarGzipDirectory() stats = %#v, archive bytes = %d", compressedStats, len(compressed))
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			t.Fatal(err)
		}
		want, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read gzip envelope: %v, %v", readErr, closeErr)
		}
		stamp := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(filepath.Join(root, "a.txt"), stamp, stamp); err != nil {
			t.Fatal(err)
		}
		var archive bytes.Buffer
		stats, err := WriteTarDirectory(t.Context(), &archive, root, nil, TarLimits{})
		if err != nil || !bytes.Equal(archive.Bytes(), want) || stats.Bytes != compressedStats.UncompressedSize ||
			stats.FileCount != compressedStats.FileCount {
			t.Fatalf("canonical tar changed: stats %+v, gzip stats %+v, error %v", stats, compressedStats, err)
		}

		var materialized bytes.Buffer
		copied, err := WriteTarDirectory(t.Context(), &materialized, root, nil, TarLimits{MaterializeSymlinks: true})
		if err != nil || !bytes.Equal(materialized.Bytes(), want) || copied != stats {
			t.Fatalf("materialized canonical tar changed: stats %+v, want %+v, error %v", copied, stats, err)
		}
	})
	t.Run("Should refuse a file swapped for an external symlink before its bytes are read", func(t *testing.T) {
		t.Parallel()
		root := writeTarGzipFixture(t)
		outside := filepath.Join(t.TempDir(), "private.txt")
		if err := os.WriteFile(outside, []byte("leak!"), 0o600); err != nil {
			t.Fatal(err)
		}
		probe := filepath.Join(root, "symlink-capability-probe")
		if err := os.Symlink(outside, probe); err != nil {
			t.Skipf("Symlink(external replacement) unavailable: %v", err)
		}
		if err := os.Remove(probe); err != nil {
			t.Fatal(err)
		}
		var archive bytes.Buffer
		swapped := false
		writer := archiveWriteFunc(func(raw []byte) (int, error) {
			if !swapped {
				swapped = true
				path := filepath.Join(root, "a.txt")
				if err := os.Remove(path); err != nil {
					return 0, err
				}
				if err := os.Symlink(outside, path); err != nil {
					return 0, err
				}
			}
			return archive.Write(raw)
		})
		_, err := WriteTarDirectory(t.Context(), writer, root, nil, TarLimits{})
		if !errors.Is(err, ErrSymlink) || bytes.Contains(archive.Bytes(), []byte("leak!")) {
			t.Fatalf(
				"symlink replacement = %v, external bytes archived: %t",
				err,
				bytes.Contains(archive.Bytes(), []byte("leak!")),
			)
		}
	})
}

type archiveWriteFunc func([]byte) (int, error)

func (f archiveWriteFunc) Write(raw []byte) (int, error) { return f(raw) }

func TestWriteTarGzipDirectory(t *testing.T) {
	t.Parallel()

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
		if !errors.Is(err, ErrTarFileCountLimit) {
			t.Fatalf("WriteTarGzipDirectory() error = %v, want ErrTarFileCountLimit", err)
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
		if !errors.Is(err, ErrTarSizeLimit) {
			t.Fatalf("WriteTarGzipDirectory() error = %v, want ErrTarSizeLimit", err)
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

func TestMaterializedTarLimits(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name   string
		limits TarLimits
		want   error
	}{
		{name: "Should count copied directory nodes", limits: TarLimits{MaxFileCount: 3, MaterializeSymlinks: true}, want: ErrTarFileCountLimit},
		{name: "Should count copied file bytes", limits: TarLimits{MaxBytes: 7000, MaterializeSymlinks: true}, want: ErrTarSizeLimit},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, "data"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(root, "data", "file"),
				bytes.Repeat([]byte("x"), 4096),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("data", filepath.Join(root, "alias")); err != nil {
				t.Fatal(err)
			}
			_, err := WriteTarDirectory(t.Context(), io.Discard, root, nil, fixture.limits)
			if !errors.Is(err, fixture.want) {
				t.Fatalf("materialized quota = %v, want %v", err, fixture.want)
			}
		})
	}
}
