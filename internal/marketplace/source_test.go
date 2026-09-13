package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

func TestPluginProjectionBudget(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve two hundred plugins with at most four workers and preserve source order", func(t *testing.T) {
		t.Parallel()
		doc := pluginsource.Document{SourceRef: "github:team/plugins", Plugins: make([]pluginsource.Plugin, 250)}
		for i := range doc.Plugins {
			doc.Plugins[i] = pluginsource.Plugin{Name: fmt.Sprintf("plugin-%03d", i)}
		}
		var calls, active, peak atomic.Int32
		barrier := make(chan struct{})
		resolver := &projectionResolver{
			resolve: func(ctx context.Context, doc pluginsource.Document, plugin pluginsource.Plugin) (pluginsource.AcquisitionRecord, error) {
				calls.Add(1)
				current := active.Add(1)
				defer active.Add(-1)
				for previous := peak.Load(); current > previous; previous = peak.Load() {
					if peak.CompareAndSwap(previous, current) {
						if current == 4 {
							close(barrier)
						}
						break
					}
				}
				select {
				case <-barrier:
				case <-ctx.Done():
					return pluginsource.AcquisitionRecord{}, ctx.Err()
				}
				return projectionRecord(t, doc, plugin), nil
			},
		}
		projector, err := NewPluginProjector(resolver, func(context.Context, string) (PluginInspection, error) {
			return PluginInspection{Contents: PluginContents{Skills: 2}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		entries, diagnostics, err := projector.Project(ctx, doc, "team-plugins", nil)
		if err != nil || len(entries) != 250 || calls.Load() != 200 || peak.Load() != 4 || active.Load() != 0 {
			t.Fatalf(
				"budget = %d entries, calls %d, peak %d, active %d, error %v",
				len(entries),
				calls.Load(),
				peak.Load(),
				active.Load(),
				err,
			)
		}
		for i, entry := range entries {
			if entry.EntryID != doc.Plugins[i].Name || entry.SourceName != "team-plugins" ||
				entry.Installable != (i < 200) {
				t.Fatalf("entry %d = %+v", i, entry)
			}
			if i >= 200 && entry.InstallBlocker != budgetExhausted {
				t.Fatalf("entry %d is not budget blocked: %+v", i, entry)
			}
		}
		sourceDiagnostics := 0
		for _, diagnostic := range diagnostics {
			if diagnostic.Plugin == "" && diagnostic.Code == budgetExhausted {
				sourceDiagnostics++
			}
		}
		if sourceDiagnostics != 1 {
			t.Fatalf("source budget diagnostics = %d", sourceDiagnostics)
		}
	})
	t.Run("Should join canceled resolves and leave unfinished plugins budget blocked", func(t *testing.T) {
		t.Parallel()
		var active atomic.Int32
		resolver := &projectionResolver{
			resolve: func(ctx context.Context, _ pluginsource.Document, _ pluginsource.Plugin) (pluginsource.AcquisitionRecord, error) {
				active.Add(1)
				defer active.Add(-1)
				<-ctx.Done()
				return pluginsource.AcquisitionRecord{}, ctx.Err()
			},
		}
		projector, err := NewPluginProjector(resolver, func(context.Context, string) (PluginInspection, error) {
			t.Error("inspected a timed-out package")
			return PluginInspection{}, nil
		}, WithPluginRefreshBudget(time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		doc := pluginsource.Document{
			SourceRef: "github:team/plugins",
			Plugins:   []pluginsource.Plugin{{Name: "one"}, {Name: "two"}},
		}
		entries, _, err := projector.Project(t.Context(), doc, "team", nil)
		if err != nil || len(entries) != 2 || active.Load() != 0 {
			t.Fatalf("timed-out projection = %+v, active %d, %v", entries, active.Load(), err)
		}
		for _, entry := range entries {
			if entry.Installable || entry.InstallBlocker != budgetExhausted {
				t.Fatalf("timed-out entry = %+v", entry)
			}
		}
	})
	t.Run("Should preserve loader and cache failures while dropping escaped sources", func(t *testing.T) {
		t.Parallel()
		doc := pluginsource.Document{SourceRef: "github:team/plugins", Plugins: []pluginsource.Plugin{
			{
				Name:        "good",
				Description: "tool",
				Author:      "Team",
				Homepage:    "https://example.com",
				License:     "MIT",
				Category:    "tools",
				Keywords:    []string{"test"},
			},
			{Name: "broken"},
			{Name: "missing"},
			{Name: "escape"},
		}}
		resolver := &projectionResolver{
			resolve: func(_ context.Context, doc pluginsource.Document, plugin pluginsource.Plugin) (pluginsource.AcquisitionRecord, error) {
				if plugin.Name == "escape" {
					return pluginsource.AcquisitionRecord{}, pluginsource.ErrSourceOutsideCheckout
				}
				record := projectionRecord(t, doc, plugin)
				record.DigestSHA256 = fmt.Sprintf("%064x", plugin.Name)
				return record, nil
			},
			inspect: func(ctx context.Context, digest string, inspect func(context.Context, string) error) error {
				if digest == fmt.Sprintf("%064x", "missing") {
					return pluginsource.ErrPackageUnavailable
				}
				return inspect(ctx, digest)
			},
		}
		projector, err := NewPluginProjector(resolver, func(_ context.Context, root string) (PluginInspection, error) {
			if root == fmt.Sprintf("%064x", "broken") {
				return PluginInspection{}, errors.New("invalid authored manifest")
			}
			return PluginInspection{Contents: PluginContents{MCPServers: 1}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		entries, diagnostics, err := projector.Project(t.Context(), doc, "team", nil)
		if err != nil || len(entries) != 3 || len(diagnostics) != 3 || !entries[0].Installable ||
			entries[1].InstallBlocker != "load_failed" || entries[2].InstallBlocker != "package_unavailable" {
			t.Fatalf("projection = %+v, diagnostics %+v, %v", entries, diagnostics, err)
		}
		detail, err := ProjectEntry(entries[0])
		if err != nil || detail.Source != "team" || detail.SourceRef != doc.SourceRef || detail.Author != "Team" ||
			detail.Extension.Acquisition.EntryID != "good" || detail.Extension.Contents.MCPServers != 1 ||
			detail.Extension.Homepage != "https://example.com" || detail.Extension.License != "MIT" ||
			detail.Extension.Category != "tools" || len(detail.Extension.Keywords) != 1 ||
			detail.Extension.Format != ExtensionFormatAgentPlugin || entries[0].Tier != extensionTierUnverified {
			t.Fatalf("projected detail = %+v, %v", detail, err)
		}
		empty, emptyDiagnostics, err := projector.Project(t.Context(), pluginsource.Document{}, "team", nil)
		if err != nil || len(empty) != 0 || len(emptyDiagnostics) != 0 {
			t.Fatalf("empty projection = %+v, %+v, %v", empty, emptyDiagnostics, err)
		}
	})
}

type projectionResolver struct {
	resolve func(context.Context, pluginsource.Document, pluginsource.Plugin) (pluginsource.AcquisitionRecord, error)
	inspect func(context.Context, string, func(context.Context, string) error) error
}

func (r *projectionResolver) Resolve(
	ctx context.Context,
	doc pluginsource.Document,
	_ *pluginsource.Snapshot,
	plugin pluginsource.Plugin,
) (pluginsource.AcquisitionRecord, error) {
	return r.resolve(ctx, doc, plugin)
}

func (r *projectionResolver) Inspect(
	ctx context.Context,
	digest string,
	inspect func(context.Context, string) error,
) error {
	if r.inspect != nil {
		return r.inspect(ctx, digest, inspect)
	}
	return inspect(ctx, digest)
}

func projectionRecord(
	t *testing.T, doc pluginsource.Document, plugin pluginsource.Plugin,
) pluginsource.AcquisitionRecord {
	t.Helper()
	return pluginsource.AcquisitionRecord{
		SourceRef: doc.SourceRef, EntryID: plugin.Name, ResolvedRef: doc.SourceRef + "@" + strings.Repeat("a", 40),
		DigestSHA256: strings.Repeat("b", 64), Version: "1.0.0", Layout: "claude-plugin", PackagePath: plugin.Name,
	}
}

func TestDecodeDocumentValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a valid document for the extension catalog", func(t *testing.T) {
		t.Parallel()

		document, err := DecodeDocument([]byte(validExtensionDocumentJSON()))
		if err != nil {
			t.Fatal(err)
		}
		if document.ManifestVersion != ManifestVersion || len(document.Entries) != 1 ||
			document.Entries[0].EntryID != "bridge-github" {
			t.Fatalf("decoded catalog = %#v", document)
		}
	})

	t.Run("Should accept a loopback HTTP extension artifact", func(t *testing.T) {
		t.Parallel()

		raw := strings.Replace(
			validExtensionDocumentJSON(),
			"https://downloads.example.test/bridge-github-v1.0.0.tar.gz",
			"http://127.0.0.1:2123/bridge-github-v1.0.0.tar.gz",
			1,
		)
		if _, err := DecodeDocument([]byte(raw)); err != nil {
			t.Fatalf("DecodeDocument(loopback HTTP extension) error = %v", err)
		}
	})

	// UT-051: the feed's `format` marker is a hard cut at the current manifest version — reader,
	// in-repo feeds, and fixtures moved together, so no pre-cut compatibility behavior exists here.
	t.Run("Should project the extension format marker at the current manifest version", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			raw        string
			wantFormat string
		}{
			{
				name:       "portable marker",
				raw:        extensionDocumentJSON(`,"format":"agent-plugin"`),
				wantFormat: ExtensionFormatAgentPlugin,
			},
			{
				name:       "explicit native marker",
				raw:        extensionDocumentJSON(`,"format":"compozy"`),
				wantFormat: ExtensionFormatCompozy,
			},
			{
				name:       "marker-less entry defaults to the native format",
				raw:        extensionDocumentJSON(""),
				wantFormat: ExtensionFormatCompozy,
			},
		}
		for _, tt := range tests {
			t.Run("Should decode a "+tt.name, func(t *testing.T) {
				t.Parallel()

				document, err := DecodeDocument([]byte(tt.raw))
				if err != nil {
					t.Fatalf("DecodeDocument(extension) error = %v", err)
				}
				if got, want := document.ManifestVersion, ManifestVersion; got != want {
					t.Fatalf("DecodeDocument(extension).ManifestVersion = %d, want %d", got, want)
				}
				if got, want := len(document.Entries), 1; got != want {
					t.Fatalf("DecodeDocument(extension) entries = %d, want %d", got, want)
				}
				details, err := ProjectEntry(document.Entries[0])
				if err != nil {
					t.Fatalf("ProjectEntry() error = %v", err)
				}
				if details.Extension == nil {
					t.Fatalf("ProjectEntry() extension details = nil, want a projection")
				}
				if got := details.Extension.Format; got != tt.wantFormat {
					t.Fatalf("ProjectEntry().Extension.Format = %q, want %q", got, tt.wantFormat)
				}
			})
		}
	})

	t.Run("Should reject documents outside the v3 schema", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string

			raw     string
			wantErr string
		}{

			{
				name:    "missing manifest version",
				raw:     `{"generated_at":"2026-07-13T00:00:00Z","entries":[]}`,
				wantErr: "manifest_version is required",
			},
			{
				name:    "missing entries array",
				raw:     `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z"}`,
				wantErr: "entries is required",
			},
			{
				name: "extension without digest",
				raw: `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing digest",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz"}]}`,
				wantErr: "digest_sha256 is required",
			},
			{
				name: "extension without registry tier",
				raw: `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing tier",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}]}`,
				wantErr: "tier is required",
			},
			{
				name: "extension without a curated artifact URL",
				raw: `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing artifact",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"official"}]}`,
				wantErr: "artifact_url is required",
			},
			{
				name: "extension with a non HTTPS artifact URL",
				raw: `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Insecure artifact",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"http://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"official"}]}`,
				wantErr: "artifact_url must be an absolute HTTPS URL",
			},
			{
				name: "extension with unknown registry tier",
				raw: `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Unknown tier",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"partner"}]}`,
				wantErr: "unsupported tier",
			},
			{
				name: "extension with unsupported format marker",
				raw: `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Unknown format",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"official","format":"claude-plugin"}]}`,
				wantErr: "unsupported format",
			},
			{
				name: "extension with invented trust field",
				raw: `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"skill","name":"Skill","description":"Invalid trust",` +
					`"install_slug":"compozy/skill","verified":true}]}`,
				wantErr: "unknown field",
			},

			{
				name:    "duplicate extension install slugs",
				raw:     duplicateExtensionInstallSlugsJSON(t),
				wantErr: `install_slug "compozy/bridge-github" is duplicated`,
			},
		}
		for _, tt := range tests {
			t.Run("Should reject "+tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := DecodeDocument([]byte(tt.raw))
				if err == nil {
					t.Fatal("DecodeDocument() error = nil, want validation error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("DecodeDocument() error = %v, want %q", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("Should classify a future manifest as a client upgrade requirement", func(t *testing.T) {
		t.Parallel()

		_, err := DecodeDocument(
			fmt.Appendf(
				nil,
				`{"manifest_version":%d,"generated_at":"2026-07-13T00:00:00Z","entries":[]}`,
				ManifestVersion+1,
			),
		)
		if err == nil {
			t.Fatal("DecodeDocument() error = nil, want unsupported manifest error")
		}
		if _, ok := errors.AsType[*UnsupportedManifestVersionError](err); !ok {
			t.Fatalf("DecodeDocument() error = %T, want UnsupportedManifestVersionError", err)
		}
		if !strings.Contains(err.Error(), "client too old") {
			t.Fatalf("DecodeDocument() error = %v, want client-too-old diagnostic", err)
		}
	})
}

func TestNormalizeMCPInputValue(t *testing.T) {
	t.Parallel()

	// Invariant: shared input bindings neither collide nor put secrets in URL queries.
	// Owner: catalog input grammar; canonical suite: TestNormalizeMCPInputValue.
	for _, test := range []struct{ name, inputs, wantErr string }{
		{"Should reject duplicated input destinations", `[{"id":"first","prompt":"First","type":"string","binding":{"type":"env","name":"SERVER_MODE"}},{"id":"second","prompt":"Second","type":"string","binding":{"type":"env","name":"SERVER_MODE"}}]`, `binding env/"SERVER_MODE" is duplicated`},
		{"Should reject secret URL query inputs", `[{"id":"token","prompt":"Access token","type":"secret","required":true,"binding":{"type":"url_query","name":"token"}}]`, "must not bind a secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var inputs []EntryInput
			if err := json.Unmarshal([]byte(test.inputs), &inputs); err != nil {
				t.Fatal(err)
			}
			if err := ValidateInputGrammar(inputs); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateInputGrammar() = %v, want %q", err, test.wantErr)
			}
		})
	}

	t.Run("Should reject a malformed typed default", func(t *testing.T) {
		t.Parallel()

		input := EntryInput{Type: mcpInputTypeBoolean, Default: json.RawMessage(`true false`)}
		if err := input.validateDefault("input"); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
			t.Fatalf("validateDefault() error = %v, want trailing JSON validation", err)
		}
	})

	t.Run("Should reject defaults that violate install-time value constraints", func(t *testing.T) {
		t.Parallel()

		oversized, err := json.Marshal(strings.Repeat("a", maxMCPInputValueBytes+1))
		if err != nil {
			t.Fatalf("json.Marshal(oversized default) error = %v", err)
		}
		tests := []struct {
			name    string
			input   EntryInput
			wantErr string
		}{
			{
				name: "Should reject NUL in an identifier default",
				input: EntryInput{
					Type: mcpInputTypeIdentifier, Default: json.RawMessage(`"unsafe\u0000identifier"`),
				},
				wantErr: "must not contain NUL",
			},
			{
				name:    "Should reject an oversized string default",
				input:   EntryInput{Type: mcpInputTypeString, Default: oversized},
				wantErr: "exceeds 8192 bytes",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := tt.input.validateDefault("input")
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("validateDefault() error = %v, want %q", err, tt.wantErr)
				}
			})
		}
	})
	t.Run("Should preserve a validated secret value without canonicalization", func(t *testing.T) {
		t.Parallel()

		raw := "  secret value  "
		value, err := NormalizeMCPInputValue(mcpInputTypeSecret, raw)
		if err != nil {
			t.Fatalf("NormalizeMCPInputValue(secret) error = %v", err)
		}
		if value != raw {
			t.Fatalf("NormalizeMCPInputValue(secret) = %q, want %q", value, raw)
		}
	})

	t.Run("Should reject a secret value containing NUL", func(t *testing.T) {
		t.Parallel()

		_, err := NormalizeMCPInputValue(mcpInputTypeSecret, "secret\x00value")
		if err == nil || !strings.Contains(err.Error(), "must not contain NUL") {
			t.Fatalf("NormalizeMCPInputValue(secret) error = %v, want NUL validation", err)
		}
	})
}

func TestValidateCatalogDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should keep every production feed browseable at first launch", func(t *testing.T) {
		t.Parallel()

		catalogDir := filepath.Join("..", "..", "catalog", "v3")
		raw, err := os.ReadFile(filepath.Join(catalogDir, "extensions.json"))
		if err != nil {
			t.Fatal(err)
		}
		document, err := DecodeDocument(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(document.Entries) == 0 {
			t.Fatal("production catalog requires at least one curated launch entry")
		}
	})

	t.Run("Should accept a complete catalog validated by the runtime schemas", func(t *testing.T) {
		t.Parallel()

		if err := ValidateCatalogDirectory(filepath.Join("testdata", "catalog", "valid")); err != nil {
			t.Fatalf("ValidateCatalogDirectory(valid) error = %v", err)
		}
	})

	t.Run("Should reject an extension fixture without a digest", func(t *testing.T) {
		t.Parallel()

		err := ValidateCatalogDirectory(filepath.Join("testdata", "catalog", "missing-digest"))
		if err == nil {
			t.Fatal("ValidateCatalogDirectory(missing-digest) error = nil, want validation failure")
		}
		if !strings.Contains(err.Error(), "digest_sha256 is required") {
			t.Fatalf("ValidateCatalogDirectory(missing-digest) error = %v, want digest diagnostic", err)
		}
	})
}

