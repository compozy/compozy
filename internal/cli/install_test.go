package cli

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	aghconfig "github.com/compozy/agh/internal/config"
)

func TestInstallCommandWritesBootstrapConfigAndAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should use explicit provider and model flags for machine output", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}

		deps.runInstallWizard = func(context.Context, installWizardInput) (installWizardSelection, error) {
			t.Fatal("install wizard should not run when provider/model flags are explicit")
			return installWizardSelection{}, nil
		}

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"install",
			"--provider",
			"claude",
			"--model",
			"claude-sonnet-4-6",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(install) error = %v", err)
		}

		var decoded installRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(install) error = %v", err)
		}
		if decoded.AgentName != aghconfig.DefaultAgentName {
			t.Fatalf("decoded.AgentName = %q, want %q", decoded.AgentName, aghconfig.DefaultAgentName)
		}
		if decoded.Provider != "claude" {
			t.Fatalf("decoded.Provider = %q, want %q", decoded.Provider, "claude")
		}
		if decoded.Permissions != string(aghconfig.PermissionModeApproveAll) {
			t.Fatalf("decoded.Permissions = %q, want %q", decoded.Permissions, aghconfig.PermissionModeApproveAll)
		}

		cfg, err := aghconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig() error = %v", err)
		}
		if cfg.Defaults.Agent != aghconfig.DefaultAgentName {
			t.Fatalf("cfg.Defaults.Agent = %q, want %q", cfg.Defaults.Agent, aghconfig.DefaultAgentName)
		}
		if cfg.Defaults.Provider != "claude" {
			t.Fatalf("cfg.Defaults.Provider = %q, want %q", cfg.Defaults.Provider, "claude")
		}

		agentContents, err := os.ReadFile(decoded.AgentFile)
		if err != nil {
			t.Fatalf("ReadFile(agent) error = %v", err)
		}
		if !strings.Contains(string(agentContents), "name: "+aghconfig.DefaultAgentName) {
			t.Fatalf("agent contents = %q, want bootstrap agent name", string(agentContents))
		}
	})
}

func updateInstallWizardModel(
	t *testing.T,
	model *installWizardModel,
	msg tea.Msg,
) (*installWizardModel, tea.Cmd) {
	t.Helper()

	next, cmd := model.Update(msg)
	typed, ok := next.(*installWizardModel)
	if !ok {
		t.Fatalf("Update() model = %T, want *installWizardModel", next)
	}
	return typed, cmd
}

