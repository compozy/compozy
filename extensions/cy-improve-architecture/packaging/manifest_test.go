// Suite: extension manifest packaging
// Invariant: the extension is a skill-only pack containing exactly the three declared skills.
// Boundary IN: the real extension.toml and skill directories loaded through exported APIs.
// Boundary OUT: generic manifest validation, owned by internal/core/extension/manifest_test.go.
package packaging

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	extensions "github.com/compozy/compozy/internal/core/extension"
)

var expectedSkillNames = []string{
	"cy-codebase-design",
	"cy-domain-modeling",
	"cy-improve-architecture",
}

func TestManifestDeclaresSkillOnlyExtension(t *testing.T) {
	t.Parallel()

	manifest, err := extensions.LoadManifest(context.Background(), extensionRoot(t))
	if err != nil {
		t.Fatalf("load extension manifest: %v", err)
	}

	if manifest.Extension.Name != "cy-improve-architecture" {
		t.Fatalf("unexpected extension name: %q", manifest.Extension.Name)
	}
	if manifest.Extension.MinCompozyVersion != "0.1.10" {
		t.Fatalf("unexpected minimum Compozy version: %q", manifest.Extension.MinCompozyVersion)
	}
	if !slices.Contains(manifest.Security.Capabilities, extensions.CapabilitySkillsShip) {
		t.Fatalf("missing %q capability: %#v", extensions.CapabilitySkillsShip, manifest.Security.Capabilities)
	}
	if slices.Contains(manifest.Security.Capabilities, extensions.CapabilityAgentsShip) {
		t.Fatalf("unexpected %q capability: %#v", extensions.CapabilityAgentsShip, manifest.Security.Capabilities)
	}
	if !slices.Contains(manifest.Resources.Skills, "skills/*") {
		t.Fatalf("missing skill resource glob: %#v", manifest.Resources.Skills)
	}
	if manifest.Subprocess != nil {
		t.Fatalf("unexpected subprocess declaration: %#v", manifest.Subprocess)
	}
	if len(manifest.Hooks) != 0 {
		t.Fatalf("unexpected hook declarations: %#v", manifest.Hooks)
	}
	if len(manifest.Resources.Agents) != 0 {
		t.Fatalf("unexpected agent resources: %#v", manifest.Resources.Agents)
	}
}

func TestManifestShipsExactlyExpectedSkillDirectories(t *testing.T) {
	t.Parallel()

	skillsDir := filepath.Join(extensionRoot(t), "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("read skills directory: %v", err)
	}

	actualSkillNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			actualSkillNames = append(actualSkillNames, entry.Name())
		}
	}
	if !slices.Equal(actualSkillNames, expectedSkillNames) {
		t.Fatalf("unexpected skill directories: got %#v want %#v", actualSkillNames, expectedSkillNames)
	}

	for _, skillName := range expectedSkillNames {
		skillName := skillName
		t.Run(skillName, func(t *testing.T) {
			t.Parallel()

			skillPath := filepath.Join(skillsDir, skillName, "SKILL.md")
			info, statErr := os.Stat(skillPath)
			if statErr != nil {
				t.Fatalf("stat %q: %v", skillPath, statErr)
			}
			if info.IsDir() {
				t.Fatalf("expected %q to be a file", skillPath)
			}
		})
	}
}

func extensionRoot(t *testing.T) string {
	t.Helper()

	_, sourcePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve packaging test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourcePath), ".."))
}
