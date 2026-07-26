package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/spf13/cobra"
)

const (
	installCommandKey = "install"
	installModelKey   = "model"
)

const (
	installAgentValue    = "Agent"
	installManagedValue  = "Managed"
	installManagerValue  = "Manager"
	installModelValue    = "Model"
	installProviderValue = "Provider"
	installAgentNameKey  = "agent_name"
	installManagedKey    = "managed"
)

var errInstallCanceled = errors.New("cli: install canceled")

type installWizardInput struct {
	Providers        []string
	SelectedProvider string
	SuggestedModels  map[string]string
	ModelRequired    map[string]bool
}

type installWizardSelection struct {
	Provider string
	Model    string
}

type installRecord struct {
	AgentName    string `json:"agent_name"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Permissions  string `json:"permissions"`
	ConfigFile   string `json:"config_file"`
	AgentFile    string `json:"agent_file"`
	CreatedAgent bool   `json:"created_agent"`
	Managed      bool   `json:"managed"`
	Manager      string `json:"manager,omitempty"`
}

type installWizardStep int

const (
	installWizardStepProvider installWizardStep = iota
	installWizardStepModel
	installWizardStepConfirm

	defaultInstallProvider = "claude"
	installWizardEnterKey  = "enter"
)

type installWizardModel struct {
	input      installWizardInput
	step       installWizardStep
	selected   int
	provider   string
	modelInput textinput.Model
	errText    string
	done       bool
	canceled   bool
}

func newInstallCommand(deps commandDeps) *cobra.Command {
	var provider string
	var model string

	cmd := &cobra.Command{
		Use:   installCommandKey,
		Short: "Bootstrap AGH and create the default general agent",
		Example: `  # Create ~/.agh/config.toml and ~/.agh/agents/general/AGENT.md
  agh install

  # Bootstrap non-interactively for automation
  agh install --provider codex --model gpt-5.6-sol -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			homePaths, err := deps.resolveHome()
			if err != nil {
				return err
			}
			if err := deps.ensureHome(homePaths); err != nil {
				return err
			}

			cfg, err := aghconfig.LoadGlobalConfig(homePaths)
			if err != nil {
				return err
			}

			selection, err := resolveInstallSelection(cmd, &cfg, provider, model, deps.runInstallWizard)
			if err != nil {
				return err
			}

			cfg, err = aghconfig.SaveBootstrapConfig(homePaths, selection.Provider, selection.Model)
			if err != nil {
				return err
			}
			agentPath, createdAgent, err := aghconfig.EnsureBootstrapAgent(homePaths)
			if err != nil {
				return err
			}
			record := installRecord{
				AgentName:    aghconfig.DefaultAgentName,
				Provider:     cfg.Defaults.Provider,
				Model:        cfg.Providers[cfg.Defaults.Provider].Models.Default,
				Permissions:  string(cfg.Permissions.Mode),
				ConfigFile:   homePaths.ConfigFile,
				AgentFile:    agentPath,
				CreatedAgent: createdAgent,
				Managed:      detectManagedState(deps).Managed,
				Manager:      detectManagedState(deps).Manager,
			}
			return writeCommandOutput(cmd, installBundle(record))
		},
	}
	cmd.Flags().StringVar(&provider, cliProviderKey, "", "Default provider to configure without opening the wizard")
	cmd.Flags().StringVar(&model, installModelKey, "", "Default model to configure without opening the wizard")
	return cmd
}

func resolveInstallSelection(
	cmd *cobra.Command,
	cfg *aghconfig.Config,
	provider string,
	model string,
	runWizard installWizardRunner,
) (installWizardSelection, error) {
	if cmd == nil {
		return installWizardSelection{}, errors.New("cli: command is required")
	}
	if runWizard == nil {
		return installWizardSelection{}, errors.New("cli: install wizard runner is required")
	}

	input := buildInstallWizardInput(cfg)
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return installWizardSelection{}, err
	}
	providerChanged := cmd.Flags().Changed(cliProviderKey)
	modelChanged := cmd.Flags().Changed(installModelKey)
	if providerChanged || modelChanged || mode != OutputHuman {
		return resolveNonInteractiveInstallSelection(input, provider, model)
	}
	return runWizard(cmd.Context(), input)
}

