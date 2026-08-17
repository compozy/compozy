package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyManifest(t *testing.T) {
	t.Parallel()

	t.Run("Should classify the exact canonical schema as supported", func(t *testing.T) {
		t.Parallel()

		root := newPackageRoot(t, "supported")
		status, declared, err := ClassifyManifest(root)
		if err != nil {
			t.Fatalf("ClassifyManifest() error = %v", err)
		}
		if status != SchemaSupported || declared != PluginSchemaID {
			t.Fatalf("ClassifyManifest() = (%v, %q), want (%v, %q)", status, declared, SchemaSupported, PluginSchemaID)
		}
	})

	t.Run("Should return an unsupported Agent Plugins version verbatim", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		declared := "https://agent-plugins.org/schemas/2.0.0/plugin.schema.json"
		writeJSONFile(t, filepath.Join(root, "plugin.json"), map[string]any{"$schema": declared, "name": "future"})
		status, got, err := ClassifyManifest(root)
		if err != nil {
			t.Fatalf("ClassifyManifest() error = %v", err)
		}
		if status != SchemaUnsupportedVersion || got != declared {
			t.Fatalf("ClassifyManifest() = (%v, %q), want (%v, %q)", status, got, SchemaUnsupportedVersion, declared)
		}
	})

	unrelated := []map[string]any{
		{"name": "missing-schema"},
		{"$schema": "https://example.com/other.json", "name": "other"},
	}
	for index, manifest := range unrelated {
		t.Run("Should classify unrelated manifest case "+string(rune('A'+index)), func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeJSONFile(t, filepath.Join(root, "plugin.json"), manifest)
			status, _, err := ClassifyManifest(root)
			if err != nil {
				t.Fatalf("ClassifyManifest() error = %v", err)
			}
			if status != SchemaUnrelated {
				t.Fatalf("ClassifyManifest() status = %v, want %v", status, SchemaUnrelated)
			}
		})
	}

	t.Run("Should treat a symlinked manifest as absent", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(t.TempDir(), "target.json")
		writeJSONFile(t, target, map[string]any{"$schema": PluginSchemaID, "name": "linked"})
		if err := os.Symlink(target, filepath.Join(root, "plugin.json")); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}
		status, _, err := ClassifyManifest(root)
		if err != nil {
			t.Fatalf("ClassifyManifest() error = %v", err)
		}
		if status != SchemaUnrelated {
			t.Fatalf("ClassifyManifest() status = %v, want %v", status, SchemaUnrelated)
		}
	})

	t.Run("Should treat a manifest directory as absent", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, "plugin.json"), 0o755); err != nil {
			t.Fatalf("os.Mkdir(plugin.json) error = %v", err)
		}
		status, _, err := ClassifyManifest(root)
		if err != nil {
			t.Fatalf("ClassifyManifest() error = %v", err)
		}
		if status != SchemaUnrelated {
			t.Fatalf("ClassifyManifest() status = %v, want %v", status, SchemaUnrelated)
		}
	})
}

func TestValidateName(t *testing.T) {
	t.Parallel()

	valid := []string{"a", "acme.tools", "my-plugin", "lint3r", strings.Repeat("a", 64)}
	for _, name := range valid {
		t.Run("Should accept "+name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateName(name); err != nil {
				t.Fatalf("ValidateName(%q) error = %v", name, err)
			}
		})
	}

	invalid := []string{"", "My-Plugin", "-start", "end.", "has--double", "too..dots", strings.Repeat("a", 65), "café"}
	for _, name := range invalid {
		t.Run(fmt.Sprintf("Should reject invalid name %q", name), func(t *testing.T) {
			t.Parallel()

			err := ValidateName(name)
			const want = "must be 1-64 lowercase letters, digits, dots, or hyphens"
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("ValidateName(%q) error = %v, want %q", name, err, want)
			}
		})
	}
}
