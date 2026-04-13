package setup

import (
	"path/filepath"
	"testing"
)

func TestBuildEffectiveCatalogPrefersCoreAndHigherPrecedenceExtensions(t *testing.T) {
	t.Parallel()

	catalog := BuildEffectiveCatalog(
		[]Skill{
			{Name: "compozy", Origin: AssetOriginBundled},
		},
		[]Skill{
			{
				Name:            "compozy",
				Origin:          AssetOriginExtension,
				ExtensionName:   "shadow-core",
				ExtensionSource: "workspace",
			},
			{
				Name:            "idea-pack",
				Origin:          AssetOriginExtension,
				ExtensionName:   "user-idea",
				ExtensionSource: "user",
				ManifestPath:    "/tmp/user.json",
				ResolvedPath:    "/tmp/user/idea-pack",
			},
			{
				Name:            "idea-pack",
				Origin:          AssetOriginExtension,
				ExtensionName:   "workspace-idea",
				ExtensionSource: "workspace",
				ManifestPath:    "/tmp/workspace.json",
				ResolvedPath:    "/tmp/workspace/idea-pack",
			},
		},
		nil,
		[]ReusableAgent{
			{
				Name:            "architect-advisor",
				Origin:          AssetOriginExtension,
				ExtensionName:   "shadow-agent",
				ExtensionSource: "workspace",
			},
			{
				Name:            "product-scout",
				Origin:          AssetOriginExtension,
				ExtensionName:   "workspace-idea",
				ExtensionSource: "workspace",
			},
		},
	)

	if got := skillNames(catalog.Skills); len(got) != 2 || got[0] != "compozy" || got[1] != "idea-pack" {
		t.Fatalf("unexpected effective skill names: %#v", got)
	}
	if catalog.Skills[1].ExtensionName != "workspace-idea" {
		t.Fatalf("expected workspace extension to win idea-pack, got %q", catalog.Skills[1].ExtensionName)
	}
	if got := reusableAgentNames(
		catalog.ReusableAgents,
	); len(got) != 2 || got[0] != "architect-advisor" ||
		got[1] != "product-scout" {
		t.Fatalf("unexpected effective reusable-agent names: %#v", got)
	}
	if len(catalog.Conflicts) != 2 {
		t.Fatalf("len(Conflicts) = %d, want 2", len(catalog.Conflicts))
	}
}

func reusableAgentNames(reusableAgents []ReusableAgent) []string {
	names := make([]string, 0, len(reusableAgents))
	for i := range reusableAgents {
		names = append(names, reusableAgents[i].Name)
	}
	return names
}

func TestInstallSelectedSkillsCopiesBundledAndExtensionSkills(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"compozy/SKILL.md": "---\nname: compozy\ndescription: Core workflow\n---\n",
	})
	bundledSkills, err := ListSkills(bundle)
	if err != nil {
		t.Fatalf("list bundled skills: %v", err)
	}

	extensionDir := writeExtensionSkillPack(t, t.TempDir(), "idea-pack", map[string]string{
		"references/notes.md": "# Notes\n",
	})
	extensionSkills, err := ListExtensionSkills([]SkillPackSource{
		{
			ExtensionName:   "idea-ext",
			ExtensionSource: "workspace",
			ManifestPath:    filepath.Join(t.TempDir(), "extension.json"),
			ResolvedPath:    extensionDir,
		},
	})
	if err != nil {
		t.Fatalf("list extension skills: %v", err)
	}

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	successes, failures, err := InstallSelectedSkills(
		ResolverOptions{CWD: projectDir, HomeDir: homeDir},
		append(bundledSkills, extensionSkills...),
		[]string{"codex"},
		false,
		InstallModeCopy,
	)
	if err != nil {
		t.Fatalf("install selected skills: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %#v", failures)
	}
	if len(successes) != 2 {
		t.Fatalf("expected 2 successes, got %#v", successes)
	}

	assertFileExists(t, filepath.Join(projectDir, ".agents", "skills", "compozy", "SKILL.md"))
	assertFileExists(t, filepath.Join(projectDir, ".agents", "skills", "idea-pack", "SKILL.md"))
	assertFileExists(t, filepath.Join(projectDir, ".agents", "skills", "idea-pack", "references", "notes.md"))
}
