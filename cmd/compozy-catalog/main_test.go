package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/marketplace"
	v2decoder "github.com/compozy/compozy/internal/marketplace/testdata/v2decoder"
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

// Invariant: retained publication is accepted by the released decoder and its semantic rules.
// Owner: catalog publication. Canonical suite: compozy-catalog main_test.go.
func TestReleasedCatalogConformance(t *testing.T) {
	t.Parallel()
	files := []struct {
		name string
		kind v2decoder.Kind
	}{
		{
			"extensions.json",
			v2decoder.KindExtension,
		},
		{"mcp.json", v2decoder.KindMCP},
		{"skills.json", v2decoder.KindSkill},
	}
	for _, file := range files {
		t.Run("Should accept the retained "+file.name+" with the released validator", func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join("..", "..", "catalog", file.name))
			if err != nil {
				t.Fatal(err)
			}
			document, err := v2decoder.DecodeDocument(file.kind, raw)
			if err != nil {
				t.Fatal(err)
			}
			if len(document.Entries) == 0 {
				t.Fatal("retained family is empty")
			}
		})
	}
	t.Run("Should reject a query binding already present in the launch URL [UT-054]", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("..", "..", "catalog", "mcp.json"))
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range document["entries"].([]any) {
			entry := item.(map[string]any)
			if entry["entry_id"] != "supabase" {
				continue
			}
			entry["launch"].(map[string]any)["url"] = "https://mcp.supabase.com/mcp?read_only=true&project_ref="
			found = true
		}
		if !found {
			t.Fatal("query input fixture missing")
		}
		candidate, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		_, err = v2decoder.DecodeDocument(v2decoder.KindMCP, candidate)
		if err == nil || !strings.Contains(err.Error(), "conflicts with launch URL") {
			t.Fatalf("released validator = %v", err)
		}
	})
}

// Invariant: one publication generates coherent v3/v2 feeds from the same verified package bytes.
// Owner: catalog publisher. Canonical suite: main_test.go.
func TestPublishCatalogFamilies(t *testing.T) {
	t.Parallel()
	source := filepath.Join("..", "..", "catalog")
	output := filepath.Join(t.TempDir(), "published")
	if err := run(t.Context(), []string{"publish", source, output}, io.Discard); err != nil {
		t.Fatal(err)
	}
	t.Run("Should emit twenty packaged extensions and a conformant retained family [UT-054]", func(t *testing.T) {
		t.Parallel()
		if err := validateReleasedPublication(output); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(output, "v3", "extensions.json"))
		if err != nil {
			t.Fatal(err)
		}
		document, err := marketplace.DecodeDocument(marketplace.KindExtension, raw)
		if err != nil {
			t.Fatal(err)
		}
		if document.ManifestVersion != 3 || len(document.Entries) != 20 {
			t.Fatalf("v3 version/count = %d/%d", document.ManifestVersion, len(document.Entries))
		}
		for _, entry := range document.Entries {
			if entry.EntryID == "documentation-writer" {
				t.Fatal("retired skill remains in v3")
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
		if len(parsed.Entries) != 2 || parsed.Entries[0].Name != "claude-plugins-official" {
			t.Fatalf("presets = %#v", parsed.Entries)
		}
	})
	t.Run("Should preserve fixed query values while stripping input-bound parameters [UT-054]", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join(output, "mcp.json"))
		if err != nil {
			t.Fatal(err)
		}
		var document publicationDocument[publicationMCPEntry]
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatal(err)
		}
		if len(document.Entries) != 17 {
			t.Fatalf("retained server count = %d", len(document.Entries))
		}
		found := false
		for _, entry := range document.Entries {
			if entry.EntryID != "supabase" {
				continue
			}
			found = true
			if entry.Launch.URL != "https://mcp.supabase.com/mcp?read_only=true" || len(entry.Inputs) != 1 ||
				entry.Inputs[0].Binding.Name != "project_ref" {
				t.Fatalf("supabase = %#v", entry)
			}
		}
		if !found {
			t.Fatal("Supabase missing")
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
		err = run(t.Context(), []string{"validate", altered}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "feed inputs differ") {
			t.Fatalf("altered validation = %v", err)
		}
	})
}
