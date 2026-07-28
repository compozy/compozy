//go:build integration

package registry

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallerInstallPipelineWithInMemoryDownloader(t *testing.T) {
	t.Parallel()

	archive := mustTarGz(t, []tarEntry{
		{name: "extension/extension.toml", content: "name = \"pipeline-ext\"\nversion = \"3.4.5\"\n"},
		{name: "extension/assets/config.json", content: `{"ok":true}`},
		{name: "extension/bin/run.sh", content: "#!/bin/sh\necho pipeline\n"},
	})

	downloader := &stubDownloader{
		downloadFunc: func(context.Context, string, DownloadOpts) (*DownloadResult, error) {
			return &DownloadResult{
				Slug:        "acme/pipeline-ext",
				Version:     "3.4.5",
				ContentType: "application/gzip",
				Reader:      io.NopCloser(bytes.NewReader(archive)),
			}, nil
		},
	}

	targetDir := filepath.Join(t.TempDir(), "extensions", "pipeline-ext")
	result, err := NewInstaller(
		downloader,
	).Install(context.Background(), "acme/pipeline-ext", DownloadOpts{}, targetDir)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if result.Slug != "acme/pipeline-ext" {
		t.Fatalf("Install() slug = %q, want acme/pipeline-ext", result.Slug)
	}
	if result.Name != "pipeline-ext" {
		t.Fatalf("Install() name = %q, want pipeline-ext", result.Name)
	}
	if result.Version != "3.4.5" {
		t.Fatalf("Install() version = %q, want 3.4.5", result.Version)
	}

	checks := []string{
		filepath.Join(targetDir, installerExtensionManifestName),
		filepath.Join(targetDir, "assets", "config.json"),
		filepath.Join(targetDir, "bin", "run.sh"),
	}
	for _, path := range checks {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
	}
}
