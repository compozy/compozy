package extension

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type archiveEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
}

func TestNormalizeInstallSourceOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     installSourceOptions
		want      installSourceOptions
		wantError string
	}{
		{
			name:  "Local defaults",
			input: installSourceOptions{},
			want:  installSourceOptions{Remote: installRemoteLocal},
		},
		{
			name:      "Local rejects ref",
			input:     installSourceOptions{Remote: installRemoteLocal, Ref: "main"},
			wantError: "--ref is only supported with --remote github",
		},
		{
			name:      "GitHub requires ref",
			input:     installSourceOptions{Remote: installRemoteGitHub},
			wantError: "--ref is required with --remote github",
		},
		{
			name: "GitHub accepts cleaned subdir",
			input: installSourceOptions{
				Remote: installRemoteGitHub,
				Ref:    "v1.2.3",
				Subdir: "extensions/cy-idea-factory/.",
			},
			want: installSourceOptions{
				Remote: installRemoteGitHub,
				Ref:    "v1.2.3",
				Subdir: "extensions/cy-idea-factory",
			},
		},
		{
			name:      "GitHub rejects escaping subdir",
			input:     installSourceOptions{Remote: installRemoteGitHub, Ref: "v1.2.3", Subdir: "../bad"},
			wantError: "--subdir must not escape the repository root",
		},
		{
			name:      "GitHub rejects backslash subdir",
			input:     installSourceOptions{Remote: installRemoteGitHub, Ref: "v1.2.3", Subdir: `..\\..\\otherdir`},
			wantError: "--subdir must use forward slashes relative to the repository root",
		},
		{
			name:      "GitHub rejects drive-letter subdir",
			input:     installSourceOptions{Remote: installRemoteGitHub, Ref: "v1.2.3", Subdir: "C:/tmp/ext"},
			wantError: "--subdir must be relative to the repository root",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeInstallSourceOptions(tt.input)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("normalizeInstallSourceOptions() error = %v, want substring %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeInstallSourceOptions() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected normalized options\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}

func TestResolveInstallSourceWithFetcherLocal(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	got, err := resolveInstallSourceWithFetcher(
		context.Background(),
		sourceDir,
		installSourceOptions{},
		installSourceFetcher{
			now: func() time.Time {
				return time.Date(2026, time.April, 13, 10, 0, 0, 0, time.UTC)
			},
		},
	)
	if err != nil {
		t.Fatalf("resolveInstallSourceWithFetcher() error = %v", err)
	}
	if got.SourcePath != sourceDir {
		t.Fatalf("unexpected local source path\nwant: %s\ngot:  %s", sourceDir, got.SourcePath)
	}
	if got.InstallOrigin == nil || got.InstallOrigin.Remote != string(installRemoteLocal) {
		t.Fatalf("expected local provenance, got %#v", got.InstallOrigin)
	}
}

func TestResolveInstallSourceWithFetcherGitHub(t *testing.T) {
	t.Parallel()

	archive := buildTarGz(t, []archiveEntry{
		{name: "compozy-v1.2.3/", typeflag: tar.TypeDir},
		{
			name:     "compozy-v1.2.3/.compozy/tasks/_archived/20260410-192523-refac/_techspec.md",
			typeflag: tar.TypeSymlink,
			linkname: "20260406-summary.md",
		},
		{name: "compozy-v1.2.3/extensions/cy-idea-factory/", typeflag: tar.TypeDir},
		{
			name: "compozy-v1.2.3/extensions/cy-idea-factory/extension.toml",
			body: `[extension]
name = "cy-idea-factory"
version = "1.0.0"
description = "Idea factory"
min_compozy_version = "0.0.1"

[security]
capabilities = ["skills.ship", "agents.ship"]

[resources]
skills = ["skills/*"]
agents = ["agents/*"]
`,
			typeflag: tar.TypeReg,
		},
	})

	errCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/compozy/compozy/tar.gz/v1.2.3"; got != want {
			errCh <- fmt.Errorf("unexpected archive path\nwant: %s\ngot:  %s", want, got)
			http.Error(w, "unexpected archive path", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	fetcher := installSourceFetcher{
		httpClient:         server.Client(),
		githubArchiveURL:   server.URL,
		createTempDir:      os.MkdirTemp,
		removeAll:          os.RemoveAll,
		now:                func() time.Time { return time.Date(2026, time.April, 13, 12, 0, 0, 0, time.UTC) },
		maxDownloadBytes:   1 << 20,
		maxExtractionBytes: 1 << 20,
	}

	got, err := resolveInstallSourceWithFetcher(
		context.Background(),
		"compozy/compozy",
		installSourceOptions{Remote: installRemoteGitHub, Ref: "v1.2.3", Subdir: "extensions/cy-idea-factory"},
		fetcher,
	)
	if err != nil {
		t.Fatalf("resolveInstallSourceWithFetcher() error = %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatal(err)
	default:
	}
	if !strings.HasSuffix(got.SourcePath, filepath.Join("extensions", "cy-idea-factory")) {
		t.Fatalf("unexpected github source path: %s", got.SourcePath)
	}
	if got.InstallOrigin == nil {
		t.Fatal("expected github provenance")
	}
	if got.InstallOrigin.Repository != "compozy/compozy" || got.InstallOrigin.Ref != "v1.2.3" {
		t.Fatalf("unexpected github provenance: %#v", got.InstallOrigin)
	}
	if got.CleanupSource == nil {
		t.Fatal("expected cleanup callback for extracted github source")
	}

	rootToRemove := strings.TrimSuffix(got.SourcePath, filepath.Join("extensions", "cy-idea-factory"))
	if err := got.CleanupSource(); err != nil {
		t.Fatalf("CleanupSource() error = %v", err)
	}
	if _, err := os.Stat(rootToRemove); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup to remove staging root, stat err = %v", err)
	}
}

func TestExtractTarGzArchiveRejectsTraversal(t *testing.T) {
	t.Parallel()

	archive := buildTarGz(t, []archiveEntry{
		{name: "../escape/extension.toml", body: "bad", typeflag: tar.TypeReg},
	})
	if _, err := extractTarGzArchive(bytes.NewReader(archive), t.TempDir(), 1<<20, ""); err == nil {
		t.Fatal("expected traversal archive to fail")
	}
}

func TestExtractTarGzArchiveRejectsSymlink(t *testing.T) {
	t.Parallel()

	archive := buildTarGz(t, []archiveEntry{
		{name: "repo/", typeflag: tar.TypeDir},
		{name: "repo/extension.toml", body: "ok", typeflag: tar.TypeReg},
		{name: "repo/link", typeflag: tar.TypeSymlink, linkname: "../outside"},
	})
	if _, err := extractTarGzArchive(bytes.NewReader(archive), t.TempDir(), 1<<20, ""); err == nil {
		t.Fatal("expected symlink archive to fail")
	}
}

func TestExtractTarGzArchiveIgnoresSymlinkOutsideIncludedSubdir(t *testing.T) {
	t.Parallel()

	destRoot := t.TempDir()
	archive := buildTarGz(t, []archiveEntry{
		{name: "repo/", typeflag: tar.TypeDir},
		{
			name:     "repo/.compozy/tasks/_archived/20260410-192523-refac/_techspec.md",
			typeflag: tar.TypeSymlink,
			linkname: "20260406-summary.md",
		},
		{name: "repo/extensions/cy-idea-factory/extension.toml", body: "ok", typeflag: tar.TypeReg},
	})

	extractedRoot, err := extractTarGzArchive(
		bytes.NewReader(archive),
		destRoot,
		1<<20,
		"extensions/cy-idea-factory",
	)
	if err != nil {
		t.Fatalf("extractTarGzArchive() error = %v", err)
	}

	manifestPath := filepath.Join(extractedRoot, "extensions", "cy-idea-factory", "extension.toml")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected extracted manifest at %q: %v", manifestPath, err)
	}
	if _, err := os.Lstat(filepath.Join(extractedRoot, ".compozy")); !os.IsNotExist(err) {
		t.Fatalf("expected unrelated archive subtree to be skipped, stat err = %v", err)
	}
}

func buildTarGz(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Mode:     0o755,
			Typeflag: entry.typeflag,
			Linkname: entry.linkname,
		}
		if entry.typeflag == tar.TypeReg {
			header.Mode = 0o644
			header.Size = int64(len(entry.body))
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", entry.name, err)
		}
		if entry.typeflag == tar.TypeReg {
			if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
				t.Fatalf("Write(%q) error = %v", entry.name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}
	return buffer.Bytes()
}
