package pluginsource

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/registry/gitsrc"
)

func TestGitSourceSnapshot(t *testing.T) {
	t.Parallel()
	t.Run("Should acquire a real pinned Git tree through an isolated repository transport", func(t *testing.T) {
		t.Parallel()
		executable, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		fixture := t.TempDir()
		writeMarketplaceDocument(
			t,
			fixture,
			".claude-plugin/marketplace.json",
			[]byte(`{"plugins":[{"name":"tool","source":"./tool"}]}`),
		)
		writeMarketplaceDocument(t, fixture, "tool/plugin.json", []byte(`{"name":"tool","version":"1.0.0"}`))
		commands := [][]string{
			{"init", "-b", "main", fixture},
			{"-C", fixture, "add", "."},
			{"-C", fixture, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.com",
				"-c", "core.hooksPath=" + os.DevNull, "commit", "--no-gpg-sign", "-m", "fixture"},
		}
		for _, args := range commands {
			if err := runFixtureGit(t.Context(), t, executable, args...); err != nil {
				t.Fatal(err)
			}
		}
		temporary := t.TempDir()
		repository := "https://8.8.8.8/owner/repo"
		source, err := NewGitSource("git+"+repository,
			gitsrc.WithLookPath(func(string) (string, error) { return executable, nil }),
			gitsrc.WithCheckoutTempDir(temporary),
			gitsrc.WithRunner(func(ctx context.Context, executable string, args ...string) error {
				for index, arg := range args {
					if arg == repository {
						args[index] = fixture
					}
				}
				return runFixtureGit(
					ctx,
					t,
					executable,
					append([]string{"-c", "protocol.file.allow=always"}, args...)...)
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
		document, err := source.Fetch(t.Context())
		if err != nil || document.Path != ".claude-plugin/marketplace.json" {
			t.Fatalf("Git document = %+v, %v", document, err)
		}
		entries, err := os.ReadDir(temporary)
		if err != nil || len(entries) != 0 {
			t.Fatalf("Fetch retained its checkout: %v, %v", entries, err)
		}
		snapshot, err := source.OpenSnapshot(t.Context(), document, temporary)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := snapshot.Close(); err != nil {
				t.Error(err)
			}
		})
		if snapshot.ResolvedRef != document.ResolvedRef {
			t.Fatalf("snapshot revision %q differs from document %q", snapshot.ResolvedRef, document.ResolvedRef)
		}
		cache := &PackageCache{Root: t.TempDir()}
		if _, err := CapturePackage(t.Context(), snapshot.Root, "tool", cache); err != nil {
			t.Fatal(err)
		}
		if err := snapshot.Close(); err != nil {
			t.Fatal(err)
		}
		entries, err = os.ReadDir(temporary)
		if err != nil || len(entries) != 0 {
			t.Fatalf("Close retained its checkout: %v, %v", entries, err)
		}
		unpinned := document
		unpinned.ResolvedRef = document.SourceRef + "@main"
		if _, err := source.OpenSnapshot(t.Context(), unpinned, temporary); err == nil {
			t.Fatal("accepted a branch in a resolved repository revision")
		}
	})
}

func runFixtureGit(ctx context.Context, t *testing.T, executable string, args ...string) error {
	t.Helper()
	command := exec.CommandContext(ctx, executable, args...)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "GIT_") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("fixture git: %w: %s", err, output)
	}
	return nil
}

