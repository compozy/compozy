package packaging

import (
	"context"
	"slices"
	"testing"

	extensions "github.com/compozy/compozy/internal/core/extension"
)

// The extension is skill-only (ADR-004): its manifest must parse and validate,
// ship the skill via skills.ship, and declare no subprocess, no hooks, and no
// agents surface. LoadManifest runs the same decode+validate the runtime uses,
// so this asserts the real contract, not a re-parse.
func TestExtensionManifestIsSkillOnly(t *testing.T) {
	t.Parallel()
	manifest, err := extensions.LoadManifest(context.Background(), extensionRoot)
	if err != nil {
		t.Fatalf("load extension.toml: %v", err)
	}

	t.Run("identity is cy-capture-decisions", func(t *testing.T) {
		t.Parallel()
		if manifest.Extension.Name != "cy-capture-decisions" {
			t.Fatalf("extension.name = %q, want cy-capture-decisions", manifest.Extension.Name)
		}
	})

	t.Run("ships the skill via skills.ship", func(t *testing.T) {
		t.Parallel()
		if !slices.Contains(manifest.Security.Capabilities, extensions.CapabilitySkillsShip) {
			t.Fatalf("capabilities = %v, want to contain skills.ship", manifest.Security.Capabilities)
		}
		if !slices.Contains(manifest.Resources.Skills, "skills/*") {
			t.Fatalf("resources.skills = %v, want to contain skills/*", manifest.Resources.Skills)
		}
	})

	t.Run("declares no subprocess, hooks, or agents surface", func(t *testing.T) {
		t.Parallel()
		if manifest.Subprocess != nil {
			t.Fatalf("subprocess = %#v, want nil for a skill-only extension", manifest.Subprocess)
		}
		if len(manifest.Hooks) != 0 {
			t.Fatalf("hooks = %v, want none for a skill-only extension", manifest.Hooks)
		}
		if len(manifest.Resources.Agents) != 0 {
			t.Fatalf("resources.agents = %v, want none for a skill-only extension", manifest.Resources.Agents)
		}
		if slices.Contains(manifest.Security.Capabilities, extensions.CapabilityAgentsShip) {
			t.Fatalf(
				"capabilities = %v, want no agents.ship for a skill-only extension",
				manifest.Security.Capabilities,
			)
		}
	})
}
