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
	"github.com/compozy/compozy/internal/registry"
	"github.com/compozy/compozy/internal/registry/gitsrc"
)

func TestResolveAndAcquire(t *testing.T) {
	t.Parallel()
	t.Run("Should retain canonical folder identity and acquire cached bytes after source deletion", func(t *testing.T) {
		t.Parallel()
		resolver, root, doc, snapshot := folderResolver(t)
		record, err := resolver.Resolve(t.Context(), doc, snapshot, doc.Plugins[0])
		if err != nil {
			t.Fatal(err)
		}
		if record.SourceRef != doc.SourceRef || record.EntryID != "tool" || record.PackagePath != "tool" ||
			record.ResolvedRef != doc.SourceRef+"@"+record.DigestSHA256 ||
			record.Version != record.DigestSHA256[:12] || record.Layout != "claude-plugin" {
			t.Fatalf("acquisition record = %+v", record)
		}
		second, err := resolver.Resolve(t.Context(), doc, snapshot, doc.Plugins[0])
		if err != nil || second != record {
			t.Fatalf("repeat acquisition = %+v, %v", second, err)
		}
		declared := doc.Plugins[0]
		declared.Version = "2.3.4"
		versioned, err := resolver.Resolve(t.Context(), doc, snapshot, declared)
		if err != nil || versioned.Version != declared.Version || versioned.DigestSHA256 != record.DigestSHA256 {
			t.Fatalf("declared version acquisition = %+v, %v", versioned, err)
		}
		if err := snapshot.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
		assertAcquiredPackage(t, resolver, record)
		assertAcquisitionClean(t, resolver)
	})
	t.Run("Should reject an approval mismatch before touching cache or source", func(t *testing.T) {
		t.Parallel()
		listed, approved := strings.Repeat("a", 64), strings.Repeat("b", 64)
		resolver := &Resolver{}
		reader, err := resolver.Acquire(t.Context(), AcquisitionRecord{DigestSHA256: listed}, approved)
		mismatch, ok := errors.AsType[*registry.ArchiveDigestMismatchError](err)
		if reader != nil || !ok || mismatch.ExpectedSHA256 != approved || mismatch.ActualSHA256 != listed {
			t.Fatalf("precheck = %v, %v", reader, err)
		}
	})
	for _, mode := range []string{"missing", "corrupt"} {
		t.Run("Should recover "+mode+" cache bytes only from the same live package", func(t *testing.T) {
			t.Parallel()
			resolver, _, doc, snapshot := folderResolver(t)
			record, err := resolver.Resolve(t.Context(), doc, snapshot, doc.Plugins[0])
			if err != nil {
				t.Fatal(err)
			}
			blob := filepath.Join(resolver.Cache.Root, record.DigestSHA256+".tar")
			if mode == "corrupt" {
				err = os.WriteFile(blob, []byte("corrupt"), 0o600)
			} else {
				err = os.Remove(blob)
			}
			if err != nil {
				t.Fatal(err)
			}
			assertAcquiredPackage(t, resolver, record)
			reader, err := resolver.Cache.Open(t.Context(), record.DigestSHA256)
			if err != nil {
				t.Fatal(err)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			assertAcquisitionClean(t, resolver)
		})
	}
	t.Run("Should reject changed live bytes without publishing either digest", func(t *testing.T) {
		t.Parallel()
		resolver, root, doc, snapshot := folderResolver(t)
		record, err := resolver.Resolve(t.Context(), doc, snapshot, doc.Plugins[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(resolver.Cache.Root, record.DigestSHA256+".tar")); err != nil {
			t.Fatal(err)
		}
		writeMarketplaceDocument(t, root, "tool/README.md", []byte("changed since approval"))
		reader, err := resolver.Acquire(t.Context(), record, record.DigestSHA256)
		mismatch, ok := errors.AsType[*registry.ArchiveDigestMismatchError](err)
		if reader != nil || !ok || mismatch.ExpectedSHA256 != record.DigestSHA256 ||
			!validCacheDigest(mismatch.ActualSHA256) || mismatch.ActualSHA256 == record.DigestSHA256 {
			t.Fatalf("changed source = %v, %v", reader, err)
		}
		entries, err := os.ReadDir(resolver.Cache.Root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("unapproved cache publication = %v, %v", entries, err)
		}
		assertAcquisitionClean(t, resolver)
	})
	t.Run("Should report an unreachable source when its approved bytes are absent", func(t *testing.T) {
		t.Parallel()
		resolver, root, doc, snapshot := folderResolver(t)
		record, err := resolver.Resolve(t.Context(), doc, snapshot, doc.Plugins[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(resolver.Cache.Root, record.DigestSHA256+".tar")); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
		reader, err := resolver.Acquire(t.Context(), record, record.DigestSHA256)
		if reader != nil || !errors.Is(err, ErrSourceUnreachable) {
			t.Fatalf("unreachable acquisition = %v, %v", reader, err)
		}
		assertAcquisitionClean(t, resolver)
	})
	t.Run("Should reject a package outside the shared snapshot", func(t *testing.T) {
		t.Parallel()
		resolver, _, doc, snapshot := folderResolver(t)
		plugin := doc.Plugins[0]
		plugin.Source.Path = "../outside"
		if _, err := resolver.Resolve(t.Context(), doc, snapshot, plugin); !errors.Is(err, ErrSourceOutsideCheckout) {
			t.Fatalf("unconfined resolve = %v", err)
		}
		assertAcquisitionClean(t, resolver)
	})
}

func TestResolveRemotePackages(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("e", 40)
	tests := []struct {
		name   string
		source PluginSource
	}{
		{name: "Should pin a GitHub object and confine its package subdirectory", source: PluginSource{
			Kind: SourceGitHub, Ref: "github:owner/repository", GitRef: "release", Path: "tool",
		}},
		{name: "Should pin a GitHub HTTPS repository to its resolved commit", source: PluginSource{
			Kind: SourceHTTPS, Ref: "https://github.com/owner/repository", Path: "tool",
		}},
		{name: "Should fingerprint a public HTTPS archive package after removing its wrapper", source: PluginSource{
			Kind: SourceHTTPS, Ref: "https://8.8.8.8/tool.tar.gz",
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := "repository/tool/.claude-plugin/plugin.json"
			if isArchiveRef(tc.source.Ref) {
				path = "tool/.claude-plugin/plugin.json"
			}
			writeMarketplaceDocument(t, root, path, []byte(`{"name":"tool"}`))
			archive := githubSnapshotArchive(t, root)
			requests := 0
			client := &http.Client{
				Transport: marketplaceRoundTripper(func(request *http.Request) (*http.Response, error) {
					requests++
					if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
						t.Error("public package acquisition carried credentials")
					}
					if strings.Contains(request.URL.Path, "/commits/") {
						ref := tc.source.GitRef
						if ref == "" {
							ref = "HEAD"
						}
						if !strings.HasSuffix(request.URL.Path, "/"+ref) {
							t.Errorf("resolved the wrong authored Git ref: %s", request.URL.Path)
						}
						return marketplaceHTTPResponse(t, request, http.StatusOK, commit), nil
					}
					if !isArchiveRef(tc.source.Ref) && !strings.HasSuffix(request.URL.Path, "/tarball/"+commit) {
						t.Errorf("download did not use the resolved commit: %s", request.URL.Path)
					}
					response := marketplaceHTTPResponse(t, request, http.StatusOK, string(archive))
					response.Header.Set("Content-Type", "application/gzip")
					return response, nil
				}),
			}
			resolver := &Resolver{Cache: &PackageCache{Root: t.TempDir()}, Sources: Sources{
				TempDir: t.TempDir(), HTTPClient: client, GitHubOptions: []GitHubOption{WithGitHubHTTPClient(client)},
			}}
			doc := Document{SourceRef: "github:team/marketplace"}
			record, err := resolver.Resolve(t.Context(), doc, nil, Plugin{Name: "tool", Source: tc.source})
			if err != nil {
				t.Fatal(err)
			}
			resolved, version, count := "github:owner/repository@"+commit, commit[:12], 2
			if isArchiveRef(tc.source.Ref) {
				resolved, version, count = tc.source.Ref+"@"+record.DigestSHA256, record.DigestSHA256[:12], 1
			}
			if record.ResolvedRef != resolved || record.Version != version || record.Layout != "claude-plugin" ||
				record.SourceRef != doc.SourceRef || requests != count {
				t.Fatalf("remote record = %+v, requests %d", record, requests)
			}
			assertAcquiredPackage(t, resolver, record)
			assertAcquisitionClean(t, resolver)
		})
	}
	t.Run("Should remove extracted archives when the HTTP reader fails to close", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeMarketplaceDocument(t, root, "plugin.json", []byte(`{"name":"tool"}`))
		archive := githubSnapshotArchive(t, root)
		closeErr := errors.New("HTTP reader cleanup failed")
		client := &http.Client{Transport: marketplaceRoundTripper(func(request *http.Request) (*http.Response, error) {
			response := marketplaceHTTPResponse(t, request, http.StatusOK, string(archive))
			response.Header.Set("Content-Type", "application/gzip")
			response.Body = &acquisitionCloseFailure{Reader: bytes.NewReader(archive), err: closeErr}
			return response, nil
		})}
		resolver := &Resolver{
			Cache:   &PackageCache{Root: t.TempDir()},
			Sources: Sources{TempDir: t.TempDir(), HTTPClient: client},
		}
		_, err := resolver.Resolve(t.Context(), Document{SourceRef: "github:team/gallery"}, nil, Plugin{
			Name: "tool", Source: PluginSource{Kind: SourceHTTPS, Ref: "https://8.8.8.8/tool.tar.gz"},
		})
		if !errors.Is(err, closeErr) {
			t.Fatalf("reader cleanup failure = %v", err)
		}
		assertAcquisitionClean(t, resolver)
		assertCacheEmpty(t, resolver.Cache)
	})
}

type acquisitionCloseFailure struct {
	io.Reader
	err error
}

func (r *acquisitionCloseFailure) Close() error { return r.err }

func folderResolver(t *testing.T) (*Resolver, string, Document, *Snapshot) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "team@work")
	writeMarketplaceDocument(t, root, "marketplace.json", []byte(`{"plugins":[{"name":"tool","source":"./tool"}]}`))
	writeMarketplaceDocument(t, root, "tool/.claude-plugin/plugin.json", []byte(`{"name":"tool"}`))
	resolver := &Resolver{Cache: &PackageCache{Root: t.TempDir()}, Sources: Sources{TempDir: t.TempDir()}}
	doc, err := resolver.Sources.Fetch(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := resolver.Sources.OpenSnapshot(t.Context(), doc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := snapshot.Close(); err != nil {
			t.Error(err)
		}
	})
	return resolver, root, doc, snapshot
}