func TestDigestFile(t *testing.T) {
	t.Parallel()

	t.Run("Should compute the lowercase SHA-256 digest of a file", func(t *testing.T) {
		t.Parallel()

		artifactPath := filepath.Join(t.TempDir(), "extension.tar.gz")
		if err := os.WriteFile(artifactPath, []byte("abc"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		digest, err := DigestFile(artifactPath)
		if err != nil {
			t.Fatalf("DigestFile() error = %v", err)
		}
		const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
		if digest != want {
			t.Fatalf("DigestFile() = %q, want %q", digest, want)
		}
	})
}

func TestCatalogSourceFetch(t *testing.T) {
	t.Parallel()

	// Invariant: file sources cannot fall back to a retired root feed.
	// Owner: directory source resolution; canonical suite: TestCatalogSourceFetch.
	t.Run("Should reject a root-only directory without a v3 family", func(t *testing.T) {
		t.Parallel()
		directory := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(directory, "extensions.json"),
			[]byte(validExtensionDocumentJSON()),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		source, err := NewDirectorySource((&url.URL{Scheme: "file", Path: directory}).String())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := source.Fetch(t.Context()); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Fetch(root-only catalog) = %v, want missing v3 family", err)
		}
	})

	t.Run("Should fetch a checkout document through a file URL", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "v3"), 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "v3", "extensions.json")
		if err := os.WriteFile(path, []byte(validExtensionDocumentJSON()), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
		baseURL := (&url.URL{Scheme: "file", Path: dir}).String()
		source, err := NewSource(baseURL, &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatalf("NewSource(file) error = %v", err)
		}
		document, err := source.Fetch(t.Context())
		if err != nil {
			t.Fatalf("Fetch(file) error = %v", err)
		}
		if got, want := document.Entries[0].EntryID, "bridge-github"; got != want {
			t.Fatalf("Fetch(file) entry id = %q, want %q", got, want)
		}
	})

	t.Run("Should reject an oversized checkout document before decoding", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "v3"), 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "v3", "extensions.json")
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", path, err)
		}
		if err := file.Truncate(defaultMaxResponseBytes + 1); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
			t.Fatalf("Truncate(%q) error = %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close(%q) error = %v", path, err)
		}
		source, err := NewDirectorySource(
			(&url.URL{Scheme: "file", Path: dir}).String(),
		)
		if err != nil {
			t.Fatalf("NewDirectorySource() error = %v", err)
		}
		if _, err := source.Fetch(t.Context()); !errors.Is(err, ErrResponseTooLarge) {
			t.Fatalf("Fetch(oversized file) error = %v, want ErrResponseTooLarge", err)
		}
	})

	t.Run("Should fetch the extension catalog with the injected timeout client", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.URL.Path, "/v3/extensions.json"; got != want {
				t.Errorf("request path = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(validExtensionDocumentJSON())); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		t.Cleanup(server.Close)

		client := &http.Client{Timeout: time.Second}
		source, err := NewHTTPSource(server.URL, client)
		if err != nil {
			t.Fatalf("NewHTTPSource() error = %v", err)
		}
		document, err := source.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if got, want := document.Entries[0].EntryID, "bridge-github"; got != want {
			t.Fatalf("Fetch() entry id = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve the construction timeout after the caller mutates its client", func(t *testing.T) {
		t.Parallel()

		const constructionTimeout = 50 * time.Millisecond
		deadlineRemaining := time.Duration(0)
		client := &http.Client{
			Timeout: constructionTimeout,
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				deadline, ok := request.Context().Deadline()
				if !ok {
					return nil, errors.New("request context has no deadline")
				}
				deadlineRemaining = time.Until(deadline)
				<-request.Context().Done()
				return nil, request.Context().Err()
			}),
		}
		source, err := NewHTTPSource("https://example.test/catalog", client)
		if err != nil {
			t.Fatalf("NewHTTPSource() error = %v", err)
		}
		client.Timeout = 0

		startedAt := time.Now()
		_, err = source.Fetch(t.Context())
		elapsed := time.Since(startedAt)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Fetch() error = %v, want context.DeadlineExceeded", err)
		}
		if deadlineRemaining <= 0 || deadlineRemaining > 2*constructionTimeout {
			t.Fatalf(
				"request deadline remaining = %s, want preserved %s timeout",
				deadlineRemaining,
				constructionTimeout,
			)
		}
		if elapsed < constructionTimeout/2 || elapsed > time.Second {
			t.Fatalf("Fetch() elapsed = %s, want bounded construction timeout near %s", elapsed, constructionTimeout)
		}
	})

	t.Run("Should reject an oversized body before decoding", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte(strings.Repeat("x", 65))); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		t.Cleanup(server.Close)

		source, err := NewHTTPSource(
			server.URL,
			&http.Client{Timeout: time.Second},
			WithMaxResponseBytes(64),
		)
		if err != nil {
			t.Fatalf("NewHTTPSource() error = %v", err)
		}
		_, err = source.Fetch(context.Background())
		if !errors.Is(err, ErrResponseTooLarge) {
			t.Fatalf("Fetch() error = %v, want ErrResponseTooLarge", err)
		}
	})

	t.Run("Should reject clients without an explicit timeout", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPSource("https://example.test/catalog", &http.Client{})
		if err == nil {
			t.Fatal("NewHTTPSource() error = nil, want timeout validation error")
		}
		if !strings.Contains(err.Error(), "timeout must be positive") {
			t.Fatalf("NewHTTPSource() error = %v, want timeout context", err)
		}
	})

	t.Run("Should preserve a non-success HTTP status for classification", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}))
		t.Cleanup(server.Close)
		source, err := NewHTTPSource(server.URL, &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatalf("NewHTTPSource() error = %v", err)
		}
		_, err = source.Fetch(context.Background())
		var statusErr *httpStatusError
		if !errors.As(err, &statusErr) || !strings.Contains(err.Error(), "HTTP 503") {
			t.Fatalf("Fetch() error = %v, want HTTP 503 classification", err)
		}
	})

	t.Run("Should reject a relative catalog URL", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPSource("relative/catalog", &http.Client{Timeout: time.Second})
		if err == nil || !strings.Contains(err.Error(), "absolute HTTP(S) URL") {
			t.Fatalf("NewHTTPSource(relative URL) error = %v, want absolute-URL validation", err)
		}
	})

	t.Run("Should reject a missing HTTP client", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPSource("https://example.test", nil)
		if err == nil || !strings.Contains(err.Error(), "HTTP client timeout must be positive") {
			t.Fatalf("NewHTTPSource(nil client) error = %v, want client validation", err)
		}
	})

	t.Run("Should reject a nil fetch context", func(t *testing.T) {
		t.Parallel()

		source, err := NewHTTPSource("https://example.test", &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatalf("NewHTTPSource(valid) error = %v", err)
		}

		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		if _, err := source.Fetch(nil); err == nil || !strings.Contains(err.Error(), "fetch context is required") {
			t.Fatalf("Fetch(nil context) error = %v, want context validation", err)
		}
	})

	t.Run("Should reject a nil HTTP source receiver", func(t *testing.T) {
		t.Parallel()

		var nilSource *HTTPSource

		_, err := nilSource.Fetch(context.Background())
		if err == nil || !strings.Contains(err.Error(), "HTTP source is required") {
			t.Fatalf("nil Fetch() error = %v, want source validation", err)
		}
	})

	t.Run("Should enforce response bounds and close every acquired HTTP response", func(t *testing.T) {
		t.Parallel()

		const drainLimit int64 = 64 << 10
		validDocument := validExtensionDocumentJSON()
		probeErr := errors.New("probe response")
		tests := []struct {
			name              string
			statusCode        int
			contentLength     int64
			body              string
			afterProbeBody    string
			maxResponseBytes  int64
			wantReadBytes     int64
			wantPrimary       error
			probeErr          error
			wantHTTPStatus    int
			closeErr          error
			oneByteChunks     bool
			wantDocument      bool
			wantValidationErr bool
		}{
			{
				name:           "Should drain a rejected HTTP status before closing",
				statusCode:     http.StatusServiceUnavailable,
				contentLength:  -1,
				body:           strings.Repeat("x", int(drainLimit*2)),
				wantReadBytes:  drainLimit,
				wantHTTPStatus: http.StatusServiceUnavailable,
			},
			{
				name:             "Should drain an advertised oversized response before closing",
				statusCode:       http.StatusOK,
				contentLength:    65,
				body:             strings.Repeat("x", int(drainLimit*2)),
				maxResponseBytes: 64,
				wantReadBytes:    drainLimit,
				wantPrimary:      ErrResponseTooLarge,
			},
			{
				name:             "Should drain only a bounded remainder after discovering an oversized response",
				statusCode:       http.StatusOK,
				contentLength:    -1,
				body:             strings.Repeat("x", 65+int(drainLimit)+1),
				maxResponseBytes: 64,
				wantReadBytes:    65 + drainLimit,
				wantPrimary:      ErrResponseTooLarge,
			},
			{
				name:             "Should preserve oversized and read error identities when the probe returns both",
				statusCode:       http.StatusOK,
				contentLength:    -1,
				body:             validDocument,
				afterProbeBody:   strings.Repeat("x", int(drainLimit)+1),
				maxResponseBytes: int64(len(validDocument)),
				wantReadBytes:    int64(len(validDocument)) + 1 + drainLimit,
				wantPrimary:      ErrResponseTooLarge,
				probeErr:         probeErr,
			},
			{
				name:              "Should close after document validation fails",
				statusCode:        http.StatusOK,
				contentLength:     -1,
				body:              "not JSON",
				wantReadBytes:     int64(len("not JSON")),
				wantValidationErr: true,
			},
			{
				name:             "Should accept a small chunked response at the largest positive limit",
				statusCode:       http.StatusOK,
				contentLength:    -1,
				body:             validDocument,
				maxResponseBytes: math.MaxInt64,
				wantReadBytes:    int64(len(validDocument)),
				oneByteChunks:    true,
				wantDocument:     true,
			},
			{
				name:             "Should accept a document exactly at the configured limit",
				statusCode:       http.StatusOK,
				contentLength:    -1,
				body:             validDocument,
				maxResponseBytes: int64(len(validDocument)),
				wantReadBytes:    int64(len(validDocument)),
				oneByteChunks:    true,
				wantDocument:     true,
			},
			{
				name:          "Should close after a successful document fetch",
				statusCode:    http.StatusOK,
				contentLength: -1,
				body:          validDocument,
				wantReadBytes: int64(len(validDocument)),
				wantDocument:  true,
			},
			{
				name:          "Should preserve a close failure after a successful document fetch",
				statusCode:    http.StatusOK,
				contentLength: -1,
				body:          validDocument,
				wantReadBytes: int64(len(validDocument)),
				closeErr:      errors.New("close successful response"),
				wantDocument:  true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var bodyReader io.Reader = strings.NewReader(tt.body)
				if tt.probeErr != nil {
					bodyReader = io.MultiReader(bodyReader, &byteAndErrorReader{
						err:       tt.probeErr,
						remainder: strings.NewReader(tt.afterProbeBody),
					})
				}
				if tt.oneByteChunks {
					bodyReader = iotest.OneByteReader(bodyReader)
				}
				body := &responseBodyTracker{reader: bodyReader, closeErr: tt.closeErr}
				response := &http.Response{
					StatusCode:    tt.statusCode,
					ContentLength: tt.contentLength,
					Body:          body,
				}
				client := &http.Client{
					Timeout: time.Second,
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return response, nil
					}),
				}
				source, err := NewHTTPSource(
					"https://example.test/catalog",
					client,
					WithMaxResponseBytes(tt.maxResponseBytes),
				)
				if err != nil {
					t.Fatalf("NewHTTPSource() error = %v", err)
				}
				document, err := source.Fetch(t.Context())
				if tt.wantPrimary != nil && !errors.Is(err, tt.wantPrimary) {
					t.Fatalf("Fetch() error = %v, want primary %v", err, tt.wantPrimary)
				}
				if tt.probeErr != nil && !errors.Is(err, tt.probeErr) {
					t.Fatalf("Fetch() error = %v, want probe read error identity", err)
				}
				if tt.wantHTTPStatus != 0 {
					statusErr, statusErrMatched := errors.AsType[*httpStatusError](err)
					if !statusErrMatched || statusErr.status != tt.wantHTTPStatus {
						t.Fatalf("Fetch() error = %v, want HTTP %d status identity", err, tt.wantHTTPStatus)
					}
				}
				if tt.closeErr != nil && !errors.Is(err, tt.closeErr) {
					t.Fatalf("Fetch() error = %v, want close error identity", err)
				}
				if tt.wantValidationErr && err == nil {
					t.Fatal("Fetch() error = nil, want document validation failure")
				}
				if tt.wantDocument && document == nil {
					t.Fatal("Fetch() document = nil, want validated document")
				}
				if tt.wantDocument && tt.closeErr == nil && err != nil {
					t.Fatalf("Fetch() error = %v, want successful document fetch", err)
				}
				if got, want := body.bytesRead, tt.wantReadBytes; got != want {
					t.Fatalf("Fetch() body bytes read = %d, want %d", got, want)
				}
				if got, want := body.closeCount, 1; got != want {
					t.Fatalf("Fetch() body close count = %d, want %d", got, want)
				}
			})
		}
	})

	t.Run("Should preserve read drain and close errors from one response", func(t *testing.T) {
		t.Parallel()

		readErr := errors.New("read response")
		drainErr := errors.New("drain response")
		closeErr := errors.New("close response")
		body := &errorSequenceReadCloser{
			readErrors: []error{readErr, drainErr},
			closeErr:   closeErr,
		}
		client := &http.Client{
			Timeout: time.Second,
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: body}, nil
			}),
		}
		source, err := NewHTTPSource("https://example.test/catalog", client)
		if err != nil {
			t.Fatalf("NewHTTPSource() error = %v", err)
		}

		_, err = source.Fetch(t.Context())
		if !errors.Is(err, readErr) {
			t.Fatalf("Fetch() error = %v, want read error identity", err)
		}
		if !errors.Is(err, drainErr) {
			t.Fatalf("Fetch() error = %v, want drain error identity", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("Fetch() error = %v, want close error identity", err)
		}
		if got, want := body.closeCount, 1; got != want {
			t.Fatalf("Fetch() body close count = %d, want %d", got, want)
		}
		if got, want := body.readCount, 2; got != want {
			t.Fatalf("Fetch() body read count = %d, want %d", got, want)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type responseBodyTracker struct {
	reader     io.Reader
	closeErr   error
	bytesRead  int64
	closeCount int
}

func (b *responseBodyTracker) Read(destination []byte) (int, error) {
	read, err := b.reader.Read(destination)
	b.bytesRead += int64(read)
	return read, err
}

func (b *responseBodyTracker) Close() error {
	b.closeCount++
	return b.closeErr
}

type byteAndErrorReader struct {
	err       error
	remainder io.Reader
	delivered bool
}

func (r *byteAndErrorReader) Read(destination []byte) (int, error) {
	if !r.delivered {
		if len(destination) == 0 {
			return 0, nil
		}
		r.delivered = true
		destination[0] = 'x'
		return 1, r.err
	}
	return r.remainder.Read(destination)
}

type errorSequenceReadCloser struct {
	readErrors []error
	closeErr   error
	readCount  int
	closeCount int
}

func (b *errorSequenceReadCloser) Read([]byte) (int, error) {
	b.readCount++
	if len(b.readErrors) == 0 {
		return 0, io.EOF
	}
	err := b.readErrors[0]
	b.readErrors = b.readErrors[1:]
	return 0, err
}

func (b *errorSequenceReadCloser) Close() error {
	b.closeCount++
	return b.closeErr
}

func TestDecodeExtensionTimestamps(t *testing.T) {
	t.Parallel()

	t.Run("Should parse published and updated timestamps for an extension entry", func(t *testing.T) {
		t.Parallel()

		raw := extensionDocumentJSON(`,"published_at":"2026-07-01T00:00:00Z","updated_at":"2026-07-12T00:00:00Z"`)
		document, err := DecodeDocument([]byte(raw))
		if err != nil {
			t.Fatalf("DecodeDocument(extension) error = %v", err)
		}
		entry := document.Entries[0]
		if entry.PublishedAt == nil || entry.UpdatedAt == nil {
			t.Fatalf(
				"DecodeDocument(extension) timestamps = %#v/%#v, want both",
				entry.PublishedAt,
				entry.UpdatedAt,
			)
		}
	})

	t.Run("Should reject a document with trailing JSON values", func(t *testing.T) {
		t.Parallel()

		_, err := DecodeDocument([]byte(validExtensionDocumentJSON() + ` {}`))
		if err == nil || !strings.Contains(err.Error(), "multiple values") {
			t.Fatalf("DecodeDocument(trailing value) error = %v, want multiple-values diagnostic", err)
		}
	})
}

func validExtensionDocumentJSON() string {
	return extensionDocumentJSON(`,"format":"agent-plugin"`)
}

func extensionDocumentJSON(formatField string) string {
	return `{"manifest_version":3,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
		`"entry_id":"bridge-github","name":"GitHub bridge","description":"Connect GitHub events to Compozy",` +
		`"version":"1.0.0","install_slug":"compozy/bridge-github",` +
		`"artifact_url":"https://downloads.example.test/bridge-github-v1.0.0.tar.gz",` +
		`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
		`"tier":"official"` + formatField + `}]}`
}

// Invariant: v3 entries retain inputs and safe icons; invalid optional icons never hide the extension.
// Owner: catalog decoding. Canonical suite: source_test.go.
func TestDecodeV3ExtensionFields(t *testing.T) {
	t.Parallel()
	t.Run("Should decode v3 inputs and validate an extension-only family [UT-001]", func(t *testing.T) {
		t.Parallel()
		raw := v3ExtensionJSON(t, "https://images.example.test/icon.png")
		document, err := DecodeDocument(raw)
		if err != nil {
			t.Fatal(err)
		}
		entry := document.Entries[0]
		if entry.Icon != "https://images.example.test/icon.png" || len(entry.Inputs) != 2 {
			t.Fatalf("entry = %#v", entry)
		}
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "v3"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "v3", "extensions.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		presets := `{"manifest_version":3,"generated_at":"2026-09-12T10:00:00Z","entries":[]}`
		if err := os.WriteFile(filepath.Join(root, "v3", "marketplaces.json"), []byte(presets), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ValidateCatalogDirectory(root); err != nil {
			t.Fatal(err)
		}
	})
	for _, icon := range []string{"http://images.example.test/icon.png", "https://images.example.test/icon.gif", "data:image/png;base64," + strings.Repeat("A", 70*1024)} {
		t.Run("Should drop an unsafe icon with a diagnostic [UT-006]", func(t *testing.T) {
			t.Parallel()
			document, err := DecodeDocument(v3ExtensionJSON(t, icon))
			if err != nil {
				t.Fatal(err)
			}
			entry := document.Entries[0]
			if entry.Icon != "" || len(entry.Diagnostics) != 1 || entry.Diagnostics[0].Field != "icon" {
				t.Fatalf("entry = %#v", entry)
			}
			if strings.Contains(string(entry.Payload), icon) {
				t.Fatal("unsafe icon retained in payload")
			}
		})
	}
}

func v3ExtensionJSON(t *testing.T, icon string) []byte {
	t.Helper()
	rawIcon, err := json.Marshal(icon)
	if err != nil {
		t.Fatal(err)
	}
	fields := `,"icon":` + string(
		rawIcon,
	) + `,"inputs":[{"id":"region","prompt":"Region","type":"identifier","required":true,"binding":{"type":"env","name":"REGION"}},{"id":"debug","prompt":"Debug","type":"boolean","required":false,"default":false,"binding":{"type":"env","name":"DEBUG"}}]`
	return []byte(extensionDocumentJSON(fields))
}

// Invariant: extension discovery reads only v3 and never retries a retired root feed.
// Owner: HTTP feed family resolution. Canonical suite: source_test.go.
func TestHTTPSourceV3Family(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		status       int
		wrongVersion bool
		wantErr      bool
	}{
		{name: "prefer v3", status: 200},
		{name: "reject an absent family without root fallback", status: 404, wantErr: true},
		{name: "reject a removed family without root fallback", status: 410, wantErr: true},
		{name: "reject v3 server errors", status: 503, wantErr: true},
		{name: "reject a v2 document at the v3 address", status: 200, wrongVersion: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run("Should "+test.name+" [UT-055]", func(t *testing.T) {
			t.Parallel()
			v3 := v3ExtensionJSON(t, "")
			if test.wrongVersion {
				v3 = []byte(
					strings.Replace(validExtensionDocumentJSON(), `"manifest_version":3`, `"manifest_version":2`, 1),
				)
			}
			var rootCalls atomic.Int64
			var v3Calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/catalog/v3/extensions.json":
					v3Calls.Add(1)
					w.WriteHeader(test.status)
					if test.status == 200 {
						if _, err := w.Write(v3); err != nil {
							t.Errorf("write v3 response: %v", err)
						}
					}
				case "/catalog/extensions.json":
					rootCalls.Add(1)
					if _, err := io.WriteString(w, validExtensionDocumentJSON()); err != nil {
						t.Errorf("write retired response: %v", err)
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			source, err := NewHTTPSource(server.URL+"/catalog", &http.Client{Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			document, err := source.Fetch(t.Context())
			if (err != nil) != test.wantErr {
				t.Fatalf("fetch = %v", err)
			}
			if rootCalls.Load() != 0 || v3Calls.Load() != 1 {
				t.Fatalf("requests v3/root = %d/%d", v3Calls.Load(), rootCalls.Load())
			}
			if !test.wantErr && document.ManifestVersion != 3 {
				t.Fatalf("version = %d", document.ManifestVersion)
			}
		})
	}
}

func duplicateExtensionInstallSlugsJSON(t *testing.T) string {
	t.Helper()
	var document struct {
		ManifestVersion int              `json:"manifest_version"`
		GeneratedAt     string           `json:"generated_at"`
		Entries         []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal([]byte(validExtensionDocumentJSON()), &document); err != nil {
		t.Fatal(err)
	}
	entry := make(map[string]any)
	for key, value := range document.Entries[0] {
		entry[key] = value
	}
	entry["entry_id"] = "second"
	document.Entries = append(document.Entries, entry)
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
