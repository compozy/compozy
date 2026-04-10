package extensions

import (
	"path/filepath"
	"testing"
)

func TestExtractDeclaredProvidersGroupsByCategory(t *testing.T) {
	entry := DiscoveredExtension{
		Ref: Ref{
			Name:   "asset-ext",
			Source: SourceWorkspace,
		},
		ManifestPath: "/tmp/workspace/.compozy/extensions/asset-ext/extension.json",
		Manifest: &Manifest{
			Providers: ProvidersConfig{
				IDE: []ProviderEntry{
					{Name: "asset-ide", Command: "bin/asset-ide"},
				},
				Review: []ProviderEntry{
					{Name: "asset-review", Command: "bin/asset-review"},
				},
				Model: []ProviderEntry{
					{Name: "asset-model", Command: "bin/asset-model"},
				},
			},
		},
	}

	providers := ExtractDeclaredProviders([]DiscoveredExtension{entry})
	if len(providers.IDE) != 1 {
		t.Fatalf("len(IDE) = %d, want 1", len(providers.IDE))
	}
	if len(providers.Review) != 1 {
		t.Fatalf("len(Review) = %d, want 1", len(providers.Review))
	}
	if len(providers.Model) != 1 {
		t.Fatalf("len(Model) = %d, want 1", len(providers.Model))
	}
	if got := providers.IDE[0].Name; got != "asset-ide" {
		t.Fatalf("IDE[0].Name = %q, want %q", got, "asset-ide")
	}
	if got := providers.Review[0].Name; got != "asset-review" {
		t.Fatalf("Review[0].Name = %q, want %q", got, "asset-review")
	}
	if got := providers.Model[0].Name; got != "asset-model" {
		t.Fatalf("Model[0].Name = %q, want %q", got, "asset-model")
	}
}

func TestExtractDeclaredSkillPacksResolvesAbsolutePaths(t *testing.T) {
	extensionDir := t.TempDir()
	writeSkillPack(t, extensionDir, "skills", "alpha")
	writeSkillPack(t, extensionDir, "skills", "beta")

	entry := DiscoveredExtension{
		Ref: Ref{
			Name:   "skill-ext",
			Source: SourceUser,
		},
		ManifestPath: filepath.Join(extensionDir, ManifestFileNameJSON),
		Manifest: &Manifest{
			Resources: ResourcesConfig{
				Skills: []string{"skills/*"},
			},
		},
		diskRoot: extensionDir,
	}

	packs := ExtractDeclaredSkillPacks([]DiscoveredExtension{entry})
	if len(packs.Packs) != 2 {
		t.Fatalf("len(Packs) = %d, want 2", len(packs.Packs))
	}
	for _, pack := range packs.Packs {
		if !filepath.IsAbs(pack.ResolvedPath) {
			t.Fatalf("ResolvedPath = %q, want absolute path", pack.ResolvedPath)
		}
	}
	if filepath.Base(packs.Packs[0].ResolvedPath) != "alpha" {
		t.Fatalf("Packs[0].ResolvedPath = %q, want alpha first", packs.Packs[0].ResolvedPath)
	}
	if filepath.Base(packs.Packs[1].ResolvedPath) != "beta" {
		t.Fatalf("Packs[1].ResolvedPath = %q, want beta second", packs.Packs[1].ResolvedPath)
	}
}
