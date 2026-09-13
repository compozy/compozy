package pluginsource

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadMarketplaceDirectory(t *testing.T) {
	t.Parallel()
	for _, location := range []string{"marketplace.json", ".claude-plugin/marketplace.json"} {
		t.Run("Should read the document at "+location, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			raw := []byte(`{"name":"team","plugins":[{"name":"plugin","source":"./plugin"}]}`)
			writeMarketplaceDocument(t, root, location, raw)
			doc, err := ReadDirectory(t.Context(), root)
			if err != nil || doc.Path != location || doc.Name != "team" || len(doc.Plugins) != 1 ||
				doc.DigestSHA256 != cacheDigest(raw) {
				t.Fatalf("ReadDirectory = %+v, %v", doc, err)
			}
		})
	}
	t.Run("Should use root precedence and refuse to hide a malformed root document", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeMarketplaceDocument(t, root, "marketplace.json", []byte(`{"name":"root","plugins":[]}`))
		writeMarketplaceDocument(t, root, ".claude-plugin/marketplace.json", []byte(`{"name":"nested","plugins":[]}`))
		doc, err := ReadDirectory(t.Context(), root)
		if err != nil || doc.Name != "root" || doc.Path != "marketplace.json" {
			t.Fatalf("root precedence = %+v, %v", doc, err)
		}
		writeMarketplaceDocument(t, root, "marketplace.json", []byte(`{"plugins":`))
		if _, err := ReadDirectory(t.Context(), root); !errors.Is(err, ErrNotMarketplace) {
			t.Fatalf("malformed root = %v", err)
		}
	})
	t.Run("Should distinguish missing documents from an unreachable source", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		for _, createChild := range []bool{false, true} {
			if createChild {
				if err := os.Mkdir(filepath.Join(root, ".claude-plugin"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			_, err := ReadDirectory(t.Context(), root)
			missing, ok := errors.AsType[*NotMarketplaceError](err)
			if !ok || !errors.Is(err, ErrNotMarketplace) || errors.Is(err, ErrSourceUnreachable) ||
				!reflect.DeepEqual(missing.Checked, []string{"marketplace.json", ".claude-plugin/marketplace.json"}) {
				t.Fatalf("missing document = %v", err)
			}
		}
		if _, err := ReadDirectory(t.Context(), filepath.Join(root, "absent")); !errors.Is(err, ErrSourceUnreachable) ||
			!errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrNotMarketplace) {
			t.Fatalf("unreachable root = %v", err)
		}
	})
	t.Run("Should refuse documents and document directories that escape through symlinks", func(t *testing.T) {
		t.Parallel()
		outside := t.TempDir()
		writeMarketplaceDocument(t, outside, "marketplace.json", []byte(`{"plugins":[]}`))
		for _, location := range []string{"marketplace.json", ".claude-plugin"} {
			root := t.TempDir()
			target := outside
			if location == "marketplace.json" {
				target = filepath.Join(outside, location)
			}
			if err := os.Symlink(target, filepath.Join(root, location)); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadDirectory(t.Context(), root); !errors.Is(err, ErrSourceUnreachable) {
				t.Fatalf("symlink %s = %v", location, err)
			}
		}
	})
	t.Run("Should bound file reads and honor cancellation before opening a source", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeMarketplaceDocument(t, root, "marketplace.json", []byte(strings.Repeat(" ", MaxDocumentBytes+1)))
		if _, err := ReadDirectory(t.Context(), root); !errors.Is(err, ErrDocumentTooLarge) {
			t.Fatalf("oversized file = %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := ReadDirectory(ctx, filepath.Join(root, "absent")); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled read = %v", err)
		}
	})
}

