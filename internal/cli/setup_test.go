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
				{
					Name:            "architect-advisor",
					Description:     "Council advisor",
					Origin:          setup.AssetOriginExtension,
					ExtensionName:   "idea-ext",
					ExtensionSource: "workspace",
				},
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
		"Reusable Agents",
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

func TestSetupRunYesUsesProjectScopeForReusableAgentsWhenGlobalFlagIsFalse(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.yes = true
	state.loadCatalog = func(_ context.Context, _ setup.ResolverOptions) (setup.EffectiveCatalog, error) {
		return setup.EffectiveCatalog{
			Skills: []setup.Skill{{Name: "compozy", Description: "Core workflow"}},
			ReusableAgents: []setup.ReusableAgent{
				{Name: "architect-advisor", Description: "Council advisor"},
			},
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
		return []setup.Agent{
			{
				Name:           "codex",
				DisplayName:    "Codex",
				ProjectRootDir: ".agents/skills",
				GlobalRootDir:  ".codex/skills",
				Universal:      true,
				Detected:       true,
			},
		}, nil
	}
	state.previewSkills = func(
		_ setup.ResolverOptions,
		skills []setup.Skill,
		agents []string,
		global bool,
		mode setup.InstallMode,
	) ([]setup.PreviewItem, error) {
		if global {
			t.Fatal("expected project scope for skill preview")
		}
		if mode != setup.InstallModeCopy {
			t.Fatalf("expected copy mode for single universal agent, got %q", mode)
		}
		if len(skills) != 1 || len(agents) != 1 {
			t.Fatalf("unexpected preview selection: skills=%d agents=%d", len(skills), len(agents))
		}
		return []setup.PreviewItem{
			{
				Skill:      skills[0],
				Agent:      setup.Agent{Name: "codex", DisplayName: "Codex"},
				TargetPath: ".agents/skills/compozy",
			},
		}, nil
	}

	var previewCfg setup.ReusableAgentInstallConfig
	state.previewReusableAgents = func(cfg setup.ReusableAgentInstallConfig) ([]setup.ReusableAgentPreviewItem, error) {
		previewCfg = cfg
		return []setup.ReusableAgentPreviewItem{
			{
				ReusableAgent: cfg.ReusableAgents[0],
				TargetPath:    ".compozy/agents/architect-advisor",
			},
		}, nil
	}

	state.installSkills = func(
		_ setup.ResolverOptions,
		skills []setup.Skill,
		agents []string,
		global bool,
		mode setup.InstallMode,
	) ([]setup.SuccessItem, []setup.FailureItem, error) {
		if global {
			t.Fatal("expected project scope for skill install")
		}
		return []setup.SuccessItem{
			{
				Skill: skills[0],
				Agent: setup.Agent{Name: agents[0], DisplayName: "Codex"},
				Path:  ".agents/skills/compozy",
				Mode:  mode,
			},
		}, nil, nil
	}

	var installCfg setup.ReusableAgentInstallConfig
	state.installReusableAgents = func(
		cfg setup.ReusableAgentInstallConfig,
	) ([]setup.ReusableAgentSuccessItem, []setup.ReusableAgentFailureItem, error) {
		installCfg = cfg
		return []setup.ReusableAgentSuccessItem{
			{
				ReusableAgent: cfg.ReusableAgents[0],
				Path:          ".compozy/agents/architect-advisor",
			},
		}, nil, nil
	}

	cmd := &cobra.Command{Use: "setup"}
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.Flags().Bool("global", false, "global")
	cmd.Flags().Bool("copy", false, "copy")

	if err := state.run(cmd, nil); err != nil {
		t.Fatalf("run setup: %v\noutput:\n%s", err, output.String())
	}
	if previewCfg.Global {
		t.Fatalf("expected reusable-agent preview to use project scope, got global=%t", previewCfg.Global)
	}
	if installCfg.Global {
		t.Fatalf("expected reusable-agent install to use project scope, got global=%t", installCfg.Global)
	}
	if len(installCfg.ReusableAgents) != 1 || installCfg.ReusableAgents[0].Name != "architect-advisor" {
		t.Fatalf("unexpected reusable-agent install config: %#v", installCfg)
	}
}

func TestSetupRunYesCleansLegacyTransferredAssetsBeforeInstall(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.yes = true
	state.loadCatalog = func(_ context.Context, _ setup.ResolverOptions) (setup.EffectiveCatalog, error) {
		return setup.EffectiveCatalog{
			Skills: []setup.Skill{{Name: "compozy", Description: "Core workflow"}},
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
		return []setup.Agent{
			{
				Name:           "codex",
				DisplayName:    "Codex",
				ProjectRootDir: ".agents/skills",
				GlobalRootDir:  ".codex/skills",
				Universal:      true,
				Detected:       true,
			},
		}, nil
	}
	state.previewSkills = func(
		_ setup.ResolverOptions,
		skills []setup.Skill,
		agents []string,
		_ bool,
		_ setup.InstallMode,
	) ([]setup.PreviewItem, error) {
		return []setup.PreviewItem{
			{
				Skill:      skills[0],
				Agent:      setup.Agent{Name: agents[0], DisplayName: "Codex"},
				TargetPath: ".agents/skills/compozy",
			},
		}, nil
	}
	callOrder := make([]string, 0, 3)
	state.cleanupLegacyAssets = func(cfg setup.LegacyAssetCleanupConfig) (setup.LegacyAssetCleanupResult, error) {
		if cfg.Global {
			t.Fatal("expected cleanup to run in project scope")
		}
		callOrder = append(callOrder, "cleanup")
		return setup.LegacyAssetCleanupResult{}, nil
	}
	state.installSkills = func(
		_ setup.ResolverOptions,
		skills []setup.Skill,
		agents []string,
		_ bool,
		mode setup.InstallMode,
	) ([]setup.SuccessItem, []setup.FailureItem, error) {
		if len(callOrder) != 1 || callOrder[0] != "cleanup" {
			t.Fatalf("expected cleanup before skill install, got %v", callOrder)
		}
		callOrder = append(callOrder, "skills")
		return []setup.SuccessItem{
			{
				Skill: skills[0],
				Agent: setup.Agent{Name: agents[0], DisplayName: "Codex"},
				Path:  ".agents/skills/compozy",
				Mode:  mode,
			},
		}, nil, nil
	}
	state.installReusableAgents = func(
		_ setup.ReusableAgentInstallConfig,
	) ([]setup.ReusableAgentSuccessItem, []setup.ReusableAgentFailureItem, error) {
		if len(callOrder) != 2 || callOrder[1] != "skills" {
			t.Fatalf("expected skill install before reusable agents, got %v", callOrder)
		}
		callOrder = append(callOrder, "agents")
		return nil, nil, nil
	}

	cmd := &cobra.Command{Use: "setup"}
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.Flags().Bool("global", false, "global")
	cmd.Flags().Bool("copy", false, "copy")

	if err := state.run(cmd, nil); err != nil {
		t.Fatalf("run setup: %v\noutput:\n%s", err, output.String())
	}
	if got, want := strings.Join(callOrder, ","), "cleanup,skills,agents"; got != want {
		t.Fatalf("unexpected setup install order\nwant: %s\ngot:  %s", want, got)
	}
}
