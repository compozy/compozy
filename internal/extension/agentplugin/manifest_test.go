package agentplugin

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadManifestFatality(t *testing.T) {
	t.Parallel()
	t.Run("Should load the same manifest bytes that were classified", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "captured")
		document, err := ReadManifest(root)
		if err != nil {
			t.Fatal(err)
		}
		status, _ := document.Classify()
		if status != SchemaSupported {
			t.Fatalf("manifest classification = %v", status)
		}
		writeJSONFile(t, filepath.Join(root, "plugin.json"), map[string]any{"$schema": "unsupported", "name": "replacement"})
		name, err := document.Name()
		if err != nil || name != "captured" {
			t.Fatalf("manifest name = %q, %v", name, err)
		}
		pkg, err := document.Load(LoadOptions{})
		if err != nil || pkg.Name != "captured" {
			t.Fatalf("captured manifest package = %#v, %v", pkg, err)
		}
		if _, err := Load(root, LoadOptions{}); err == nil {
			t.Fatal("fresh load accepted the replacement manifest")
		}
	})

	tests := []struct {
		name      string
		manifest  map[string]any
		wantField string
	}{
		{name: "Should reject a missing name", manifest: map[string]any{"$schema": PluginSchemaID}, wantField: "name"},
		{
			name:      "Should reject a numeric homepage",
			manifest:  map[string]any{"$schema": PluginSchemaID, "name": "bad-home", "homepage": 42},
			wantField: "homepage",
		},
		{
			name:      "Should reject a non-string keyword",
			manifest:  map[string]any{"$schema": PluginSchemaID, "name": "bad-keyword", "keywords": []any{"ok", 42}},
			wantField: "keywords",
		},
		{
			name:      "Should reject a null keyword",
			manifest:  map[string]any{"$schema": PluginSchemaID, "name": "null-keyword", "keywords": []any{"ok", nil}},
			wantField: "keywords",
		},
		{
			name:      "Should reject an explicit null version",
			manifest:  map[string]any{"$schema": PluginSchemaID, "name": "null-version", "version": nil},
			wantField: "version",
		},
		{
			name: "Should reject an unknown author field",
			manifest: map[string]any{
				"$schema": PluginSchemaID,
				"name":    "bad-author",
				"author":  map[string]any{"handle": "x"},
			},
			wantField: "author.handle",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeJSONFile(t, filepath.Join(root, "plugin.json"), test.manifest)
			_, err := Load(root, LoadOptions{})
			var manifestErr *ManifestError
			if !errors.As(err, &manifestErr) {
				t.Fatalf("Load() error = %v, want *ManifestError", err)
			}
			if len(manifestErr.Issues) != 1 || manifestErr.Issues[0].Path != test.wantField {
				t.Fatalf("Load() issues = %#v, want one issue for %q", manifestErr.Issues, test.wantField)
			}
		})
	}

	t.Run("Should allow an absent optional version", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "no-version")
		pkg := loadForTest(t, root)
		if pkg.Version != "" {
			t.Fatalf("Load() Version = %q, want empty", pkg.Version)
		}
	})
}

func TestLoadManifestTolerance(t *testing.T) {
	t.Parallel()

	t.Run("Should report and ignore unknown top-level fields", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeJSONFile(t, filepath.Join(root, "plugin.json"), map[string]any{
			"$schema": PluginSchemaID,
			"name":    "unknown-field",
			"icon":    "icon.svg",
		})
		pkg := loadForTest(t, root)
		want := []Diagnostic{{Scope: "manifest", Message: "ignored unknown top-level field: icon"}}
		if !reflect.DeepEqual(pkg.Diagnostics, want) {
			t.Fatalf("Load() diagnostics = %#v, want %#v", pkg.Diagnostics, want)
		}
	})

	t.Run("Should drop a non-object extensions field", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeJSONFile(t, filepath.Join(root, "plugin.json"), map[string]any{
			"$schema":    PluginSchemaID,
			"name":       "bad-extensions",
			"extensions": 42,
		})
		pkg := loadForTest(t, root)
		want := Diagnostic{Scope: "manifest", Message: "ignored non-object extensions field"}
		if len(pkg.Diagnostics) != 1 || pkg.Diagnostics[0] != want {
			t.Fatalf("Load() diagnostics = %#v, want %#v", pkg.Diagnostics, []Diagnostic{want})
		}
	})

	t.Run("Should ignore arbitrary extension namespace contents", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeJSONFile(t, filepath.Join(root, "plugin.json"), map[string]any{
			"$schema":    PluginSchemaID,
			"name":       "opaque-extensions",
			"extensions": map[string]any{"com.example.x": map[string]any{"garbage": []any{true, 42}}},
		})
		pkg := loadForTest(t, root)
		if len(pkg.Diagnostics) != 0 {
			t.Fatalf("Load() diagnostics = %#v, want none", pkg.Diagnostics)
		}
	})

	t.Run("Should retain typed author metadata", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeJSONFile(t, filepath.Join(root, "plugin.json"), map[string]any{
			"$schema": PluginSchemaID,
			"name":    "typed-author",
			"author":  map[string]any{"name": "Ada", "email": "ada@example.com", "url": "not-validated"},
		})
		pkg := loadForTest(t, root)
		want := &AuthorInfo{Name: "Ada", Email: "ada@example.com", URL: "not-validated"}
		if !reflect.DeepEqual(pkg.Author, want) {
			t.Fatalf("Load() Author = %#v, want %#v", pkg.Author, want)
		}
	})
}