func writeMarketplaceDocument(t *testing.T, root, location string, raw []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(location))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeRef(t *testing.T) {
	t.Parallel()
	t.Run("Should join equivalent repository spellings into one origin", func(t *testing.T) {
		t.Parallel()
		for _, input := range []string{
			"owner/repo", "github:owner/repo", "https://github.com/owner/repo",
			" git+https://GitHub.com/Owner/Repo.git/ ", "https://github.com:443/OWNER/REPO",
		} {
			got, err := NormalizeRef(input)
			if err != nil || got != "github:owner/repo" {
				t.Fatalf("NormalizeRef(%q) = %q, %v", input, got, err)
			}
		}
	})
	t.Run("Should preserve folder and generic Git source identity", func(t *testing.T) {
		t.Parallel()
		folder := filepath.Join(t.TempDir(), "folder with spaces")
		want := (&url.URL{Scheme: "file", Path: filepath.ToSlash(folder)}).String()
		for _, input := range []string{folder, want, folder + string(filepath.Separator)} {
			got, err := NormalizeRef(input)
			if err != nil || got != want {
				t.Fatalf("NormalizeRef(%q) = %q, %v", input, got, err)
			}
		}
		got, err := NormalizeRef("git+https://GitLab.com/Team/Repo.git/")
		if err != nil || got != "git+https://gitlab.com/Team/Repo.git" {
			t.Fatalf("Git reference = %q, %v", got, err)
		}
	})
	t.Run("Should reject ambiguous credentials and unsupported source references", func(t *testing.T) {
		t.Parallel()
		for _, input := range []string{
			"", "owner", "owner/repo/child", "../repo", "github:owner/..", "github:owner/repo?token=x",
			"https://user:secret@github.com/owner/repo", "https://github.com/owner/repo#main",
			"https://github.com/owner/repo?", "https://github.com/owner/repo#",
			"https://gitlab.com/team/repo", "git+http://gitlab.com/team/repo",
			"git+https://127.0.0.1/repo", "git+https://example.com/", "git+https://example.com:0/repo",
			"file://remote/path", "file:relative", "file:///tmp/%00", "/tmp/\x00", "file:///tmp/a?token=x",
			"ssh://git@github.com/owner/repo", "https://github.com/%zz",
		} {
			if got, err := NormalizeRef(input); !errors.Is(err, ErrInvalidRef) || got != "" {
				t.Fatalf("NormalizeRef(%q) = %q, %v", input, got, err)
			}
		}
	})
}

