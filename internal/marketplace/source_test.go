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
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

func TestDecodeDocumentValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a valid document for every curated kind", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			kind Kind
			raw  string
		}{
			{kind: KindMCP, raw: validMCPDocumentJSON()},
			{kind: KindExtension, raw: validExtensionDocumentJSON()},
			{kind: KindSkill, raw: validSkillDocumentJSON()},
		}
		for _, tt := range tests {
			t.Run("Should accept a valid "+string(tt.kind)+" document", func(t *testing.T) {
				t.Parallel()

				document, err := DecodeDocument(tt.kind, []byte(tt.raw))
				if err != nil {
					t.Fatalf("DecodeDocument(%q) error = %v", tt.kind, err)
				}
				if got, want := document.ManifestVersion, ManifestVersion; got != want {
					t.Fatalf("DecodeDocument(%q).ManifestVersion = %d, want %d", tt.kind, got, want)
				}
				if got, want := len(document.Entries), 1; got != want {
					t.Fatalf("DecodeDocument(%q) entries = %d, want %d", tt.kind, got, want)
				}
				if got := document.Entries[0].Kind; got != tt.kind {
					t.Fatalf("DecodeDocument(%q) entry kind = %q, want %q", tt.kind, got, tt.kind)
				}
			})
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
		if _, err := DecodeDocument(KindExtension, []byte(raw)); err != nil {
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

				document, err := DecodeDocument(KindExtension, []byte(tt.raw))
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

	t.Run("Should reject documents outside the v2 schema", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			kind    Kind
			raw     string
			wantErr string
		}{
			{
				name:    "unknown kind",
				kind:    Kind("artifact"),
				raw:     validSkillDocumentJSON(),
				wantErr: "unsupported kind",
			},
			{
				name:    "missing manifest version",
				kind:    KindMCP,
				raw:     `{"generated_at":"2026-07-13T00:00:00Z","entries":[]}`,
				wantErr: "manifest_version is required",
			},
			{
				name:    "missing entries array",
				kind:    KindMCP,
				raw:     `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z"}`,
				wantErr: "entries is required",
			},
			{
				name: "extension without digest",
				kind: KindExtension,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing digest",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz"}]}`,
				wantErr: "digest_sha256 is required",
			},
			{
				name: "extension without registry tier",
				kind: KindExtension,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing tier",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}]}`,
				wantErr: "tier is required",
			},
			{
				name: "extension without a curated artifact URL",
				kind: KindExtension,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing artifact",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"official"}]}`,
				wantErr: "artifact_url is required",
			},
			{
				name: "extension with a non HTTPS artifact URL",
				kind: KindExtension,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Insecure artifact",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"http://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"official"}]}`,
				wantErr: "artifact_url must be an absolute HTTPS URL",
			},
			{
				name: "extension with unknown registry tier",
				kind: KindExtension,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Unknown tier",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"partner"}]}`,
				wantErr: "unsupported tier",
			},
			{
				name: "extension with unsupported format marker",
				kind: KindExtension,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Unknown format",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"official","format":"claude-plugin"}]}`,
				wantErr: "unsupported format",
			},
			{
				name: "skill with invented trust field",
				kind: KindSkill,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"skill","name":"Skill","description":"Invalid trust",` +
					`"install_slug":"compozy/skill","verified":true}]}`,
				wantErr: "unknown field",
			},
			{
				name: "skill using the synthetic remote ID namespace",
				kind: KindSkill,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"skill_QGFjbWUvc2tpbGw","name":"Colliding skill",` +
					`"description":"Cannot shadow a synthetic ID","install_slug":"@acme/skill"}]}`,
				wantErr: "reserved prefix",
			},
			{
				name: "duplicate skill install slugs",
				kind: KindSkill,
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[` +
					`{"entry_id":"first","name":"First","description":"First skill","install_slug":"@acme/shared"},` +
					`{"entry_id":"second","name":"Second","description":"Second skill","install_slug":"@acme/shared"}]}`,
				wantErr: `install_slug "@acme/shared" is duplicated`,
			},
		}
		for _, tt := range tests {
			t.Run("Should reject "+tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := DecodeDocument(tt.kind, []byte(tt.raw))
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
			KindMCP,
			fmt.Appendf(
				nil,
				`{"manifest_version":%d,"generated_at":"2026-07-13T00:00:00Z","entries":[]}`,
				ManifestVersion+1,
			),
		)
		if err == nil {
			t.Fatal("DecodeDocument() error = nil, want unsupported manifest error")
		}
		var versionErr *UnsupportedManifestVersionError
		if !errors.As(err, &versionErr) {
			t.Fatalf("DecodeDocument() error = %T, want UnsupportedManifestVersionError", err)
		}
		if !strings.Contains(err.Error(), "client too old") {
			t.Fatalf("DecodeDocument() error = %v, want client-too-old diagnostic", err)
		}
	})
}

func TestDecodeMCPV2LaunchAndInputValidation(t *testing.T) {
	t.Parallel()

	validEntry := func(launch string, inputs string) string {
		return `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"server","name":"Server","description":"Validated launch",` +
			launch + `,"inputs":` + inputs + `,"default_scope":"workspace"}]}`
	}

	t.Run("Should accept every typed launch distribution", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			raw      string
			wantArgs []string
		}{
			{
				name: "Should accept npm launch arguments as literal argv",
				raw: validEntry(
					`"launch":{"type":"npm","package":"@acme/server","version":"1.2.3","args":["--read-only","path with spaces"]}`,
					`[]`,
				),
				wantArgs: []string{"--read-only", "path with spaces"},
			},
			{
				name: "Should accept uvx launch",
				raw: validEntry(
					`"launch":{"type":"uvx","package":"mcp-server","version":"1.2.3"}`,
					`[]`,
				),
			},
			{
				name: "Should accept digest pinned docker launch",
				raw: validEntry(
					`"launch":{"type":"docker","image":"ghcr.io/acme/server","digest":"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`,
					`[]`,
				),
			},
			{
				name: "Should accept remote launch with non-secret query input",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://mcp.example.test"}`,
					`[{"id":"project_ref","prompt":"Project reference","type":"identifier","required":true,"binding":{"type":"url_query","name":"project_ref"}}]`,
				),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				document, err := DecodeDocument(KindMCP, []byte(tt.raw))
				if err != nil {
					t.Fatalf("DecodeDocument() error = %v", err)
				}
				if tt.wantArgs != nil {
					details, projectErr := ProjectEntry(document.Entries[0])
					if projectErr != nil {
						t.Fatalf("ProjectEntry() error = %v", projectErr)
					}
					if got := details.MCP.Launch.Args; !slices.Equal(got, tt.wantArgs) {
						t.Fatalf("ProjectEntry() launch args = %#v, want %#v", got, tt.wantArgs)
					}
				}
			})
		}
	})

	t.Run("Should accept a public remote IP literal", func(t *testing.T) {
		t.Parallel()

		for _, rawURL := range []string{"https://8.8.8.8/mcp", "https://[2001:4860:4860::8888]/mcp"} {
			t.Run("Should accept "+rawURL, func(t *testing.T) {
				t.Parallel()

				_, err := DecodeDocument(KindMCP, []byte(validEntry(
					`"launch":{"type":"remote","url":"`+rawURL+`"}`,
					`[]`,
				)))
				if err != nil {
					t.Fatalf("DecodeDocument(%q) error = %v", rawURL, err)
				}
			})
		}
	})

	t.Run("Should reject unsafe or legacy MCP v2 declarations", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			raw     string
			wantErr string
		}{
			{
				name:    "Should reject a v1 manifest",
				raw:     `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[]}`,
				wantErr: "feed too old",
			},
			{
				name: "Should reject legacy MCP launch fields",
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"legacy","name":"Legacy","description":"Unsupported contract",` +
					`"transport":"stdio","command":"server","default_scope":"workspace"}]}`,
				wantErr: "unknown field",
			},
			{
				name: "Should reject an insecure remote URL",
				raw: validEntry(
					`"launch":{"type":"remote","url":"http://mcp.example.test"}`,
					`[]`,
				),
				wantErr: "absolute HTTPS URL",
			},
			{
				name: "Should reject an explicit localhost remote URL",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://mcp.localhost/mcp"}`,
					`[]`,
				),
				wantErr: "public host",
			},
			{
				name: "Should reject an obviously local remote hostname",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://mcp.internal/mcp"}`,
					`[]`,
				),
				wantErr: "public host",
			},
			{
				name: "Should reject a single-label remote hostname",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://mcp/mcp"}`,
					`[]`,
				),
				wantErr: "public host",
			},
			{
				name: "Should reject a loopback remote IP literal",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://127.0.0.1/mcp"}`,
					`[]`,
				),
				wantErr: "public HTTPS destination",
			},
			{
				name: "Should reject an unspecified remote IP literal",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://0.0.0.0/mcp"}`,
					`[]`,
				),
				wantErr: "public HTTPS destination",
			},
			{
				name: "Should reject a private remote IP literal",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://10.0.0.8/mcp"}`,
					`[]`,
				),
				wantErr: "public HTTPS destination",
			},
			{
				name: "Should reject a link-local remote IP literal",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://169.254.169.254/mcp"}`,
					`[]`,
				),
				wantErr: "public HTTPS destination",
			},
			{
				name: "Should reject a multicast remote IP literal",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://224.0.0.1/mcp"}`,
					`[]`,
				),
				wantErr: "public HTTPS destination",
			},
			{
				name: "Should reject a documentation remote IP literal",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://192.0.2.1/mcp"}`,
					`[]`,
				),
				wantErr: "public HTTPS destination",
			},
			{
				name: "Should reject a special-use remote IP literal",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://100.64.0.1/mcp"}`,
					`[]`,
				),
				wantErr: "public HTTPS destination",
			},
			{
				name: "Should reject duplicated input destinations",
				raw: validEntry(
					`"launch":{"type":"npm","package":"acme-server","version":"1.2.3"}`,
					`[{"id":"first","prompt":"First","type":"string","required":false,"binding":{"type":"env","name":"SERVER_MODE"}},{"id":"second","prompt":"Second","type":"string","required":false,"binding":{"type":"env","name":"SERVER_MODE"}}]`,
				),
				wantErr: "binding env/\"SERVER_MODE\" is duplicated",
			},
			{
				name: "Should reject option injection in package names",
				raw: validEntry(
					`"launch":{"type":"npm","package":"-y","version":"1.2.3"}`,
					`[]`,
				),
				wantErr: "exact package name",
			},
			{
				name: "Should reject an npm-scoped package in a uvx launch",
				raw: validEntry(
					`"launch":{"type":"uvx","package":"@acme/server","version":"1.2.3"}`,
					`[]`,
				),
				wantErr: "exact package name",
			},
			{
				name: "Should reject tagged docker images",
				raw: validEntry(
					`"launch":{"type":"docker","image":"ghcr.io/acme/server:latest","digest":"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`,
					`[]`,
				),
				wantErr: "exact untagged image name",
			},
			{
				name: "Should reject secret URL query inputs",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://mcp.example.test"}`,
					`[{"id":"token","prompt":"Access token","type":"secret","required":true,"binding":{"type":"url_query","name":"token"}}]`,
				),
				wantErr: "must not bind a secret",
			},
			{
				name: "Should reject query bindings that override launch configuration",
				raw: validEntry(
					`"launch":{"type":"remote","url":"https://mcp.example.test?read_only=true"}`,
					`[{"id":"read_only","prompt":"Read only","type":"boolean","required":false,"binding":{"type":"url_query","name":"read_only"}}]`,
				),
				wantErr: "conflicts with launch URL",
			},
			{
				name: "Should reject a missing default scope",
				raw: `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"server","name":"Server","description":"Validated launch",` +
					`"launch":{"type":"npm","package":"acme-server","version":"1.2.3"}}]}`,
				wantErr: "default_scope must be workspace or global",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := DecodeDocument(KindMCP, []byte(tt.raw))
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("DecodeDocument() error = %v, want %q", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("Should reject a malformed typed default", func(t *testing.T) {
		t.Parallel()

		input := mcpInput{Type: mcpInputTypeBoolean, Default: json.RawMessage(`true false`)}
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
			input   mcpInput
			wantErr string
		}{
			{
				name: "Should reject NUL in an identifier default",
				input: mcpInput{
					Type: mcpInputTypeIdentifier, Default: json.RawMessage(`"unsafe\u0000identifier"`),
				},
				wantErr: "must not contain NUL",
			},
			{
				name:    "Should reject an oversized string default",
				input:   mcpInput{Type: mcpInputTypeString, Default: oversized},
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
}

func TestNormalizeMCPInputValue(t *testing.T) {
	t.Parallel()

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

		catalogDir := filepath.Join("..", "..", "catalog")
		for _, kind := range AllKinds() {
			filename, err := kindFilename(kind)
			if err != nil {
				t.Fatalf("kindFilename(%q) error = %v", kind, err)
			}
			raw, err := os.ReadFile(filepath.Join(catalogDir, filename))
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", filename, err)
			}
			document, err := DecodeDocument(kind, raw)
			if err != nil {
				t.Fatalf("DecodeDocument(%q) error = %v", kind, err)
			}
			if len(document.Entries) == 0 {
				t.Fatalf("production catalog %q entries = 0, want at least one curated launch entry", kind)
			}
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

	t.Run("Should fetch a checkout document through a file URL", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "skills.json")
		if err := os.WriteFile(path, []byte(validSkillDocumentJSON()), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
		baseURL := (&url.URL{Scheme: "file", Path: dir}).String()
		source, err := NewSource(KindSkill, baseURL, &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatalf("NewSource(file) error = %v", err)
		}
		document, err := source.Fetch(t.Context())
		if err != nil {
			t.Fatalf("Fetch(file) error = %v", err)
		}
		if got, want := document.Entries[0].EntryID, "compozy"; got != want {
			t.Fatalf("Fetch(file) entry id = %q, want %q", got, want)
		}
	})

	t.Run("Should reject an oversized checkout document before decoding", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "skills.json")
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
			KindSkill,
			(&url.URL{Scheme: "file", Path: dir}).String(),
		)
		if err != nil {
			t.Fatalf("NewDirectorySource() error = %v", err)
		}
		if _, err := source.Fetch(t.Context()); !errors.Is(err, ErrResponseTooLarge) {
			t.Fatalf("Fetch(oversized file) error = %v, want ErrResponseTooLarge", err)
		}
	})

	t.Run("Should fetch one kind with the injected timeout client", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.URL.Path, "/mcp.json"; got != want {
				t.Errorf("request path = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(validMCPDocumentJSON())); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		t.Cleanup(server.Close)

		client := &http.Client{Timeout: time.Second}
		source, err := NewHTTPSource(KindMCP, server.URL, client)
		if err != nil {
			t.Fatalf("NewHTTPSource() error = %v", err)
		}
		document, err := source.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if got, want := document.Entries[0].EntryID, "filesystem"; got != want {
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
		source, err := NewHTTPSource(KindMCP, "https://example.test/catalog", client)
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
			KindMCP,
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

		_, err := NewHTTPSource(KindMCP, "https://example.test/catalog", &http.Client{})
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
		source, err := NewHTTPSource(KindSkill, server.URL, &http.Client{Timeout: time.Second})
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

		_, err := NewHTTPSource(KindSkill, "relative/catalog", &http.Client{Timeout: time.Second})
		if err == nil || !strings.Contains(err.Error(), "absolute HTTP(S) URL") {
			t.Fatalf("NewHTTPSource(relative URL) error = %v, want absolute-URL validation", err)
		}
	})

	t.Run("Should reject a missing HTTP client", func(t *testing.T) {
		t.Parallel()

		_, err := NewHTTPSource(KindSkill, "https://example.test", nil)
		if err == nil || !strings.Contains(err.Error(), "HTTP client timeout must be positive") {
			t.Fatalf("NewHTTPSource(nil client) error = %v, want client validation", err)
		}
	})

	t.Run("Should reject a nil fetch context", func(t *testing.T) {
		t.Parallel()

		source, err := NewHTTPSource(KindSkill, "https://example.test", &http.Client{Timeout: time.Second})
		if err != nil {
			t.Fatalf("NewHTTPSource(valid) error = %v", err)
		}
		if got := source.Kind(); got != KindSkill {
			t.Fatalf("Kind() = %q, want %q", got, KindSkill)
		}
		//nolint:staticcheck // Explicitly verifies the public nil-context guard.
		if _, err := source.Fetch(nil); err == nil || !strings.Contains(err.Error(), "fetch context is required") {
			t.Fatalf("Fetch(nil context) error = %v, want context validation", err)
		}
	})

	t.Run("Should reject a nil HTTP source receiver", func(t *testing.T) {
		t.Parallel()

		var nilSource *HTTPSource
		if got := nilSource.Kind(); got != "" {
			t.Fatalf("nil Kind() = %q, want empty", got)
		}
		_, err := nilSource.Fetch(context.Background())
		if err == nil || !strings.Contains(err.Error(), "HTTP source is required") {
			t.Fatalf("nil Fetch() error = %v, want source validation", err)
		}
	})

	t.Run("Should enforce response bounds and close every acquired HTTP response", func(t *testing.T) {
		t.Parallel()

		const drainLimit int64 = 64 << 10
		validDocument := validMCPDocumentJSON()
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
					KindMCP,
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
		source, err := NewHTTPSource(KindMCP, "https://example.test/catalog", client)
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

func TestDecodeRemoteMCPAndTimestamps(t *testing.T) {
	t.Parallel()

	t.Run("Should parse published and updated timestamps for a remote MCP entry", func(t *testing.T) {
		t.Parallel()

		raw := `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"remote","name":"Remote","description":"OAuth remote",` +
			`"published_at":"2026-07-01T00:00:00Z","updated_at":"2026-07-12T00:00:00Z",` +
			`"launch":{"type":"remote","url":"https://mcp.example.test/v1"},` +
			`"auth":{"method":"oauth","registration":"auto","scopes":["mcp.read"]},` +
			`"default_scope":"workspace"}]}`
		document, err := DecodeDocument(KindMCP, []byte(raw))
		if err != nil {
			t.Fatalf("DecodeDocument(remote MCP) error = %v", err)
		}
		entry := document.Entries[0]
		if entry.PublishedAt == nil || entry.UpdatedAt == nil {
			t.Fatalf(
				"DecodeDocument(remote MCP) timestamps = %#v/%#v, want both",
				entry.PublishedAt,
				entry.UpdatedAt,
			)
		}
	})

	t.Run("Should reject a document with trailing JSON values", func(t *testing.T) {
		t.Parallel()

		_, err := DecodeDocument(KindSkill, []byte(validSkillDocumentJSON()+` {}`))
		if err == nil || !strings.Contains(err.Error(), "multiple values") {
			t.Fatalf("DecodeDocument(trailing value) error = %v, want multiple-values diagnostic", err)
		}
	})
}

func validMCPDocumentJSON() string {
	return `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
		`"entry_id":"filesystem","name":"Filesystem","description":"Read and write approved local files",` +
		`"version":"1.0.0","launch":{"type":"npm","package":"@modelcontextprotocol/server-filesystem",` +
		`"version":"1.0.0"},"inputs":[{"id":"root_path","prompt":"Allowed root",` +
		`"type":"string","required":true,"binding":{"type":"env","name":"ROOT_PATH"}}],` +
		`"default_scope":"workspace"` +
		`}]}`
}

func validExtensionDocumentJSON() string {
	return extensionDocumentJSON(`,"format":"agent-plugin"`)
}

func extensionDocumentJSON(formatField string) string {
	return `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
		`"entry_id":"bridge-github","name":"GitHub bridge","description":"Connect GitHub events to Compozy",` +
		`"version":"1.0.0","install_slug":"compozy/bridge-github",` +
		`"artifact_url":"https://downloads.example.test/bridge-github-v1.0.0.tar.gz",` +
		`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
		`"tier":"official"` + formatField + `}]}`
}

func validSkillDocumentJSON() string {
	return `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
		`"entry_id":"compozy","name":"Compozy","display_name":"Compozy operator",` +
		`"description":"Operate Compozy through its structured surfaces","version":"1.0.0",` +
		`"install_slug":"compozy/compozy","author":"Compozy","tags":["compozy","operations"]` +
		`}]}`
}