func TestInstallCommandMachineOutput(t *testing.T) {
	t.Parallel()

	t.Run("Should use effective defaults without opening the wizard", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		cfg, err := aghconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig() error = %v", err)
		}
		input := buildInstallWizardInput(&cfg)
		if input.SelectedProvider == "" {
			t.Fatal("install input selected provider = empty, want default provider")
		}
		wantModel := strings.TrimSpace(input.SuggestedModels[input.SelectedProvider])
		if wantModel == "" {
			t.Fatalf("SuggestedModels[%q] = empty, want default model", input.SelectedProvider)
		}

		deps.runInstallWizard = func(context.Context, installWizardInput) (installWizardSelection, error) {
			t.Fatal("install wizard should not run for machine output")
			return installWizardSelection{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "install", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(install -o json) error = %v", err)
		}

		var decoded installRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(install) error = %v", err)
		}
		if decoded.Provider != input.SelectedProvider {
			t.Fatalf("decoded.Provider = %q, want %q", decoded.Provider, input.SelectedProvider)
		}
		if decoded.Model != wantModel {
			t.Fatalf("decoded.Model = %q, want %q", decoded.Model, wantModel)
		}
	})

	t.Run("Should bootstrap provider-managed drivers without a model", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		deps.runInstallWizard = func(context.Context, installWizardInput) (installWizardSelection, error) {
			t.Fatal("install wizard should not run for explicit machine output")
			return installWizardSelection{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "install", "--provider", "blackbox", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(install blackbox) error = %v", err)
		}

		var decoded installRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(install) error = %v", err)
		}
		if decoded.Provider != "blackbox" {
			t.Fatalf("decoded.Provider = %q, want %q", decoded.Provider, "blackbox")
		}
		if decoded.Model != "" {
			t.Fatalf("decoded.Model = %q, want empty provider-managed model", decoded.Model)
		}

		cfg, err := aghconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig() error = %v", err)
		}
		if cfg.Defaults.Provider != "blackbox" {
			t.Fatalf("cfg.Defaults.Provider = %q, want blackbox", cfg.Defaults.Provider)
		}
		if got := cfg.Providers["blackbox"].Models.Default; got != "" {
			t.Fatalf("cfg.Providers[blackbox].Models.Default = %q, want empty", got)
		}
	})

	t.Run("Should canonicalize aliases before resolving suggested models", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		deps.runInstallWizard = func(context.Context, installWizardInput) (installWizardSelection, error) {
			t.Fatal("install wizard should not run for explicit machine output")
			return installWizardSelection{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "install", "--provider", "kimi", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(install kimi alias) error = %v", err)
		}

		var decoded installRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(install) error = %v", err)
		}
		if decoded.Provider != "moonshot" {
			t.Fatalf("decoded.Provider = %q, want canonical moonshot", decoded.Provider)
		}
		if decoded.Model != "kimi-k2-thinking" {
			t.Fatalf("decoded.Model = %q, want moonshot default model", decoded.Model)
		}

		cfg, err := aghconfig.LoadGlobalConfig(homePaths)
		if err != nil {
			t.Fatalf("LoadGlobalConfig() error = %v", err)
		}
		if cfg.Defaults.Provider != "moonshot" {
			t.Fatalf("cfg.Defaults.Provider = %q, want canonical moonshot", cfg.Defaults.Provider)
		}
	})

	t.Run("Should use canonical direct driver alias defaults", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultWithHome(aghconfig.HomePaths{})
		input := buildInstallWizardInput(&cfg)
		selection, err := resolveNonInteractiveInstallSelection(input, "qwen", "")
		if err != nil {
			t.Fatalf("resolveNonInteractiveInstallSelection(qwen alias) error = %v", err)
		}
		if selection.Provider != "qwen-code" {
			t.Fatalf("selection.Provider = %q, want qwen-code", selection.Provider)
		}
		if selection.Model != "qwen3.6-plus" {
			t.Fatalf("selection.Model = %q, want qwen3.6-plus", selection.Model)
		}
	})

	t.Run("Should reject missing model for pi-backed providers", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultWithHome(aghconfig.HomePaths{})
		cfg.Providers["custom-pi"] = aghconfig.ProviderConfig{
			Command:         "npx -y pi-acp@latest",
			Harness:         aghconfig.ProviderHarnessPiACP,
			RuntimeProvider: "custom",
		}
		input := buildInstallWizardInput(&cfg)
		_, err := resolveNonInteractiveInstallSelection(input, "custom-pi", "")
		if err == nil {
			t.Fatal("resolveNonInteractiveInstallSelection() error = nil, want model required error")
		}
		wantErr := `cli: install model is required for provider "custom-pi"`
		if err.Error() != wantErr {
			t.Fatalf("resolveNonInteractiveInstallSelection() error = %q, want %q", err.Error(), wantErr)
		}
	})
}

func TestInstallCommandHumanOutput(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the interactive wizard for human output", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		deps.runInstallWizard = func(_ context.Context, input installWizardInput) (installWizardSelection, error) {
			if len(input.Providers) == 0 {
				t.Fatal("install wizard input providers = empty, want built-in providers")
			}
			return installWizardSelection{
				Provider: "claude",
				Model:    "claude-sonnet-4-6",
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "install")
		if err != nil {
			t.Fatalf("executeRootCommand(install) error = %v", err)
		}
		if !strings.Contains(stdout, "Provider:") || !strings.Contains(stdout, "claude") {
			t.Fatalf("install human stdout = %q, want selected provider", stdout)
		}
	})
}

func TestBuildInstallWizardInputAndBundleFormats(t *testing.T) {
	t.Parallel()

	t.Run("Should build wizard input and bundle formats", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultWithHome(aghconfig.HomePaths{})
		cfg.Defaults.Provider = "codex"
		cfg.Providers["custom"] = aghconfig.ProviderConfig{
			Models: aghconfig.ProviderModelsConfig{
				Default: "custom-model",
			},
		}

		input := buildInstallWizardInput(&cfg)
		if len(input.Providers) == 0 {
			t.Fatal("buildInstallWizardInput() providers = empty, want builtin/custom providers")
		}
		if input.SelectedProvider != "codex" {
			t.Fatalf("SelectedProvider = %q, want %q", input.SelectedProvider, "codex")
		}
		if input.SuggestedModels["custom"] != "custom-model" {
			t.Fatalf("SuggestedModels[custom] = %q, want %q", input.SuggestedModels["custom"], "custom-model")
		}

		record := installRecord{
			AgentName:    aghconfig.DefaultAgentName,
			Provider:     "codex",
			Model:        "gpt-5.4",
			Permissions:  string(aghconfig.PermissionModeApproveAll),
			ConfigFile:   "/tmp/config.toml",
			AgentFile:    "/tmp/AGENT.md",
			CreatedAgent: true,
		}

		human, err := installBundle(record).human()
		if err != nil {
			t.Fatalf("installBundle().human() error = %v", err)
		}
		if !strings.Contains(human, "Install") || !strings.Contains(human, "created bootstrap agent file") {
			t.Fatalf("install human output = %q, want install summary", human)
		}

		toon, err := installBundle(record).toon()
		if err != nil {
			t.Fatalf("installBundle().toon() error = %v", err)
		}
		if !strings.Contains(
			toon,
			"install{agent_name,provider,model,permissions,config_file,agent_file,created_agent,managed,manager}:",
		) {
			t.Fatalf("install toon output = %q, want TOON header", toon)
		}
	})
}

func TestInstallWizardModelTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should transition through provider model and confirmation steps", func(t *testing.T) {
		t.Parallel()

		model := newInstallWizardModel(installWizardInput{
			Providers:        []string{"claude", "codex"},
			SelectedProvider: "claude",
			SuggestedModels: map[string]string{
				"claude": "claude-sonnet",
				"codex":  "gpt-5.4",
			},
			ModelRequired: map[string]bool{
				"codex": true,
			},
		})

		if model.Init() == nil {
			t.Fatal("Init() = nil, want blink command")
		}
		if !strings.Contains(model.View(), "Select the default provider") {
			t.Fatalf("provider view = %q, want provider prompt", model.View())
		}

		var cmd tea.Cmd
		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyDown})
		if cmd != nil {
			t.Fatalf("provider navigation cmd = %v, want nil", cmd)
		}
		if model.selected != 1 {
			t.Fatalf("selected = %d, want 1", model.selected)
		}

		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("provider enter cmd = nil, want blink command")
		}
		if model.step != installWizardStepModel || model.provider != "codex" {
			t.Fatalf("provider step transition = %#v, want model step for codex", model)
		}
		if model.modelInput.Value() != "gpt-5.4" {
			t.Fatalf("modelInput.Value() = %q, want %q", model.modelInput.Value(), "gpt-5.4")
		}
		if !strings.Contains(model.View(), "Selected provider: codex") {
			t.Fatalf("model view = %q, want selected provider", model.View())
		}

		model.modelInput.SetValue("")
		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Fatalf("blank model enter cmd = %v, want nil", cmd)
		}
		if model.errText != "model is required" {
			t.Fatalf("errText = %q, want model is required", model.errText)
		}

		model.modelInput.SetValue("gpt-5.4")
		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Fatalf("model enter cmd = %v, want nil", cmd)
		}
		if model.step != installWizardStepConfirm {
			t.Fatalf("step = %v, want confirm", model.step)
		}
		if !strings.Contains(model.View(), "Review the bootstrap configuration.") {
			t.Fatalf("confirm view = %q, want review prompt", model.View())
		}

		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})
		if cmd == nil {
			t.Fatal("confirm esc cmd = nil, want blink command")
		}
		if model.step != installWizardStepModel {
			t.Fatalf("step after esc = %v, want model", model.step)
		}

		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Fatalf("model enter after esc cmd = %v, want nil", cmd)
		}
		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("confirm enter cmd = nil, want quit command")
		}
		if !model.done {
			t.Fatal("done = false, want true after confirm enter")
		}
	})

	t.Run("Should allow blank model for provider-managed drivers", func(t *testing.T) {
		t.Parallel()

		model := newInstallWizardModel(installWizardInput{
			Providers:        []string{"blackbox"},
			SelectedProvider: "blackbox",
			SuggestedModels:  map[string]string{"blackbox": ""},
			ModelRequired:    map[string]bool{"blackbox": false},
		})

		var cmd tea.Cmd
		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("provider enter cmd = nil, want blink command")
		}
		if !strings.Contains(model.View(), "provider-managed default") {
			t.Fatalf("model view = %q, want provider-managed guidance", model.View())
		}

		model, cmd = updateInstallWizardModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil {
			t.Fatalf("blank optional model enter cmd = %v, want nil", cmd)
		}
		if model.errText != "" {
			t.Fatalf("errText = %q, want empty", model.errText)
		}
		if model.step != installWizardStepConfirm {
			t.Fatalf("step = %v, want confirm", model.step)
		}
		if !strings.Contains(model.View(), "Model:       -") {
			t.Fatalf("confirm view = %q, want model dash", model.View())
		}
	})
}