func TestLoadEmptyAndDeterministic(t *testing.T) {
	t.Parallel()

	t.Run("Should load a manifest-only package", func(t *testing.T) {
		t.Parallel()

		pkg := loadForTest(t, newPackageRoot(t, "empty-package"))
		if len(pkg.Skills) != 0 || len(pkg.Servers) != 0 || len(pkg.Diagnostics) != 0 {
			t.Fatalf("Load() package = %#v, want no components or diagnostics", pkg)
		}
	})

	t.Run("Should return deep-equal results for identical canonical inputs", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "repeatable")
		writeFile(t, filepath.Join(root, "skills", "review", "SKILL.md"), validSkill("review", "Review code."))
		dataDir := filepath.Join(t.TempDir(), "data")
		first, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("first Load() error = %v", err)
		}
		second, err := Load(root, LoadOptions{DataDir: dataDir})
		if err != nil {
			t.Fatalf("second Load() error = %v", err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("Load() results differ:\nfirst=%#v\nsecond=%#v", first, second)
		}
	})
}

func TestClientManifestAdapter(t *testing.T) {
	t.Run("Should load declared skills and both MCP transports and report ignored components", func(t *testing.T) {
		t.Parallel()
		root := canonicalPathForTest(t, t.TempDir())
		writeJSONFile(t, filepath.Join(root, ".cursor-plugin", "plugin.json"), map[string]any{
			"name": "client", "version": "1.0.0", "skills": []string{"./extra/review", "./extra/review"},
			"commands": "./commands", "agents": []string{"./agents"}, "hooks": map[string]any{},
			"mcpServers": map[string]any{
				"local": map[string]any{"command": "node", "args": []string{"${PLUGIN_ROOT}/server.js"}},
				"remote": map[string]any{
					"type":    "http",
					"url":     "https://example.com/mcp",
					"headers": map[string]string{"X-Tenant": "client"},
				},
			},
		})
		writeFile(
			t,
			filepath.Join(root, "extra", "review", "SKILL.md"),
			[]byte("---\nname: review\ndescription: Review code\n---\nReview carefully.\n"),
		)
		pkg := loadForTest(t, root)
		if pkg.Layout != "cursor-plugin" || len(pkg.Skills) != 1 || pkg.Skills[0].Name != "review" ||
			len(pkg.Servers) != 2 {
			t.Fatalf("package = %#v", pkg)
		}
		if pkg.Servers[0].Transport != transportStdio || pkg.Servers[0].Args[0] != filepath.Join(root, "server.js") ||
			pkg.Servers[1].Transport != transportStreamableHTTP {
			t.Fatalf("servers = %#v", pkg.Servers)
		}
		if len(pkg.Diagnostics) != 3 {
			t.Fatalf("diagnostics = %#v", pkg.Diagnostics)
		}
		for _, diagnostic := range pkg.Diagnostics {
			if diagnostic.Code != ClientComponentIgnored {
				t.Fatalf("diagnostic = %#v", diagnostic)
			}
		}
	})
	t.Run("Should reject components that escape the package root", func(t *testing.T) {
		t.Parallel()
		parent := canonicalPathForTest(t, t.TempDir())
		root := filepath.Join(parent, "package")
		writeJSONFile(
			t,
			filepath.Join(root, ".claude-plugin", "plugin.json"),
			map[string]any{"name": "client", "skills": "../outside", "mcpServers": "../mcp.json"},
		)
		writeFile(
			t,
			filepath.Join(parent, "outside", "SKILL.md"),
			[]byte("---\nname: outside\ndescription: Must not load\n---\nOutside\n"),
		)
		writeJSONFile(
			t,
			filepath.Join(parent, "mcp.json"),
			map[string]any{"mcpServers": map[string]any{"outside": map[string]any{"command": "node"}}},
		)
		pkg := loadForTest(t, root)
		if len(pkg.Skills) != 0 || len(pkg.Servers) != 0 || len(pkg.Diagnostics) != 2 {
			t.Fatalf("escaped resources = %#v", pkg)
		}
	})
}
