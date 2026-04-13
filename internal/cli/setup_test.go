package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

func TestSetupHelpShowsSetupFlagsOnly(t *testing.T) {
	t.Parallel()

	output, err := executeRootCommand("setup", "--help")
	if err != nil {
		t.Fatalf("execute setup help: %v", err)
	}

	required := []string{"--agent", "--skill", "--global", "--copy", "--list", "--yes", "--all"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected setup help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--provider", "--pr", "--tasks-dir", "--batch-size", "--concurrent"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected setup help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestSetupRunYesFailsWithoutDetectedAgents(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.loadCatalog = func(_ context.Context, _ setup.ResolverOptions) (setup.EffectiveCatalog, error) {
		return setup.EffectiveCatalog{
			Skills: []setup.Skill{{Name: "cy-create-prd", Description: "Create a PRD"}},
		}, nil
	}
	state.listAgents = func(setup.ResolverOptions) ([]setup.Agent, error) {
		return []setup.Agent{
			{
				Name:           "codex",
				DisplayName:    "Codex",
				ProjectRootDir: ".agents/skills",
				GlobalRootDir:  ".codex/skills",
				Universal:      true,
			},
		}, nil
	}
	state.detectAgents = func(setup.ResolverOptions) ([]setup.Agent, error) {
		return nil, nil
	}
	state.yes = true

	cmd := &cobra.Command{Use: "setup"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().Bool("global", false, "global")
	cmd.Flags().Bool("copy", false, "copy")

	err := state.run(cmd, nil)
	if err == nil {
		t.Fatal("expected setup run to fail when no agents are detected")
	}
	if !strings.Contains(err.Error(), "no agents detected") {
		t.Fatalf("expected missing detected agents error, got %v", err)
	}
}

func TestSetupListIncludesExtensionSourcesAndConflictWarnings(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.loadCatalog = func(_ context.Context, _ setup.ResolverOptions) (setup.EffectiveCatalog, error) {
		return setup.EffectiveCatalog{
			Skills: []setup.Skill{
				{Name: "compozy", Description: "Core workflow", Origin: setup.AssetOriginBundled},
				{
					Name:            "idea-pack",
					Description:     "Extension workflow",
					Origin:          setup.AssetOriginExtension,
					ExtensionName:   "idea-ext",
					ExtensionSource: "workspace",
				},
			},
			ReusableAgents: []setup.ReusableAgent{
				{Name: "architect-advisor", Description: "Core council", Origin: setup.AssetOriginBundled},
				{
					Name:            "product-scout",
					Description:     "Extension reusable agent",
					Origin:          setup.AssetOriginExtension,
					ExtensionName:   "idea-ext",
					ExtensionSource: "workspace",
				},
			},
			Conflicts: []setup.CatalogConflict{
				{
					Kind:       setup.CatalogAssetKindSkill,
					Name:       "compozy",
					Resolution: setup.CatalogConflictCoreWins,
					Winner:     setup.AssetRef{Origin: setup.AssetOriginBundled, Name: "compozy"},
					Ignored: setup.AssetRef{
						Origin:          setup.AssetOriginExtension,
						Name:            "compozy",
						ExtensionName:   "shadow-ext",
						ExtensionSource: "workspace",
					},
				},
			},
		}, nil
	}
	state.list = true

	cmd := &cobra.Command{Use: "setup"}
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	if err := state.run(cmd, nil); err != nil {
		t.Fatalf("run setup list: %v\noutput:\n%s", err, output.String())
	}

	required := []string{
		"Setup Skills",
		"[core]",
		"[workspace:idea-ext]",
		"Global Reusable Agents",
		"architect-advisor",
		"product-scout",
		"Warnings",
		`ignored extension skill "compozy" from workspace:shadow-ext because the core skill wins`,
	}
	for _, snippet := range required {
		if !strings.Contains(output.String(), snippet) {
			t.Fatalf("expected setup --list output to include %q\noutput:\n%s", snippet, output.String())
		}
	}
}
