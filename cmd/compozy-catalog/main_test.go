package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/registry"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("Should propagate digest output failures", func(t *testing.T) {
		t.Parallel()

		artifactPath := filepath.Join(t.TempDir(), "artifact")
		if err := os.WriteFile(artifactPath, []byte("catalog artifact"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		writeErr := errors.New("stdout unavailable")
		err := run(t.Context(), []string{catalogDigestCommand, artifactPath}, failingWriter{err: writeErr})
		if !errors.Is(err, writeErr) {
			t.Fatalf("run(digest) error = %v, want output failure", err)
		}
	})

	t.Run("Should preserve command cancellation before filesystem work", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := run(ctx, []string{catalogDigestCommand, "unused"}, io.Discard)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run(canceled) error = %v, want context.Canceled", err)
		}
	})
}

// Invariant: one v3 publication is readable by the runtime source and preserves extension identities and bytes.
// Owner: catalog publisher. Canonical suite: main_test.go.
func TestPublishCatalog(t *testing.T) {
	t.Parallel()
	source := filepath.Join("..", "..", "catalog")
	output := filepath.Join(t.TempDir(), "published")
	if err := run(t.Context(), []string{"publish", source, output}, io.Discard); err != nil {
		t.Fatal(err)
	}
	t.Run("Should emit only the current catalog family with twenty extensions [UT-054]", func(t *testing.T) {
		t.Parallel()
		files, err := os.ReadDir(output)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 2 || files[0].Name() != "artifacts" || files[1].Name() != "v3" {
			t.Fatalf("publication roots = %v, want artifacts and v3", files)
		}
		server := httptest.NewServer(http.StripPrefix("/catalog/", http.FileServer(http.Dir(output))))
		t.Cleanup(server.Close)
		source, err := marketplace.NewHTTPSource(server.URL+"/catalog", &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		document, err := source.Fetch(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if document.ManifestVersion != 3 || len(document.Entries) != 20 {
			t.Fatalf("v3 version/count = %d/%d", document.ManifestVersion, len(document.Entries))
		}
		for _, entry := range document.Entries {
			if entry.EntryID == "documentation-writer" || entry.EntryID == "repository-orientation" {
				t.Fatalf("retired listing %q remains in v3", entry.EntryID)
			}
		}
		presets, err := os.ReadFile(filepath.Join(output, "v3", "marketplaces.json"))
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := marketplace.DecodePresets(presets)
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed.Entries) != 2 || parsed.Entries[0].Name != "claude-plugins-official" ||
			parsed.Entries[0].Default != "on" || parsed.Entries[1].Name != "openai-codex" || parsed.Entries[1].Default != "off" {
			t.Fatalf("presets = %#v", parsed.Entries)
		}
	})
	t.Run("Should preserve existing extension metadata and artifact bytes [UT-007]", func(t *testing.T) {
		t.Parallel()
		original := readPublishedExtensions(t, source)
		published := readPublishedExtensions(t, output)
		for _, name := range []string{"batuta", "herdr-bridge"} {
			before, existed := original[name]
			after, exists := published[name]
			if !existed || !exists || !reflect.DeepEqual(before, after) {
				t.Fatalf("existing extension %q changed: before=%#v after=%#v", name, before, after)
			}
			filename, err := curatedArtifactFilename(before.ArtifactURL)
			if err != nil {
				t.Fatal(err)
			}
			for _, directory := range []string{source, output} {
				digest, err := marketplace.DigestFile(filepath.Join(directory, "artifacts", filename))
				if err != nil {
					t.Fatal(err)
				}
				if digest != before.DigestSHA256 {
					t.Fatalf("artifact %q in %q changed: %s, want %s", name, directory, digest, before.DigestSHA256)
				}
			}
		}
	})
	t.Run("Should publish changed package content instead of retaining old artifact bytes", func(t *testing.T) {
		t.Parallel()

		changedSource := t.TempDir()
		if err := os.CopyFS(changedSource, os.DirFS(source)); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(changedSource, "packages", "batuta", "README.md"),
			[]byte("Changed publication content.\n"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		changedOutput := t.TempDir()
		if err := run(t.Context(), []string{"publish", changedSource, changedOutput}, io.Discard); err != nil {
			t.Fatal(err)
		}
		before := readPublishedExtensions(t, source)["batuta"]
		after := readPublishedExtensions(t, changedOutput)["batuta"]
		if after.DigestSHA256 == "" || after.DigestSHA256 == before.DigestSHA256 {
			t.Fatal("changed package did not publish a new artifact digest")
		}
	})
	t.Run("Should reject source manifest drift in a committed catalog", func(t *testing.T) {
		t.Parallel()
		altered := t.TempDir()
		if err := os.CopyFS(altered, os.DirFS(output)); err != nil {
			t.Fatal(err)
		}
		if err := os.CopyFS(
			filepath.Join(altered, "packages"),
			os.DirFS(filepath.Join(source, "packages")),
		); err != nil {
			t.Fatal(err)
		}
		if err := run(t.Context(), []string{"validate", altered}, io.Discard); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(altered, "packages", "linear", "extension.toml")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		changed := strings.Replace(
			string(raw),
			`min_compozy_version = "0.3.0-beta.25"`,
			`min_compozy_version = "0.5.0"`,
			1,
		)
		if changed == string(raw) {
			t.Fatal("fixture minimum version was not changed")
		}
		if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
			t.Fatal(err)
		}
		err = run(t.Context(), []string{"validate", altered}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), `extension "linear" packaged manifest differs from source`) {
			t.Fatalf("drift validation = %v", err)
		}
	})
	for _, layout := range []string{"plugin.json", ".claude-plugin/plugin.json", ".codex-plugin/plugin.json", ".cursor-plugin/plugin.json"} {
		t.Run("Should publish portable packages without native requirements using "+layout, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			sources, err := readPublicationSources(source)
			if err != nil {
				t.Fatal(err)
			}
			sources.Entries = []publicationEntry{{
				EntryID: "portable", Name: "Portable", Description: "Portable package", InstallSlug: "acme/portable",
				Tier: "official", Format: "agent-plugin",
			}}
			if err := writePublicationJSON(root, "sources.json", sources); err != nil {
				t.Fatal(err)
			}
			presets, err := os.ReadFile(filepath.Join(source, "marketplaces.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "marketplaces.json"), presets, 0o600); err != nil {
				t.Fatal(err)
			}
			manifest := map[string]string{
				"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
				"name":    "portable", "version": "1.0.0", "description": "Portable package",
			}
			manifestPath := filepath.Join("packages", "portable", filepath.FromSlash(layout))
			if err := writePublicationJSON(root, manifestPath, manifest); err != nil {
				t.Fatal(err)
			}
			if err := run(t.Context(), []string{"publish", root, root}, io.Discard); err != nil {
				t.Fatal(err)
			}
			if err := run(t.Context(), []string{"validate", root}, io.Discard); err != nil {
				t.Fatal(err)
			}
			manifest["description"] = "Changed source"
			if err := writePublicationJSON(root, manifestPath, manifest); err != nil {
				t.Fatal(err)
			}
			if err := run(
				t.Context(),
				[]string{"validate", root},
				io.Discard,
			); err == nil ||
				!strings.Contains(err.Error(), "packaged manifest differs from source") {
				t.Fatalf("portable drift validation = %v", err)
			}
		})
	}

	t.Run("Should reject a published artifact above the compressed size limit", func(t *testing.T) {
		t.Parallel()
		oversizedSource := t.TempDir()
		if err := os.CopyFS(oversizedSource, os.DirFS(source)); err != nil {
			t.Fatal(err)
		}
		entry := readPublishedExtensions(t, source)["batuta"]
		filename, err := curatedArtifactFilename(entry.ArtifactURL)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Truncate(
			filepath.Join(oversizedSource, "artifacts", filename), registry.DefaultMaxArchiveSize+1,
		); err != nil {
			t.Fatal(err)
		}
		err = run(t.Context(), []string{"publish", oversizedSource, t.TempDir()}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "exceeds the compressed size limit") {
			t.Fatalf("expected compressed archive size rejection, got %v", err)
		}
	})
	t.Run("Should reject a root-only catalog without falling back [UT-055]", func(t *testing.T) {
		t.Parallel()
		rootOnly := t.TempDir()
		raw, err := os.ReadFile(filepath.Join(output, "v3", "extensions.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootOnly, "extensions.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		err = run(t.Context(), []string{"validate", rootOnly}, io.Discard)
		if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "v3") {
			t.Fatalf("root-only validation = %v, want missing v3", err)
		}
		server := httptest.NewServer(http.FileServer(http.Dir(rootOnly)))
		t.Cleanup(server.Close)
		source, err := marketplace.NewHTTPSource(server.URL, &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := source.Fetch(t.Context()); err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("root-only runtime fetch = %v, want missing v3", err)
		}
	})
	t.Run("Should reject a published input declaration that disagrees with package bytes", func(t *testing.T) {
		t.Parallel()
		altered := filepath.Join(t.TempDir(), "altered")
		if err := os.CopyFS(altered, os.DirFS(output)); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(altered, "v3", "extensions.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document publicationDocument[publicationEntry]
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		for index := range document.Entries {
			if document.Entries[index].EntryID == "context7" {
				document.Entries[index].Inputs = nil
			}
		}
		if err := writePublicationJSON(altered, "v3/extensions.json", document); err != nil {
			t.Fatal(err)
		}
		metadata, err := os.ReadFile(filepath.Join(altered, "v3", "compatibility.json"))
		if err != nil {
			t.Fatal(err)
		}
		var compatibility marketplace.CompatibilityDocument
		if err := json.Unmarshal(metadata, &compatibility); err != nil {
			t.Fatal(err)
		}
		compatibility.FeedDigestSHA256, err = marketplace.DigestFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := writePublicationJSON(altered, "v3/compatibility.json", compatibility); err != nil {
			t.Fatal(err)
		}
		err = run(t.Context(), []string{"validate", altered}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "feed inputs differ") {
			t.Fatalf("altered validation = %v", err)
		}
	})
}

func readPublishedExtensions(t *testing.T, directory string) map[string]publicationEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(directory, "v3", "extensions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document publicationDocument[publicationEntry]
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]publicationEntry, len(document.Entries))
	for _, entry := range document.Entries {
		entries[entry.EntryID] = entry
	}
	return entries
}