func TestGitHubSnapshot(t *testing.T) {
	t.Parallel()
	t.Run("Should capture relative packages from one pinned repository snapshot", func(t *testing.T) {
		t.Parallel()
		manifest := []byte(`{"name":"team","plugins":[{"name":"tool","source":"./plugins/tool"}]}`)
		checkout := t.TempDir()
		writeMarketplaceDocument(t, checkout, "snapshot/marketplace.json", manifest)
		writeMarketplaceDocument(
			t,
			checkout,
			"snapshot/plugins/tool/plugin.json",
			[]byte(`{"name":"tool","version":"1.0.0"}`),
		)
		archive := githubSnapshotArchive(t, checkout)
		archiveRequests := 0
		source := githubSnapshotSource(t, manifest, archive, &archiveRequests)
		document, err := source.Fetch(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		temporary := t.TempDir()
		snapshot, err := source.OpenSnapshot(t.Context(), document, temporary)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := snapshot.Close(); err != nil {
				t.Error(err)
			}
		})
		if snapshot.ResolvedRef != document.ResolvedRef || archiveRequests != 1 {
			t.Fatalf("snapshot = %+v, archive requests %d", snapshot, archiveRequests)
		}
		cache := &PackageCache{Root: t.TempDir()}
		digest, err := CapturePackage(t.Context(), snapshot.Root, document.Plugins[0].Source.Path, cache)
		if err != nil {
			t.Fatal(err)
		}
		if err := snapshot.Close(); err != nil {
			t.Fatal(err)
		}
		entries, readErr := os.ReadDir(temporary)
		if readErr != nil || len(entries) != 0 {
			t.Fatalf("snapshot cleanup left %v, %v", entries, readErr)
		}
		reader, err := cache.Open(t.Context(), digest)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should remove a snapshot whose document differs or whose archive is unsafe", func(t *testing.T) {
		t.Parallel()
		manifest := []byte(`{"plugins":[]}`)
		for _, failure := range []string{"different_document", "symlink_entry"} {
			checkout := t.TempDir()
			writeMarketplaceDocument(
				t,
				checkout,
				"snapshot/marketplace.json",
				[]byte(`{"name":"changed","plugins":[]}`),
			)
			if failure == "symlink_entry" {
				if err := os.Symlink("../outside", filepath.Join(checkout, "snapshot", "escape")); err != nil {
					t.Fatal(err)
				}
			}
			requests := 0
			source := githubSnapshotSource(t, manifest, githubSnapshotArchive(t, checkout), &requests)
			document, err := source.Fetch(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			temporary := t.TempDir()
			if snapshot, err := source.OpenSnapshot(t.Context(), document, temporary); err == nil || snapshot != nil {
				t.Fatalf("%s accepted snapshot %+v, %v", failure, snapshot, err)
			}
			entries, readErr := os.ReadDir(temporary)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("snapshot cleanup left %v, %v", entries, readErr)
			}
		}
	})
}

func TestDirectorySnapshot(t *testing.T) {
	t.Parallel()
	t.Run("Should capture a local source without taking ownership of the operator folder", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeMarketplaceDocument(t, root, "marketplace.json", []byte(`{"plugins":[{"name":"tool","source":"./tool"}]}`))
		writeMarketplaceDocument(t, root, "tool/plugin.json", []byte(`{"name":"tool"}`))
		source, err := NewDirectorySource(root)
		if err != nil {
			t.Fatal(err)
		}
		document, err := source.Fetch(t.Context())
		if err != nil || document.SourceRef != folderRef(root) {
			t.Fatalf("local document = %+v, %v", document, err)
		}
		snapshot, err := source.OpenSnapshot(t.Context(), document)
		if err != nil {
			t.Fatal(err)
		}
		cache := &PackageCache{Root: t.TempDir()}
		if _, err := CapturePackage(t.Context(), snapshot.Root, document.Plugins[0].Source.Path, cache); err != nil {
			t.Fatal(err)
		}
		if err := snapshot.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, "marketplace.json")); err != nil {
			t.Fatalf("source folder changed during snapshot close: %v", err)
		}
	})
	t.Run("Should reject changed documents and mismatched source identity before capture", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeMarketplaceDocument(t, root, "marketplace.json", []byte(`{"plugins":[]}`))
		source, err := NewDirectorySource(root)
		if err != nil {
			t.Fatal(err)
		}
		document, err := source.Fetch(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		foreign := document
		foreign.SourceRef = folderRef(t.TempDir())
		if _, err := source.OpenSnapshot(t.Context(), foreign); err == nil {
			t.Fatal("accepted a document from a different source")
		}
		writeMarketplaceDocument(t, root, "marketplace.json", []byte(`{"name":"changed","plugins":[]}`))
		if _, err := source.OpenSnapshot(t.Context(), document); err == nil {
			t.Fatal("accepted a changed document")
		}
		if _, err := NewDirectorySource("github:owner/repo"); !errors.Is(err, ErrInvalidRef) {
			t.Fatalf("directory source accepted a repository: %v", err)
		}
	})
}

