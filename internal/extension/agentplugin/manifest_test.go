package agentplugin

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadManifestFatality(t *testing.T) {
	t.Parallel()

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
		if len(pkg.Diagnostics) != 1 || pkg.Diagnostics[0].Scope != "manifest" {
			t.Fatalf("Load() diagnostics = %#v, want one manifest diagnostic", pkg.Diagnostics)
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