func resolveNonInteractiveInstallSelection(
	input installWizardInput,
	provider string,
	model string,
) (installWizardSelection, error) {
	selectedProvider := aghconfig.CanonicalProviderName(provider)
	if selectedProvider == "" {
		selectedProvider = aghconfig.CanonicalProviderName(input.SelectedProvider)
	}
	if selectedProvider == "" {
		return installWizardSelection{}, errors.New("cli: install provider is required")
	}

	selectedModel := strings.TrimSpace(model)
	if selectedModel == "" {
		selectedModel = strings.TrimSpace(input.SuggestedModels[selectedProvider])
	}
	if selectedModel == "" && input.ModelRequired[selectedProvider] {
		return installWizardSelection{}, fmt.Errorf(
			"cli: install model is required for provider %q",
			selectedProvider,
		)
	}

	return installWizardSelection{Provider: selectedProvider, Model: selectedModel}, nil
}

func buildInstallWizardInput(cfg *aghconfig.Config) installWizardInput {
	seen := make(map[string]struct{})
	providers := make([]string, 0, len(cfg.Providers)+8)
	for name := range aghconfig.BuiltinProviders() {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		providers = append(providers, name)
	}
	for name := range cfg.Providers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		providers = append(providers, name)
	}
	sort.Strings(providers)

	suggestedModels := make(map[string]string, len(providers))
	modelRequired := make(map[string]bool, len(providers))
	for _, provider := range providers {
		resolved, err := cfg.ResolveProvider(provider)
		if err == nil {
			suggestedModels[provider] = strings.TrimSpace(resolved.Models.Default)
			modelRequired[provider] = installProviderRequiresModel(resolved)
			continue
		}
		configured := cfg.Providers[provider]
		suggestedModels[provider] = strings.TrimSpace(configured.Models.Default)
		modelRequired[provider] = installProviderRequiresModel(configured)
	}

	selectedProvider := aghconfig.CanonicalProviderName(cfg.Defaults.Provider)
	if selectedProvider == "" {
		if _, ok := seen[defaultInstallProvider]; ok {
			selectedProvider = defaultInstallProvider
		}
	}
	if selectedProvider == "" && len(providers) > 0 {
		selectedProvider = providers[0]
	}

	return installWizardInput{
		Providers:        providers,
		SelectedProvider: selectedProvider,
		SuggestedModels:  suggestedModels,
		ModelRequired:    modelRequired,
	}
}

func installProviderRequiresModel(provider aghconfig.ProviderConfig) bool {
	return provider.RequiresRuntimeModel()
}

