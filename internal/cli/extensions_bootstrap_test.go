package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/modelprovider"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

type bootstrapFetchProvider struct {
	name string
}

func (p bootstrapFetchProvider) Name() string { return p.name }

func (bootstrapFetchProvider) FetchReviews(context.Context, provider.FetchRequest) ([]provider.ReviewItem, error) {
	return nil, nil
}

func (bootstrapFetchProvider) ResolveIssues(context.Context, string, []provider.ResolvedIssue) error {
	return nil
}

func TestPrepareAndRunInstallsEnabledExtensionSkillPackDuringPreflight(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	chdirCLITest(t, workspaceRoot)

	manifest := bootstrapManifestFixture("skills-ext")
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilitySkillsShip}
	manifest.Resources.Skills = []string{"skills/*"}
	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "skills-ext")
	writeBootstrapManifestJSON(t, extensionDir, manifest)
	writeBootstrapTestFile(
		t,
		filepath.Join(extensionDir, "skills", "ext-pack", "SKILL.md"),
		"---\nname: ext-pack\ndescription: Extension skill\n---\n",
	)
	enableBootstrapWorkspaceExtension(t, homeDir, workspaceRoot, "skills-ext")

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.ide = string(core.IDECodex)
	state.skipValidation = true
	state.isInteractive = func() bool { return true }
	state.listBundledSkills = func() ([]setup.Skill, error) { return nil, nil }
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Mode:  setup.InstallModeCopy,
		}, nil
	}
	state.installBundledSkills = func(cfg setup.InstallConfig) (*setup.Result, error) {
		if len(cfg.SkillNames) != 0 {
			t.Fatalf("expected no bundled skill install, got %#v", cfg.SkillNames)
		}
		return &setup.Result{}, nil
	}
	state.confirmSkillRefresh = func(_ *cobra.Command, prompt skillRefreshPrompt) (bool, error) {
		if !strings.Contains(strings.Join(prompt.DriftedSkills, ","), "ext-pack") {
			t.Fatalf("expected extension skill in refresh prompt, got %#v", prompt.DriftedSkills)
		}
		return true, nil
	}

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	if err := state.prepareAndRun(cmd, nil, false); err != nil {
		t.Fatalf("prepareAndRun: %v", err)
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner after extension skill install")
	}

	assertBootstrapFileExists(t, filepath.Join(workspaceRoot, ".agents", "skills", "ext-pack", "SKILL.md"))
	if !strings.Contains(output.String(), "Updated required Compozy skills for Codex (project scope).") {
		t.Fatalf("expected refresh output, got %q", output.String())
	}
}

func TestPrepareAndRunIgnoresDisabledExtensionSkillPack(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	chdirCLITest(t, workspaceRoot)

	manifest := bootstrapManifestFixture("skills-ext")
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilitySkillsShip}
	manifest.Resources.Skills = []string{"skills/*"}
	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "skills-ext")
	writeBootstrapManifestJSON(t, extensionDir, manifest)
	writeBootstrapTestFile(
		t,
		filepath.Join(extensionDir, "skills", "ext-pack", "SKILL.md"),
		"---\nname: ext-pack\ndescription: Extension skill\n---\n",
	)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.ide = string(core.IDECodex)
	state.skipValidation = true
	state.isInteractive = func() bool { return true }
	state.listBundledSkills = func() ([]setup.Skill, error) { return nil, nil }
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Mode:  setup.InstallModeCopy,
		}, nil
	}
	state.installBundledSkills = func(setup.InstallConfig) (*setup.Result, error) {
		return &setup.Result{}, nil
	}
	state.confirmSkillRefresh = func(*cobra.Command, skillRefreshPrompt) (bool, error) {
		t.Fatal("did not expect refresh prompt for disabled extension")
		return false, nil
	}
	state.installExtensionSkills = func(setup.ExtensionInstallConfig) (*setup.ExtensionResult, error) {
		t.Fatal("did not expect extension skill install for disabled extension")
		return nil, nil
	}

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"

	if err := state.prepareAndRun(cmd, nil, false); err != nil {
		t.Fatalf("prepareAndRun: %v", err)
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner when disabled extension is ignored")
	}
	if _, err := os.Stat(
		filepath.Join(workspaceRoot, ".agents", "skills", "ext-pack", "SKILL.md"),
	); !os.IsNotExist(
		err,
	) {
		t.Fatalf("expected disabled extension skill to remain absent, got err=%v", err)
	}
}

