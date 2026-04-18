package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/workspace"
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

func TestPreferredRuntimeDefaultsUseReadmeExamples(t *testing.T) {
	t.Parallel()

	defaults := workspace.DefaultsConfig{}
	if got := preferredRuntimeIDE(defaults, nil); got != "codex" {
		t.Fatalf("unexpected ide default: %q", got)
	}
	if got := preferredRuntimeModel(defaults, model.IDECodex); got != "gpt-5.4" {
		t.Fatalf("unexpected model default: %q", got)
	}
	if got := preferredRuntimeReasoningEffort(defaults); got != "medium" {
		t.Fatalf("unexpected reasoning effort default: %q", got)
	}
}

func TestPreferredRuntimeIDEUsesSelectedAgentsWhenConfigUnset(t *testing.T) {
	t.Parallel()

	defaults := workspace.DefaultsConfig{}
	if got := preferredRuntimeIDE(defaults, []string{"claude-code"}); got != model.IDEClaude {
		t.Fatalf("unexpected claude ide default: %q", got)
	}
	if got := preferredRuntimeIDE(defaults, []string{"cursor"}); got != model.IDECursor {
		t.Fatalf("unexpected cursor ide default: %q", got)
	}
}

func TestPreferredRuntimeModelFollowsConfiguredIDE(t *testing.T) {
	t.Parallel()

	defaults := workspace.DefaultsConfig{IDE: stringPtr(model.IDECursor)}
	if got := preferredRuntimeModel(defaults, model.IDECursor); got != model.DefaultCursorModel {
		t.Fatalf("unexpected cursor model default: %q", got)
	}
}

func TestSetupResolverOptionsUseDiscoveredWorkspaceRoot(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.resolveWorkspace = func(context.Context) (workspace.Context, error) {
		return workspace.Context{Root: "/tmp/workspace-root"}, nil
	}

	resolver, err := state.resolverOptions(context.Background())
	if err != nil {
		t.Fatalf("resolver options: %v", err)
	}
	if resolver.CWD != "/tmp/workspace-root" {
		t.Fatalf("unexpected resolver cwd: %q", resolver.CWD)
	}
}

func TestDefaultModelForIDEResolvesRegistryDefault(t *testing.T) {
	t.Parallel()

	if got := defaultModelForIDE(model.IDECursor); got != model.DefaultCursorModel {
		t.Fatalf("unexpected cursor default model: %q", got)
	}
}

func TestPersistRuntimeDefaultsPreservesOtherConfigSections(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, ".compozy", "config.toml")
	provider := "coderabbit"
	accessMode := "full"
	if err := workspace.WriteConfig(context.Background(), configPath, workspace.ProjectConfig{
		Defaults:     workspace.DefaultsConfig{AccessMode: &accessMode},
		FetchReviews: workspace.FetchReviewsConfig{Provider: &provider},
	}); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	state := newSetupCommandState()
	plan := runtimeDefaultsPlan{
		Enabled:    true,
		ConfigPath: configPath,
		Defaults: workspace.DefaultsConfig{
			IDE:             stringPtr("codex"),
			Model:           stringPtr("gpt-5.4"),
			ReasoningEffort: stringPtr("medium"),
		},
	}

	if err := state.persistRuntimeDefaults(context.Background(), plan); err != nil {
		t.Fatalf("persist runtime defaults: %v", err)
	}

	loaded, exists, err := workspace.LoadConfigFile(context.Background(), configPath)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if !exists {
		t.Fatal("expected config file to exist")
	}
	if loaded.Defaults.IDE == nil || *loaded.Defaults.IDE != "codex" {
		t.Fatalf("unexpected defaults.ide: %#v", loaded.Defaults.IDE)
	}
	if loaded.Defaults.Model == nil || *loaded.Defaults.Model != "gpt-5.4" {
		t.Fatalf("unexpected defaults.model: %#v", loaded.Defaults.Model)
	}
	if loaded.Defaults.ReasoningEffort == nil || *loaded.Defaults.ReasoningEffort != "medium" {
		t.Fatalf("unexpected defaults.reasoning_effort: %#v", loaded.Defaults.ReasoningEffort)
	}
	if loaded.Defaults.AccessMode == nil || *loaded.Defaults.AccessMode != "full" {
		t.Fatalf("unexpected defaults.access_mode: %#v", loaded.Defaults.AccessMode)
	}
	if loaded.FetchReviews.Provider == nil || *loaded.FetchReviews.Provider != "coderabbit" {
		t.Fatalf("unexpected fetch_reviews.provider: %#v", loaded.FetchReviews.Provider)
	}
}