func installBundle(record installRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			status := "reused existing agent file"
			if record.CreatedAgent {
				status = "created bootstrap agent file"
			}
			return renderHumanSection("Install", []keyValue{
				{Label: installAgentValue, Value: stringOrDash(record.AgentName)},
				{Label: installProviderValue, Value: stringOrDash(record.Provider)},
				{Label: installModelValue, Value: stringOrDash(record.Model)},
				{Label: installPermissionsValue, Value: stringOrDash(record.Permissions)},
				{Label: "Config File", Value: stringOrDash(record.ConfigFile)},
				{Label: "Agent File", Value: stringOrDash(record.AgentFile)},
				{Label: "Agent Status", Value: status},
				{Label: installManagedValue, Value: fmt.Sprintf("%t", record.Managed)},
				{Label: installManagerValue, Value: stringOrDash(record.Manager)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(installCommandKey, []string{
				installAgentNameKey,
				cliProviderKey,
				installModelKey,
				configPermissionsKey,
				"config_file",
				"agent_file",
				"created_agent",
				installManagedKey,
				"manager",
			}, []string{
				record.AgentName,
				record.Provider,
				record.Model,
				record.Permissions,
				record.ConfigFile,
				record.AgentFile,
				fmt.Sprintf("%t", record.CreatedAgent),
				fmt.Sprintf("%t", record.Managed),
				record.Manager,
			}), nil
		},
	}
}

func runInstallWizard(_ context.Context, input installWizardInput) (installWizardSelection, error) {
	program := tea.NewProgram(newInstallWizardModel(input))
	finalModel, err := program.Run()
	if err != nil {
		return installWizardSelection{}, fmt.Errorf("cli: run install wizard: %w", err)
	}

	model, ok := finalModel.(*installWizardModel)
	if !ok {
		return installWizardSelection{}, errors.New("cli: install wizard returned unexpected model")
	}
	if model.canceled {
		return installWizardSelection{}, errInstallCanceled
	}
	if !model.done {
		return installWizardSelection{}, errors.New("cli: install wizard did not complete")
	}

	return installWizardSelection{
		Provider: strings.TrimSpace(model.provider),
		Model:    strings.TrimSpace(model.modelInput.Value()),
	}, nil
}

func newInstallWizardModel(input installWizardInput) *installWizardModel {
	modelInput := textinput.New()
	modelInput.Prompt = "model> "
	modelInput.Placeholder = "default model"
	modelInput.Focus()

	selected := 0
	for i, provider := range input.Providers {
		if provider == input.SelectedProvider {
			selected = i
			break
		}
	}

	return &installWizardModel{
		input:      input,
		step:       installWizardStepProvider,
		selected:   selected,
		modelInput: modelInput,
	}
}

func (m *installWizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *installWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if ok {
		if keyMsg.String() == "ctrl+c" {
			m.canceled = true
			return m, tea.Quit
		}

		switch m.step {
		case installWizardStepProvider:
			return m.updateProviderStep(keyMsg)
		case installWizardStepModel:
			return m.updateModelStep(keyMsg)
		case installWizardStepConfirm:
			return m.updateConfirmStep(keyMsg)
		}
	}

	if m.step != installWizardStepModel {
		return m, nil
	}

	var cmd tea.Cmd
	m.modelInput, cmd = m.modelInput.Update(msg)
	return m, cmd
}

func (m *installWizardModel) View() string {
	var builder strings.Builder

	builder.WriteString("AGH Install\n")
	builder.WriteString("===========\n\n")

	switch m.step {
	case installWizardStepProvider:
		builder.WriteString("Select the default provider for the bootstrap `general` agent.\n\n")
		for i, provider := range m.input.Providers {
			cursor := "  "
			if i == m.selected {
				cursor = "> "
			}
			builder.WriteString(cursor + provider + "\n")
		}
		builder.WriteString("\nUse up/down or j/k, press Enter to continue, Ctrl+C to cancel.\n")
	case installWizardStepModel:
		builder.WriteString("Selected provider: " + m.provider + "\n")
		if m.modelRequired() {
			builder.WriteString("Enter the default model for this provider.\n\n")
		} else {
			builder.WriteString("Enter a default model, or leave blank for the provider-managed default.\n\n")
		}
		builder.WriteString(m.modelInput.View() + "\n")
		builder.WriteString("\nPress Enter to continue, Esc to go back, Ctrl+C to cancel.\n")
	case installWizardStepConfirm:
		builder.WriteString("Review the bootstrap configuration.\n\n")
		builder.WriteString("Agent:       " + aghconfig.DefaultAgentName + "\n")
		builder.WriteString("Provider:    " + m.provider + "\n")
		builder.WriteString("Model:       " + stringOrDash(strings.TrimSpace(m.modelInput.Value())) + "\n")
		builder.WriteString("Permissions: " + string(aghconfig.PermissionModeApproveAll) + "\n")
		builder.WriteString("\nPress Enter to write ~/.agh/config.toml and ensure ~/.agh/agents/general/AGENT.md.\n")
		builder.WriteString("Press Esc to edit the model, or Ctrl+C to cancel.\n")
	}

	if strings.TrimSpace(m.errText) != "" {
		builder.WriteString("\nError: " + m.errText + "\n")
	}

	return builder.String()
}

func (m *installWizardModel) updateProviderStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		if m.selected < len(m.input.Providers)-1 {
			m.selected++
		}
	case installWizardEnterKey:
		if len(m.input.Providers) == 0 {
			m.errText = "no providers available"
			return m, nil
		}
		m.provider = m.input.Providers[m.selected]
		m.modelInput.SetValue(m.input.SuggestedModels[m.provider])
		m.modelInput.Focus()
		m.errText = ""
		m.step = installWizardStepModel
		return m, textinput.Blink
	}
	return m, nil
}

func (m *installWizardModel) updateModelStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.errText = ""
		m.step = installWizardStepProvider
		return m, nil
	case installWizardEnterKey:
		if strings.TrimSpace(m.modelInput.Value()) == "" && m.modelRequired() {
			m.errText = "model is required"
			return m, nil
		}
		m.errText = ""
		m.step = installWizardStepConfirm
		return m, nil
	}

	var cmd tea.Cmd
	m.modelInput, cmd = m.modelInput.Update(msg)
	return m, cmd
}

func (m *installWizardModel) modelRequired() bool {
	return m.input.ModelRequired[strings.TrimSpace(m.provider)]
}

func (m *installWizardModel) updateConfirmStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.errText = ""
		m.step = installWizardStepModel
		m.modelInput.Focus()
		return m, textinput.Blink
	case installWizardEnterKey:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}
