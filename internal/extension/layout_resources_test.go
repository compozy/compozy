package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/windowmanager"
)

func TestLoadLayoutResources(t *testing.T) {
	t.Parallel()

	t.Run("Should decode valid layouts at global scope", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(rootDir, "layouts"), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(layouts) error = %v", err)
		}
		writeLayoutJSONFile(t, filepath.Join(rootDir, "layouts", "two-up.json"), testLayoutResource("two-up"))
		layouts, err := LoadLayoutResources(t.Context(), rootDir, []string{"layouts/two-up.json"})
		if err != nil {
			t.Fatalf("LoadLayoutResources() error = %v", err)
		}
		if len(layouts) != 1 || layouts[0].ID != "two-up" {
			t.Fatalf("LoadLayoutResources() = %#v, want two-up", layouts)
		}
		if layouts[0].Document.WorkspaceID != "" {
			t.Fatalf("Document.WorkspaceID = %q, want global scope", layouts[0].Document.WorkspaceID)
		}
	})

	t.Run("Should stop layout validation when the caller is canceled", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		path := filepath.Join(rootDir, "layouts", "two-up.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(layouts) error = %v", err)
		}
		writeLayoutJSONFile(t, path, testLayoutResource("two-up"))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := LoadLayoutResources(ctx, rootDir, []string{"layouts/two-up.json"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("LoadLayoutResources() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should name both files declaring a duplicate layout", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		layoutsDir := filepath.Join(rootDir, "layouts")
		if err := os.MkdirAll(layoutsDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(layouts) error = %v", err)
		}
		writeLayoutJSONFile(t, filepath.Join(layoutsDir, "alpha.json"), testLayoutResource("two-up"))
		writeLayoutJSONFile(t, filepath.Join(layoutsDir, "beta.json"), testLayoutResource("two-up"))

		_, err := LoadLayoutResources(t.Context(), rootDir, []string{"layouts"})
		if !errors.Is(err, ErrManifestInvalid) || !strings.Contains(err.Error(), "alpha.json") ||
			!strings.Contains(err.Error(), "beta.json") || !strings.Contains(err.Error(), "two-up") {
			t.Fatalf("LoadLayoutResources() error = %v, want both files and duplicate ID", err)
		}
	})
}

func writeLayoutJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
}

func testLayoutResource(id string) windowmanager.LayoutResource {
	return windowmanager.LayoutResource{
		Version:          windowmanager.LayoutResourceVersion,
		ID:               id,
		DisplayName:      "Two up",
		ParticipantSlots: []windowmanager.WindowID{"primary", "secondary"},
		Document: windowmanager.LayoutDocument{
			Version: windowmanager.SnapshotVersion,
			Desktops: []windowmanager.Desktop{{
				ID:       "desktop-default",
				Name:     "Desktop 1",
				Groups:   []windowmanager.LayoutGroup{},
				Floating: []windowmanager.WindowID{},
			}},
			Windows: map[windowmanager.WindowID]windowmanager.Window{},
		},
	}
}

func TestLoadLayoutResourcesRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		setup func(*testing.T, string) string
		want  string
	}{
		{
			name: "Should reject a non-JSON layout file",
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				path := filepath.Join(rootDir, "layouts", "two-up.toml")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(layouts) error = %v", err)
				}
				writeFile(t, path, "id = \"two-up\"")
				return "layouts/two-up.toml"
			},
			want: ".json file",
		},
		{
			name: "Should reject a directory masquerading as a JSON file",
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				path := filepath.Join(rootDir, "layouts", "two-up.json")
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatalf("os.MkdirAll(two-up.json) error = %v", err)
				}
				return "layouts/two-up.json"
			},
			want: "regular .json file",
		},
		{
			name: "Should preserve codec validation errors",
			setup: func(t *testing.T, rootDir string) string {
				t.Helper()
				path := filepath.Join(rootDir, "layouts", "missing-id.json")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(layouts) error = %v", err)
				}
				layout := testLayoutResource("two-up")
				layout.ID = ""
				body, err := json.Marshal(layout)
				if err != nil {
					t.Fatalf("json.Marshal(layout) error = %v", err)
				}
				writeFile(t, path, string(body))
				return "layouts/missing-id.json"
			},
			want: "id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			declared := tc.setup(t, rootDir)
			_, err := LoadLayoutResources(t.Context(), rootDir, []string{declared})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadLayoutResources() error = %v, want %q", err, tc.want)
			}
			if strings.Contains(tc.name, "codec") && !errors.Is(err, ErrManifestInvalid) {
				t.Fatalf("LoadLayoutResources() error = %v, want ErrManifestInvalid", err)
			}
		})
	}
}