func TestResolveRuntimeDefaultsPlanUsesProvidedContextForConfigLoad(t *testing.T) {
	t.Parallel()

	type contextKey string
	const key contextKey = "source"
	loadErr := context.Canceled

	state := newSetupCommandState()
	state.resolveWorkspace = func(ctx context.Context) (workspace.Context, error) {
		if got := ctx.Value(key); got != "cmd-context" {
			t.Fatalf("expected command context to reach workspace resolution, got %#v", got)
		}
		configPath := filepath.Join(t.TempDir(), ".compozy", "config.toml")
		return workspace.Context{Root: filepath.Dir(filepath.Dir(configPath)), ConfigPath: configPath}, nil
	}
	state.promptRuntimeDefaults = func(string) (bool, error) {
		return true, nil
	}
	state.loadConfigFile = func(ctx context.Context, _ string) (workspace.ProjectConfig, bool, error) {
		if got := ctx.Value(key); got != "cmd-context" {
			t.Fatalf("expected command context to reach config load, got %#v", got)
		}
		return workspace.ProjectConfig{}, false, loadErr
	}

	ctx := context.WithValue(context.Background(), key, "cmd-context")
	_, err := state.resolveRuntimeDefaultsPlan(ctx, false, []string{"codex"})
	if err == nil {
		t.Fatal("expected runtime defaults plan to return load error")
	}
	if !strings.Contains(err.Error(), loadErr.Error()) {
		t.Fatalf("expected load error to be wrapped, got %v", err)
	}
}

func TestPersistRuntimeDefaultsUsesProvidedContextForLoadAndWrite(t *testing.T) {
	t.Parallel()

	type contextKey string
	const key contextKey = "source"

	state := newSetupCommandState()
	state.loadConfigFile = func(ctx context.Context, _ string) (workspace.ProjectConfig, bool, error) {
		if got := ctx.Value(key); got != "cmd-context" {
			t.Fatalf("expected command context to reach config load, got %#v", got)
		}
		return workspace.ProjectConfig{}, true, nil
	}
	state.writeConfig = func(ctx context.Context, _ string, cfg workspace.ProjectConfig) error {
		if got := ctx.Value(key); got != "cmd-context" {
			t.Fatalf("expected command context to reach config write, got %#v", got)
		}
		if cfg.Defaults.IDE == nil || *cfg.Defaults.IDE != "codex" {
			t.Fatalf("unexpected defaults.ide: %#v", cfg.Defaults.IDE)
		}
		return nil
	}

	ctx := context.WithValue(context.Background(), key, "cmd-context")
	err := state.persistRuntimeDefaults(ctx, runtimeDefaultsPlan{
		Enabled:    true,
		ConfigPath: filepath.Join(t.TempDir(), ".compozy", "config.toml"),
		Defaults: workspace.DefaultsConfig{
			IDE:             stringPtr("codex"),
			Model:           stringPtr("gpt-5.4"),
			ReasoningEffort: stringPtr("medium"),
		},
	})
	if err != nil {
		t.Fatalf("persist runtime defaults: %v", err)
	}
}

func TestResolveRuntimeDefaultsPlanSkipsWorkspacePromptForGlobalInstall(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.loadConfigFile = func(context.Context, string) (workspace.ProjectConfig, bool, error) {
		t.Fatal("expected global setup to skip workspace config loading")
		return workspace.ProjectConfig{}, false, nil
	}

	plan, err := state.resolveRuntimeDefaultsPlan(context.Background(), true, []string{"codex"})
	if err != nil {
		t.Fatalf("resolve runtime defaults plan: %v", err)
	}
	if plan.Enabled {
		t.Fatal("expected global setup to skip runtime defaults plan")
	}
}

func TestResolveRuntimeDefaultsPlanSkipsConfigLoadWhenPromptDeclined(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.promptRuntimeDefaults = func(string) (bool, error) {
		return false, nil
	}
	state.loadConfigFile = func(context.Context, string) (workspace.ProjectConfig, bool, error) {
		t.Fatal("expected declining runtime defaults prompt to skip workspace config loading")
		return workspace.ProjectConfig{}, false, nil
	}

	plan, err := state.resolveRuntimeDefaultsPlan(context.Background(), false, []string{"codex"})
	if err != nil {
		t.Fatalf("resolve runtime defaults plan: %v", err)
	}
	if plan.Enabled {
		t.Fatal("expected declined runtime defaults prompt to skip runtime defaults plan")
	}
}