func assertAcquiredPackage(t *testing.T, resolver *Resolver, record AcquisitionRecord) {
	t.Helper()
	reader, err := resolver.Acquire(t.Context(), record, record.DigestSHA256)
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(reader)
	if err := errors.Join(readErr, reader.Close()); err != nil {
		t.Fatal(err)
	}
	archive := tar.NewReader(bytes.NewReader(content))
	found := false
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == ".claude-plugin/plugin.json" {
			manifest, err := io.ReadAll(archive)
			if err != nil || string(manifest) != `{"name":"tool"}` {
				t.Fatalf("acquired manifest = %q, %v", manifest, err)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("acquired archive has no authored manifest")
	}
}

func assertAcquisitionClean(t *testing.T, resolver *Resolver) {
	t.Helper()
	entries, err := os.ReadDir(resolver.Sources.TempDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("acquisition retained staging or inspection resources: %v, %v", entries, err)
	}
}

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
		resolver := &Resolver{Cache: &PackageCache{Root: t.TempDir()}, Sources: Sources{
			GitOptions: source.options, TempDir: temporary,
		}}
		document, err := resolver.Sources.Fetch(t.Context(), source.ref)
		if err != nil || document.Path != ".claude-plugin/marketplace.json" {
			t.Fatalf("Git document = %+v, %v", document, err)
		}
		entries, err := os.ReadDir(temporary)
		if err != nil || len(entries) != 0 {
			t.Fatalf("Fetch retained its checkout: %v, %v", entries, err)
		}
		snapshot, err := resolver.Sources.OpenSnapshot(t.Context(), document)
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
		record, err := resolver.Resolve(t.Context(), document, snapshot, document.Plugins[0])
		if err != nil {
			t.Fatal(err)
		}
		if record.ResolvedRef != document.ResolvedRef || record.PackagePath != "tool" || record.Layout != "standard" {
			t.Fatalf("Git acquisition record = %+v", record)
		}
		if err := snapshot.Close(); err != nil {
			t.Fatal(err)
		}
		entries, err = os.ReadDir(temporary)
		if err != nil || len(entries) != 0 {
			t.Fatalf("Close retained its checkout: %v, %v", entries, err)
		}
		if err := os.Remove(filepath.Join(resolver.Cache.Root, record.DigestSHA256+".tar")); err != nil {
			t.Fatal(err)
		}
		reader, err := resolver.Acquire(t.Context(), record, record.DigestSHA256)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		assertAcquisitionClean(t, resolver)
		unpinned := document
		unpinned.ResolvedRef = document.SourceRef + "@main"
		if _, err := source.OpenSnapshot(t.Context(), unpinned, temporary); err == nil {
			t.Fatal("accepted a branch in a resolved repository revision")
		}
		externalDoc := Document{SourceRef: "github:team/gallery"}
		external, err := resolver.Resolve(t.Context(), externalDoc, nil, Plugin{
			Name: "tool", Source: PluginSource{Kind: SourceHTTPS, Ref: repository, Path: "tool"},
		})
		if err != nil || external.SourceRef != externalDoc.SourceRef ||
			external.ResolvedRef != record.ResolvedRef || external.DigestSHA256 != record.DigestSHA256 {
			t.Fatalf("external Git acquisition = %+v, %v", external, err)
		}
		assertAcquisitionClean(t, resolver)
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
