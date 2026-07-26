package marketplace

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
				if got, want := document.ManifestVersion, 1; got != want {
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

	t.Run("Should reject documents outside the v1 schema", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			kind    Kind
			raw     string
			wantErr string
		}{
			{
				name:    "unknown kind",
				kind:    Kind("bundle"),
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
				raw:     `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z"}`,
				wantErr: "entries is required",
			},
			{
				name: "stdio and remote transport fields together",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"mixed","name":"Mixed","description":"Invalid mixed transport",` +
					`"transport":"stdio","command":"server","url":"https://example.test/mcp"}]}`,
				wantErr: "must not set url",
			},
			{
				name: "oauth without client id",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote","name":"Remote","description":"Remote MCP",` +
					`"transport":"http","url":"https://example.test/mcp","oauth":{"issuer_url":"https://example.test"}}]}`,
				wantErr: "oauth.client_id is required",
			},
			{
				name: "oauth without a metadata source or direct endpoint pair",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote","name":"Remote","description":"Remote MCP",` +
					`"transport":"http","url":"https://example.test/mcp","oauth":{"client_id":"client"}}]}`,
				wantErr: "requires issuer_url or both authorization_url and token_url",
			},
			{
				name: "oauth with an incomplete direct endpoint pair",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote","name":"Remote","description":"Remote MCP",` +
					`"transport":"http","url":"https://example.test/mcp","oauth":{` +
					`"client_id":"client","authorization_url":"https://auth.example/authorize"}}]}`,
				wantErr: "requires issuer_url or both authorization_url and token_url",
			},
			{
				name: "oauth with a relative issuer URL",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote","name":"Remote","description":"Remote MCP",` +
					`"transport":"http","url":"https://example.test/mcp","oauth":{` +
					`"client_id":"client","issuer_url":"/oauth"}}]}`,
				wantErr: "oauth.issuer_url must be an absolute HTTP(S) URL",
			},
			{
				name: "oauth with a non-HTTP token URL",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote","name":"Remote","description":"Remote MCP",` +
					`"transport":"http","url":"https://example.test/mcp","oauth":{` +
					`"client_id":"client","authorization_url":"https://auth.example/authorize",` +
					`"token_url":"file:///tmp/token"}}]}`,
				wantErr: "oauth.token_url must be an absolute HTTP(S) URL",
			},
			{
				name: "secret env field with plaintext default",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"secret-default","name":"Secret default","description":"Unsafe secret",` +
					`"transport":"stdio","command":"server","env":[{` +
					`"name":"API_TOKEN","required":true,"secret":true,"default":"plaintext"}]}]}`,
				wantErr: "must not set default for secret fields",
			},
			{
				name: "forbidden stdio environment key",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"node-options","name":"Node options","description":"Unsafe injection",` +
					`"transport":"stdio","command":"server","env":[{` +
					`"name":"NODE_OPTIONS","required":false,"secret":false}]}]}`,
				wantErr: "is forbidden for stdio MCP servers",
			},
			{
				name: "secret-like plain stdio environment key",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"plain-token","name":"Plain token","description":"Unsafe plaintext",` +
					`"transport":"stdio","command":"server","env":[{` +
					`"name":"API_TOKEN","required":true,"secret":false}]}]}`,
				wantErr: "must move secret-like values to secret_env",
			},
			{
				name: "non-loopback plaintext OAuth endpoint",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote-http","name":"Remote HTTP","description":"Insecure OAuth",` +
					`"transport":"http","url":"https://example.test/mcp","oauth":{` +
					`"client_id":"client","authorization_url":"http://auth.example/authorize",` +
					`"token_url":"https://auth.example/token"}}]}`,
				wantErr: "must use https unless host is loopback",
			},
			{
				name: "missing common identity fields deterministically",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"transport":"stdio","command":"server"}]}`,
				wantErr: "entry entry_id is required",
			},
			{
				name: "entry id containing a path separator",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"nested/server","name":"Nested","description":"Unsafe route identity",` +
					`"transport":"stdio","command":"server"}]}`,
				wantErr: "one URL-safe path segment",
			},
			{
				name: "reserved current-directory entry id",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":".","name":"Current","description":"Ambiguous route identity",` +
					`"transport":"stdio","command":"server"}]}`,
				wantErr: "one URL-safe path segment",
			},
			{
				name: "reserved parent-directory entry id",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"..","name":"Parent","description":"Ambiguous route identity",` +
					`"transport":"stdio","command":"server"}]}`,
				wantErr: "one URL-safe path segment",
			},
			{
				name: "MCP URL containing credentials",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote","name":"Remote","description":"Credential-bearing URL",` +
					`"transport":"http","url":"https://user:password@example.test/mcp"}]}`,
				wantErr: "without credentials",
			},
			{
				name: "OAuth URL containing credentials",
				kind: KindMCP,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"remote","name":"Remote","description":"Credential-bearing OAuth URL",` +
					`"transport":"http","url":"https://example.test/mcp","oauth":{` +
					`"client_id":"client","issuer_url":"https://user:password@auth.example.test"}}]}`,
				wantErr: "without credentials",
			},
			{
				name: "extension without digest",
				kind: KindExtension,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing digest",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz"}]}`,
				wantErr: "digest_sha256 is required",
			},
			{
				name: "extension without registry tier",
				kind: KindExtension,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing tier",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}]}`,
				wantErr: "tier is required",
			},
			{
				name: "extension without a curated artifact URL",
				kind: KindExtension,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Missing artifact",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"official"}]}`,
				wantErr: "artifact_url is required",
			},
			{
				name: "extension with a non HTTPS artifact URL",
				kind: KindExtension,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
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
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"extension","name":"Extension","description":"Unknown tier",` +
					`"version":"1.0.0","install_slug":"compozy/extension",` +
					`"artifact_url":"https://downloads.example.test/extension-v1.0.0.tar.gz",` +
					`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
					`"tier":"partner"}]}`,
				wantErr: "unsupported tier",
			},
			{
				name: "skill with invented trust field",
				kind: KindSkill,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"skill","name":"Skill","description":"Invalid trust",` +
					`"install_slug":"compozy/skill","verified":true}]}`,
				wantErr: "unknown field",
			},
			{
				name: "skill using the synthetic remote ID namespace",
				kind: KindSkill,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
					`"entry_id":"skill_QGFjbWUvc2tpbGw","name":"Colliding skill",` +
					`"description":"Cannot shadow a synthetic ID","install_slug":"@acme/skill"}]}`,
				wantErr: "reserved prefix",
			},
			{
				name: "duplicate skill install slugs",
				kind: KindSkill,
				raw: `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[` +
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
			[]byte(`{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[]}`),
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

func TestHTTPSourceFetch(t *testing.T) {
	t.Parallel()

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
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestDecodeRemoteMCPAndTimestamps(t *testing.T) {
	t.Parallel()

	t.Run("Should parse published and updated timestamps for a remote MCP entry", func(t *testing.T) {
		t.Parallel()

		raw := `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"remote","name":"Remote","description":"OAuth remote",` +
			`"published_at":"2026-07-01T00:00:00Z","updated_at":"2026-07-12T00:00:00Z",` +
			`"transport":"http","url":"https://mcp.example.test/v1",` +
			`"oauth":{"issuer_url":"https://auth.example.test","client_id":"agh-desktop","scopes":["mcp.read"]},` +
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
	return `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
		`"entry_id":"filesystem","name":"Filesystem","description":"Read and write approved local files",` +
		`"version":"1.0.0","transport":"stdio","command":"npx",` +
		`"args":["-y","@modelcontextprotocol/server-filesystem"],` +
		`"env":[{"name":"ROOT_PATH","prompt":"Allowed root","required":true,"secret":false}]` +
		`}]}`
}

func validExtensionDocumentJSON() string {
	return `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
		`"entry_id":"bridge-github","name":"GitHub bridge","description":"Connect GitHub events to AGH",` +
		`"version":"1.0.0","install_slug":"compozy/bridge-github",` +
		`"artifact_url":"https://downloads.example.test/bridge-github-v1.0.0.tar.gz",` +
		`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
		`"tier":"official"}]}`
}

func validSkillDocumentJSON() string {
	return `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
		`"entry_id":"agh","name":"AGH","display_name":"AGH operator",` +
		`"description":"Operate AGH through its structured surfaces","version":"1.0.0",` +
		`"install_slug":"compozy/agh","author":"Compozy","tags":["agh","operations"]` +
		`}]}`
}
