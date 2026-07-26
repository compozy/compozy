package extensionpkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	looppkg "github.com/compozy/agh/internal/loop"
)

func TestLoopResourcesShouldLoadFromManifestResources(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize resources.loops and load YAML definitions", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		loopDir := filepath.Join(root, "loops", "market-loop")
		if err := os.MkdirAll(loopDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(loopDir) error = %v", err)
		}
		loopPath := filepath.Join(loopDir, looppkg.DefinitionFileName)
		if err := os.WriteFile(loopPath, []byte(extensionLoopYAML("market-loop")), 0o644); err != nil {
			t.Fatalf("os.WriteFile(loopPath) error = %v", err)
		}
		sidecarPath := filepath.Join(loopDir, "notes.yaml")
		if err := os.WriteFile(sidecarPath, []byte("not: a loop definition\n"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(sidecarPath) error = %v", err)
		}

		normalized := normalizeResourcesConfig(ResourcesConfig{Loops: []string{" loops "}})
		if got, want := len(normalized.Loops), 1; got != want {
			t.Fatalf("len(normalized.Loops) = %d, want %d", got, want)
		}
		if got, want := normalized.Loops[0], "loops"; got != want {
			t.Fatalf("normalized.Loops[0] = %q, want %q", got, want)
		}

		manager := &Manager{}
		loops, err := manager.loadLoopResources(&managedExtension{
			rootDir: root,
			info: ExtensionInfo{
				Name:   "market-ext",
				Source: SourceMarketplace,
			},
			manifest: &Manifest{
				Resources: normalized,
			},
		})
		if err != nil {
			t.Fatalf("loadLoopResources() error = %v", err)
		}
		if got, want := len(loops), 1; got != want {
			t.Fatalf("len(loadLoopResources()) = %d, want %d", got, want)
		}
		if got, want := loops[0].Name, "market-loop"; got != want {
			t.Fatalf("loops[0].Name = %q, want %q", got, want)
		}
		if got, want := loops[0].Source, looppkg.SourceMarketplace; got != want {
			t.Fatalf("loops[0].Source = %q, want %q", got, want)
		}
		if got, want := loops[0].InstalledFromExtension, "market-ext"; got != want {
			t.Fatalf("loops[0].InstalledFromExtension = %q, want %q", got, want)
		}
	})

	t.Run("Should reject duplicate loop names from one extension", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		for _, dir := range []string{"loops-a", "loops-b"} {
			loopDir := filepath.Join(root, dir, "duplicate-loop")
			if err := os.MkdirAll(loopDir, 0o755); err != nil {
				t.Fatalf("os.MkdirAll(%s) error = %v", loopDir, err)
			}
			loopPath := filepath.Join(loopDir, looppkg.DefinitionFileName)
			if err := os.WriteFile(loopPath, []byte(extensionLoopYAML("duplicate-loop")), 0o644); err != nil {
				t.Fatalf("os.WriteFile(%s) error = %v", loopPath, err)
			}
		}

		manager := &Manager{}
		_, err := manager.loadLoopResources(&managedExtension{
			rootDir: root,
			info: ExtensionInfo{
				Name:   "market-ext",
				Source: SourceMarketplace,
			},
			manifest: &Manifest{
				Resources: ResourcesConfig{Loops: []string{"loops-a", "loops-b"}},
			},
		})
		if err == nil {
			t.Fatal("loadLoopResources() error = nil, want duplicate loop error")
		}
		if !strings.Contains(err.Error(), "duplicate loop resource") {
			t.Fatalf("loadLoopResources() error = %v, want duplicate loop resource", err)
		}
	})
}

func extensionLoopYAML(name string) string {
	return `apiVersion: agh.loop/v1
kind: Loop
meta:
  name: ` + name + `
  version: 1
  description: Extension loop
  catalog:
    use_when: Test extension loop resource loading
    keywords: [extension]
    category: extension
concurrency: queue
inputs:
  target:
    type: string
    required: true
contract:
  goal: Test extension loop loading
  definition_of_done: Loop loads from manifest resources
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress:
    window: 2
    hash_fields: []
  budget:
    tokens: 0
    wall_clock_sec: 0
    on_exceeded: halt
start:
  - kind: cli
graph:
  nodes:
    - id: target_input
      class: source
      kind: input
      input_ref: target
    - id: normalize_target
      class: action
      kind: transform
      params:
        map:
          target:
            value: ok
  edges:
    - from: target_input
      to: normalize_target
`
}