func githubSnapshotArchive(t *testing.T, root string) []byte {
	t.Helper()
	var archive bytes.Buffer
	if _, err := fileutil.WriteTarGzipDirectory(
		t.Context(),
		&archive,
		root,
		nil,
		fileutil.TarGzipLimits{},
	); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func githubSnapshotSource(t *testing.T, manifest, archive []byte, requests *int) *GitHubSource {
	t.Helper()
	commit := strings.Repeat("c", 40)
	return githubMarketplaceSource(t, func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(request.URL.Path, "/commits/"):
			return marketplaceHTTPResponse(t, request, http.StatusOK, commit), nil
		case strings.Contains(request.URL.Path, "/tarball/"):
			*requests++
			if !strings.HasSuffix(request.URL.Path, "/"+commit) {
				t.Error("snapshot download did not use the fetched document commit")
			}
			response := marketplaceHTTPResponse(t, request, http.StatusOK, string(archive))
			response.Header.Set("Content-Type", "application/gzip")
			return response, nil
		default:
			return marketplaceHTTPResponse(t, request, http.StatusOK, string(manifest)), nil
		}
	})
}

func TestCapturePackage(t *testing.T) {
	t.Parallel()
	t.Run("Should retain the same confined package bytes after the checkout is removed", func(t *testing.T) {
		t.Parallel()
		checkout := t.TempDir()
		manifest := []byte(`{"name":"tool","version":"1.0.0"}`)
		writeMarketplaceDocument(t, checkout, "plugins/tool/plugin.json", manifest)
		writeMarketplaceDocument(t, checkout, "plugins/tool/.git/config", []byte("private Git configuration"))
		writeMarketplaceDocument(t, checkout, "unrelated.txt", []byte("outside the package"))
		cache := &PackageCache{Root: t.TempDir()}
		first, err := CapturePackage(t.Context(), checkout, "./plugins/tool", cache)
		if err != nil {
			t.Fatal(err)
		}
		stamp := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(filepath.Join(checkout, "plugins", "tool", "plugin.json"), stamp, stamp); err != nil {
			t.Fatal(err)
		}
		second, err := CapturePackage(t.Context(), checkout, "plugins/tool", cache)
		if err != nil || first != second {
			t.Fatalf("repeat capture digest = %q, want %q, %v", second, first, err)
		}
		if err := os.RemoveAll(checkout); err != nil {
			t.Fatal(err)
		}
		reopened := &PackageCache{Root: cache.Root}
		reader, err := reopened.Open(t.Context(), first)
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || cacheDigest(raw) != first {
			t.Fatalf("cached package integrity: read %v, close %v", readErr, closeErr)
		}
		archive := tar.NewReader(bytes.NewReader(raw))
		header, err := archive.Next()
		if err != nil || header.Name != "plugin.json" {
			t.Fatalf("package entry = %+v, %v", header, err)
		}
		payload, err := io.ReadAll(archive)
		if err != nil || !bytes.Equal(payload, manifest) {
			t.Fatalf("cached manifest = %q, %v", payload, err)
		}
		if _, err := archive.Next(); !errors.Is(err, io.EOF) {
			t.Fatalf("package included extra checkout/Git entries: %v", err)
		}
	})
	t.Run("Should refuse traversal absolute paths and symlink package roots without caching them", func(t *testing.T) {
		t.Parallel()
		checkout := t.TempDir()
		outside := t.TempDir()
		writeMarketplaceDocument(t, outside, "plugin.json", []byte(`{"name":"outside"}`))
		if err := os.Symlink(outside, filepath.Join(checkout, "escape")); err != nil {
			t.Fatal(err)
		}
		cache := &PackageCache{Root: t.TempDir()}
		for _, path := range []string{"../outside", outside, "escape", "..\\outside", "\x00"} {
			if _, err := CapturePackage(t.Context(), checkout, path, cache); !errors.Is(err, ErrSourceOutsideCheckout) {
				t.Fatalf("CapturePackage(%q) = %v", path, err)
			}
		}
		assertCacheEmpty(t, cache)
	})
	t.Run("Should leave the cache empty when a package exceeds its budget or cannot be read", func(t *testing.T) {
		t.Parallel()
		checkout := t.TempDir()
		writeMarketplaceDocument(t, checkout, "plugin.json", []byte(strings.Repeat("x", 4096)))
		cache := &PackageCache{Root: t.TempDir(), MaxBytes: 1024}
		if _, err := CapturePackage(t.Context(), checkout, "", cache); !errors.Is(err, fileutil.ErrTarSizeLimit) {
			t.Fatalf("oversized package = %v", err)
		}
		if _, err := CapturePackage(t.Context(), checkout, "missing", cache); !errors.Is(err, ErrSourceUnreachable) {
			t.Fatalf("missing package = %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		cancel()
		if _, err := CapturePackage(ctx, checkout, ".", cache); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled package = %v", err)
		}
		assertCacheEmpty(t, cache)
	})
}