func TestResolveRuntimeDefaultsPlanDoesNotParseConfigBeforeOptIn(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	configPath := filepath.Join(workspaceRoot, ".compozy", "config.toml")
	if err := workspace.WriteConfig(context.Background(), configPath, workspace.ProjectConfig{}); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("[defaults]\nide = ["), 0o600); err != nil {
		t.Fatalf("write malformed config: %v", err)
	}

	state := newSetupCommandState()
	state.resolveWorkspace = func(context.Context) (workspace.Context, error) {
		return workspace.Context{Root: workspaceRoot, ConfigPath: configPath}, nil
	}
	state.promptRuntimeDefaults = func(string) (bool, error) {
		return false, nil
	}
	state.loadConfigFile = func(context.Context, string) (workspace.ProjectConfig, bool, error) {
		t.Fatal("expected config parsing to be skipped before opt-in")
		return workspace.ProjectConfig{}, false, nil
	}

	plan, err := state.resolveRuntimeDefaultsPlan(context.Background(), false, []string{"claude-code"})
	if err != nil {
		t.Fatalf("resolve runtime defaults plan: %v", err)
	}
	if plan.Enabled {
		t.Fatal("expected declined runtime defaults prompt to skip runtime defaults plan")
	}
}

func TestRuntimeDefaultsExistingTracksManagedFields(t *testing.T) {
	t.Parallel()

	blank := "   "
	modelName := "gpt-5.4"
	accessMode := "full"

	tests := []struct {
		name   string
		exists bool
		config workspace.ProjectConfig
		want   bool
	}{
		{
			name:   "missing file",
			exists: false,
			config: workspace.ProjectConfig{
				Defaults: workspace.DefaultsConfig{Model: &modelName},
			},
			want: false,
		},
		{
			name:   "blank managed field",
			exists: true,
			config: workspace.ProjectConfig{
				Defaults: workspace.DefaultsConfig{IDE: &blank},
			},
			want: false,
		},
		{
			name:   "unmanaged defaults field ignored",
			exists: true,
			config: workspace.ProjectConfig{
				Defaults: workspace.DefaultsConfig{AccessMode: &accessMode},
			},
			want: false,
		},
		{
			name:   "managed defaults field present",
			exists: true,
			config: workspace.ProjectConfig{
				Defaults: workspace.DefaultsConfig{Model: &modelName},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hasExistingDefaults :=
				(tc.config.Defaults.IDE != nil && strings.TrimSpace(*tc.config.Defaults.IDE) != "") ||
					(tc.config.Defaults.Model != nil && strings.TrimSpace(*tc.config.Defaults.Model) != "") ||
					(tc.config.Defaults.ReasoningEffort != nil && strings.TrimSpace(*tc.config.Defaults.ReasoningEffort) != "")
			if got := tc.exists && hasExistingDefaults; got != tc.want {
				t.Fatalf("existing runtime defaults = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExecuteInstallDoesNotPersistDefaultsWhenInstallFails(t *testing.T) {
	t.Parallel()

	state := newSetupCommandState()
	state.installSkills = func(
		_ setup.ResolverOptions,
		_ []setup.Skill,
		_ []string,
		_ bool,
		_ setup.InstallMode,
	) ([]setup.SuccessItem, []setup.FailureItem, error) {
		return nil, []setup.FailureItem{{Error: "boom"}}, nil
	}
	state.installReusableAgents = func(
		_ setup.ReusableAgentInstallConfig,
	) ([]setup.ReusableAgentSuccessItem, []setup.ReusableAgentFailureItem, error) {
		return nil, nil, nil
	}
	state.writeConfig = func(context.Context, string, workspace.ProjectConfig) error {
		t.Fatal("expected failed setup to skip config persistence")
		return nil
	}

	cmd := &cobra.Command{Use: "setup"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := state.executeInstall(context.Background(), cmd, setupInstallPlan{
		Config: setup.InstallConfig{},
		RuntimePlan: runtimeDefaultsPlan{
			Enabled:    true,
			ConfigPath: filepath.Join(t.TempDir(), ".compozy", "config.toml"),
			Defaults: workspace.DefaultsConfig{
				IDE:             stringPtr("codex"),
				Model:           stringPtr("gpt-5.4"),
				ReasoningEffort: stringPtr("medium"),
			},
		},
	})
	if err == nil {
		t.Fatal("expected setup failure")
	}
	if !strings.Contains(err.Error(), "1 failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}