func TestDecodePluginDocument(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve plugin metadata and every supported source form", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"name":"team","owner":{"name":"Team","email":"team@example.com"},"future":true,
		"plugins":[{"name":"local","source":"./plugins/local","description":"Local tools","version":"1.2.3",
		"author":{"name":"Author"},"homepage":"https://example.com","repository":"https://github.com/o/r",
		"license":"MIT","keywords":["tools","local"],"category":"developer","icon":"icon.svg","unknown":{}},
		{"name":"remote","source":{"source":"github","repo":"O/R","ref":"stable","path":"plugins/tool","future":42}},
		{"name":"archive","source":"https://example.com/plugin.tar.gz","author":"Other"}]}`)
		doc, err := DecodeDocument(raw)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Name != "team" || doc.Owner != "Team" || doc.DigestSHA256 != cacheDigest(raw) ||
			len(doc.Plugins) != 3 || len(doc.Diagnostics) != 0 {
			t.Fatalf("document = %+v", doc)
		}
		want := Plugin{
			Name: "local", Description: "Local tools", Version: "1.2.3", Author: "Author",
			Homepage: "https://example.com", Repository: "https://github.com/o/r", License: "MIT",
			Keywords: []string{"tools", "local"}, Category: "developer", Icon: "icon.svg",
			Source: PluginSource{Kind: SourceRelative, Path: "./plugins/local"},
		}
		if !reflect.DeepEqual(doc.Plugins[0], want) {
			t.Fatalf("local plugin = %+v", doc.Plugins[0])
		}
		if doc.Plugins[1].Source != (PluginSource{Kind: SourceGitHub, Ref: "github:o/r", GitRef: "stable", Path: "plugins/tool"}) {
			t.Fatalf("GitHub source = %+v", doc.Plugins[1].Source)
		}
		if doc.Plugins[2].Source != (PluginSource{Kind: SourceHTTPS, Ref: "https://example.com/plugin.tar.gz"}) ||
			doc.Plugins[2].Author != "Other" {
			t.Fatalf("archive plugin = %+v", doc.Plugins[2])
		}
	})
	t.Run("Should ignore optional metadata drift and retain paths for confinement at resolution", func(t *testing.T) {
		t.Parallel()
		doc, err := DecodeDocument([]byte(`{"name":42,"owner":[],"plugins":[{"name":"tool","source":"../outside",
		"description":false,"author":{"name":7},"keywords":["valid",42]}]}`))
		if err != nil || len(doc.Plugins) != 1 || len(doc.Diagnostics) != 0 {
			t.Fatalf("document = %+v, %v", doc, err)
		}
		plugin := doc.Plugins[0]
		if plugin.Source.Path != "../outside" || plugin.Description != "" || plugin.Author != "" ||
			plugin.Keywords != nil {
			t.Fatalf("plugin = %+v", plugin)
		}
	})
	t.Run("Should drop invalid and duplicate plugins without hiding healthy neighbors", func(t *testing.T) {
		t.Parallel()
		doc, err := DecodeDocument([]byte(`{"plugins":[null,42,{"name":"bad/name","source":"."},
		{"name":"first","source":"."},{"name":"first","source":"./other"},
		{"name":"missing"},{"name":"unknown","source":{"source":"npm","package":"x"}},
		{"name":"repo","source":{"source":"github","repo":"owner"}},
		{"name":"typed","source":{"source":"github","repo":"o/r","ref":42}},
		{"name":"last","source":"./last"}]}`))
		if err != nil || len(doc.Plugins) != 2 || len(doc.Diagnostics) != 8 {
			t.Fatalf("document = %+v, %v", doc, err)
		}
		if doc.Plugins[0].Name != "first" || doc.Plugins[1].Name != "last" ||
			doc.Diagnostics[3].Code != "duplicate_plugin" || doc.Diagnostics[4].Code != "unsupported_source" ||
			doc.Diagnostics[4].Plugin != "missing" {
			t.Fatalf("document = %+v", doc)
		}
	})
	t.Run("Should reject unsupported string sources without exposing supplied credentials", func(t *testing.T) {
		t.Parallel()
		doc, err := DecodeDocument([]byte(`{"plugins":[
		{"name":"auth","source":"https://user:secret@example.com/repo"},
		{"name":"http","source":"http://example.com/repo"},
		{"name":"local","source":"/outside"},
		{"name":"windows","source":"..\\outside"},
		{"name":"host","source":"//example.com/repo"},
		{"name":"null","source":"\u0000"},
		{"name":"empty","source":" "} ]}`))
		if err != nil || len(doc.Plugins) != 0 || len(doc.Diagnostics) != 7 {
			t.Fatalf("document = %+v, %v", doc, err)
		}
		for _, diagnostic := range doc.Diagnostics {
			if diagnostic.Code != "unsupported_source" || strings.Contains(diagnostic.Message, "secret") {
				t.Fatalf("diagnostic = %+v", diagnostic)
			}
		}
	})
	t.Run("Should accept empty marketplaces and bound the complete document", func(t *testing.T) {
		t.Parallel()
		raw := []byte(`{"plugins":[]}` + strings.Repeat(" ", MaxDocumentBytes-len(`{"plugins":[]}`)))
		doc, err := DecodeDocument(raw)
		if err != nil || doc.Plugins == nil || len(doc.Plugins) != 0 {
			t.Fatalf("empty document = %+v, %v", doc, err)
		}
		if _, err := DecodeDocument(append(raw, ' ')); !errors.Is(err, ErrDocumentTooLarge) {
			t.Fatalf("oversized document = %v", err)
		}
	})
	t.Run("Should reject non-marketplaces and trailing JSON", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{"", "null", "[]", "{}", `{"plugins":null}`, `{"plugins":{}}`,
			`{"plugins":[]} {}`, `{"plugins":[`} {
			if _, err := DecodeDocument([]byte(raw)); !errors.Is(err, ErrNotMarketplace) {
				t.Fatalf("DecodeDocument(%q) = %v", raw, err)
			}
		}
	})
}
