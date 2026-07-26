package daytona

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarRejectsUnsafeEntries(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header tar.Header
		body   string
	}{
		{
			name:   "absolute path",
			header: tar.Header{Name: "/tmp/evil", Mode: 0o600, Size: 1},
			body:   "x",
		},
		{
			name:   "parent traversal",
			header: tar.Header{Name: "../evil", Mode: 0o600, Size: 1},
			body:   "x",
		},
		{
			name:   "symlink escape",
			header: tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../outside"},
		},
		{
			name:   "unsupported mode",
			header: tar.Header{Name: "device", Typeflag: tar.TypeChar},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			archive := tarArchiveWithHeader(t, tc.header, tc.body)
			if _, err := extractTar(t.TempDir(), bytes.NewReader(archive)); err == nil {
				t.Fatal("extractTar() error = nil, want unsafe entry rejection")
			}
		})
	}
}

func TestExtractTarRejectsExistingSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	archive := makeTar(t, map[string]string{"link/escape.txt": "bad"})
	if _, err := extractTar(root, bytes.NewReader(archive)); err == nil {
		t.Fatal("extractTar() error = nil, want symlink parent rejection")
	}
}

func TestWriteAndExtractTarRoundTripWithSymlinkAndExclusions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "content")
	writeTestFile(t, filepath.Join(root, "node_modules", "ignored.txt"), "ignored")
	if err := os.Symlink("file.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	var archive bytes.Buffer
	stats, err := writeTar(context.Background(), root, &archive, nil)
	if err != nil {
		t.Fatalf("writeTar() error = %v", err)
	}
	if stats.Files != 1 || stats.Bytes != int64(len("content")) {
		t.Fatalf("writeTar() stats = %+v, want one regular file", stats)
	}

	dest := t.TempDir()
	extracted, err := extractTar(dest, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatalf("extractTar() error = %v", err)
	}
	if extracted.Files != 1 || extracted.Bytes != int64(len("content")) {
		t.Fatalf("extractTar() stats = %+v, want one regular file", extracted)
	}
	assertFileContent(t, filepath.Join(dest, "file.txt"), "content")
	if _, err := os.Stat(filepath.Join(dest, "node_modules", "ignored.txt")); !os.IsNotExist(err) {
		t.Fatalf("excluded file exists or stat returned unexpected error: %v", err)
	}
	target, err := os.Readlink(filepath.Join(dest, "link.txt"))
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if target != "file.txt" {
		t.Fatalf("Readlink() = %q, want file.txt", target)
	}
}

func TestExtractTarIgnoresRootEntry(t *testing.T) {
	t.Parallel()

	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	if err := writer.WriteHeader(&tar.Header{Name: ".", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatalf("WriteHeader(root) error = %v", err)
	}
	body := "content"
	if err := writer.WriteHeader(&tar.Header{Name: "./file.txt", Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatalf("WriteHeader(file) error = %v", err)
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatalf("Write(file) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	dest := t.TempDir()
	stats, err := extractTar(dest, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatalf("extractTar() error = %v", err)
	}
	if stats.Files != 1 || stats.Bytes != int64(len(body)) {
		t.Fatalf("extractTar() stats = %+v, want one regular file", stats)
	}
	assertFileContent(t, filepath.Join(dest, "file.txt"), body)
}

func tarArchiveWithHeader(t *testing.T, header tar.Header, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	if header.Size == 0 && body != "" {
		header.Size = int64(len(body))
	}
	if err := writer.WriteHeader(&header); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if body != "" {
		if _, err := writer.Write([]byte(body)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}