func TestFetchReviewsUsesEnabledExtensionProviderOverlay(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	chdirCLITest(t, workspaceRoot)

	manifest := bootstrapManifestFixture("review-ext")
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilityProvidersRegister}
	manifest.Providers.Review = []extensions.ProviderEntry{{Name: "ext-review", Command: "stub"}}
	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "review-ext")
	writeBootstrapManifestJSON(t, extensionDir, manifest)
	enableBootstrapWorkspaceExtension(t, homeDir, workspaceRoot, "review-ext")

	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	state.name = "demo"
	state.provider = "ext-review"
	state.pr = "259"
	state.fetchReviewsFn = func(_ context.Context, cfg core.Config) (*core.FetchResult, error) {
		base := provider.NewRegistry()
		base.Register(bootstrapFetchProvider{name: "stub"})

		resolved, err := provider.ResolveRegistry(base).Get(cfg.Provider)
		if err != nil {
			return nil, err
		}
		return &core.FetchResult{
			Name:       cfg.Name,
			Provider:   resolved.Name(),
			PR:         cfg.PR,
			Round:      1,
			ReviewsDir: filepath.Join(workspaceRoot, ".compozy", "tasks", cfg.Name, "reviews-001"),
			Total:      1,
		}, nil
	}

	cmd := &cobra.Command{Use: "fetch-reviews"}
	cmd.Flags().String("provider", "", "")
	_ = cmd.Flags().Set("provider", "ext-review")
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	if err := state.fetchReviews(cmd, nil); err != nil {
		t.Fatalf("fetchReviews: %v", err)
	}
	if !strings.Contains(output.String(), "Fetched 1 review issues from ext-review") {
		t.Fatalf("expected overlay provider fetch summary, got %q", output.String())
	}
}

func TestPrepareAndRunAcceptsEnabledExtensionACPRuntimeOverlay(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	chdirCLITest(t, workspaceRoot)

	manifest := bootstrapManifestFixture("runtime-ext")
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilityProvidersRegister}
	manifest.Providers.IDE = []extensions.ProviderEntry{{
		Name:    "ext-adapter",
		Command: "mock-acp --serve",
		Metadata: map[string]string{
			"agent_name":    "codex",
			"default_model": "mock-model",
			"display_name":  "Mock ACP",
		},
	}}
	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "runtime-ext")
	writeBootstrapManifestJSON(t, extensionDir, manifest)
	enableBootstrapWorkspaceExtension(t, homeDir, workspaceRoot, "runtime-ext")

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.ide = "ext-adapter"
	state.skipValidation = true
	state.listBundledSkills = func() ([]setup.Skill, error) { return nil, nil }
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Mode:  setup.InstallModeCopy,
		}, nil
	}
	state.installBundledSkills = func(setup.InstallConfig) (*setup.Result, error) {
		return &setup.Result{}, nil
	}

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"

	if err := state.prepareAndRun(cmd, nil, false); err != nil {
		t.Fatalf("prepareAndRun: %v", err)
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner with overlay IDE")
	}
}

func TestBootstrapDeclarativeAssetsActivatesModelAliasOverlay(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	chdirCLITest(t, workspaceRoot)

	manifest := bootstrapManifestFixture("model-ext")
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilityProvidersRegister}
	manifest.Providers.Model = []extensions.ProviderEntry{{
		Name:        "ext-model",
		Target:      "openai/gpt-5.4",
		DisplayName: "Extension Model",
	}}
	manifest.Providers.IDE = []extensions.ProviderEntry{{
		Name:         "ext-adapter",
		Command:      "mock-acp --serve",
		DefaultModel: "ext-model",
		DisplayName:  "Mock ACP",
	}}
	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "model-ext")
	writeBootstrapManifestJSON(t, extensionDir, manifest)
	enableBootstrapWorkspaceExtension(t, homeDir, workspaceRoot, "model-ext")

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	assets, cleanup, err := state.bootstrapDeclarativeAssetsForWorkspaceRoot(
		context.Background(),
		workspaceRoot,
		"compozy start",
	)
	if err != nil {
		t.Fatalf("bootstrapDeclarativeAssetsForWorkspaceRoot() error = %v", err)
	}
	defer cleanup()

	if len(assets.Discovery.Providers.Model) != 1 || assets.Discovery.Providers.Model[0].Name != "ext-model" {
		t.Fatalf("assets.Discovery.Providers.Model = %#v, want ext-model entry", assets.Discovery.Providers.Model)
	}
	if got := modelprovider.ResolveAlias("ext-model"); got != "openai/gpt-5.4" {
		t.Fatalf("ResolveAlias(ext-model) = %q, want %q", got, "openai/gpt-5.4")
	}
	if got, err := agent.ResolveRuntimeModel("ext-adapter", ""); err != nil || got != "openai/gpt-5.4" {
		t.Fatalf("ResolveRuntimeModel(ext-adapter) = %q err=%v, want %q", got, err, "openai/gpt-5.4")
	}
}

func bootstrapManifestFixture(name string) *extensions.Manifest {
	return &extensions.Manifest{
		Extension: extensions.ExtensionInfo{
			Name:              name,
			Version:           "1.0.0",
			Description:       "Fixture " + name,
			MinCompozyVersion: "0.0.1",
		},
		Security: extensions.SecurityConfig{},
	}
}

func writeBootstrapManifestJSON(t *testing.T, dir string, manifest *extensions.Manifest) {
	t.Helper()

	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeBootstrapTestFile(t, filepath.Join(dir, extensions.ManifestFileNameJSON), string(payload))
}

func writeBootstrapTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func assertBootstrapFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %q to exist: %v", path, err)
	}
}

func enableBootstrapWorkspaceExtension(t *testing.T, homeDir string, workspaceRoot string, name string) {
	t.Helper()

	store, err := extensions.NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("create enablement store: %v", err)
	}
	if err := store.Enable(context.Background(), extensions.Ref{
		Name:          name,
		Source:        extensions.SourceWorkspace,
		WorkspaceRoot: workspaceRoot,
	}); err != nil {
		t.Fatalf("enable workspace extension %q: %v", name, err)
	}
}
