package registry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerInstallPreservesExistingTargetWhenChecksumFails(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the existing package when replacement checksum fails", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("POSIX owner read bits are required to make the checksum open fail deterministically")
		}

		archive := mustTarGz(t, []tarEntry{
			{name: "extension/extension.toml", content: "name = \"demo-ext\"\nversion = \"2.0.0\"\n"},
			{name: "extension/bin/unreadable", content: "replacement", mode: 0o111},
		})
		downloader := &stubDownloader{
			downloadFunc: func(context.Context, string, DownloadOpts) (*DownloadResult, error) {
				return &DownloadResult{
					Slug:        "acme/demo-ext",
					Version:     "2.0.0",
					ContentType: "application/gzip",
					Reader:      io.NopCloser(bytes.NewReader(archive)),
				}, nil
			},
		}

		targetDir := filepath.Join(t.TempDir(), "extensions", "demo-ext")
		writeTestFile(t, filepath.Join(targetDir, "extension.toml"), "name = \"demo-ext\"\nversion = \"1.0.0\"\n")
		writeTestFile(t, filepath.Join(targetDir, "README.md"), "existing package content")

		_, err := NewInstaller(downloader).Install(context.Background(), "acme/demo-ext", DownloadOpts{}, targetDir)
		if err == nil {
			t.Fatal("Install() error = nil, want checksum failure")
		}
		if !strings.Contains(err.Error(), "open checksum path") {
			t.Fatalf("Install() error = %v, want checksum open failure", err)
		}

		manifest, readErr := os.ReadFile(filepath.Join(targetDir, "extension.toml"))
		if readErr != nil {
			t.Fatalf("ReadFile(existing manifest) error = %v", readErr)
		}
		if got, want := string(manifest), "name = \"demo-ext\"\nversion = \"1.0.0\"\n"; got != want {
			t.Fatalf("existing manifest = %q, want %q", got, want)
		}
		content, readErr := os.ReadFile(filepath.Join(targetDir, "README.md"))
		if readErr != nil {
			t.Fatalf("ReadFile(existing content) error = %v", readErr)
		}
		if got, want := string(content), "existing package content"; got != want {
			t.Fatalf("existing content = %q, want %q", got, want)
		}
		if _, statErr := os.Stat(filepath.Join(targetDir, "bin", "unreadable")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(failed replacement file) error = %v, want os.ErrNotExist", statErr)
		}
	})
}
