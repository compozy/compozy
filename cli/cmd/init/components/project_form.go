package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/engine/runtime"
)

const (
	modeMemory      = "memory"
	modePersistent  = "persistent"
	modeDistributed = "distributed"
)

var modeDisplayLabels = map[string]string{
	modeMemory:      "🚀 Memory",
	modePersistent:  "💾 Persistent",
	modeDistributed: "🏭 Distributed",
}

var modeHelpTexts = map[string]string{
	modeMemory: strings.Join([]string{
		"Memory Mode (🚀):",
		"- Zero dependencies, instant startup",
		"- Perfect for tests and quick prototyping",
		"- No persistence (data lost on restart)",
	}, "\n"),
	modePersistent: strings.Join([]string{
		"Persistent Mode (💾):",
		"- File-based storage, state preserved",
		"- Ideal for local development",
		"- Still zero external dependencies",
	}, "\n"),
	modeDistributed: strings.Join([]string{
		"Distributed Mode (🏭):",
		"- External PostgreSQL, Redis, Temporal",
		"- Production-ready, horizontal scaling",
		"- Requires Docker or managed services",
	}, "\n"),
}

// ProjectFormData holds the project initialization data
type ProjectFormData struct {
	Name          string
	Description   string
	Version       string
	Author        string
	AuthorURL     string
	Template      string
	Mode          string
	IncludeDocker bool
	InstallBun    bool // Whether to install Bun if not available
}

// GetMode returns the selected project mode.
func (d *ProjectFormData) GetMode() string {
	return d.Mode
}

// SetMode updates the selected project mode.
func (d *ProjectFormData) SetMode(mode string) {
	d.Mode = mode
}

// NewProjectForm creates the project initialization form
func NewProjectForm(data *ProjectFormData) *huh.Form {
	setDefaults(data)
	fields := createBaseFields(data)
	fields = addConditionalFields(fields, data)
	return huh.NewForm(huh.NewGroup(fields...))
}

// setDefaults sets default values for form data
func setDefaults(data *ProjectFormData) {
	if data.Version == "" {
		data.Version = "0.1.0"
	}
	if data.Template == "" {
		data.Template = "basic"
	}
	if !isValidMode(data.Mode) {
		data.Mode = modeMemory
	}
	if data.Mode != modeDistributed {
		data.IncludeDocker = false
	}
}

// createBaseFields creates the basic form fields
func createBaseFields(data *ProjectFormData) []huh.Field {
	return []huh.Field{
		huh.NewInput().
			Title("Project Name").
			Description("The name of your Compozy project").
			Value(&data.Name).
			Validate(validateProjectName),
		huh.NewInput().
			Title("Description").
			Description("A brief description of your project").
			Value(&data.Description).
			Validate(validateDescription),
		huh.NewInput().
			Title("Version").
			Description("Semantic version of your project").
			Value(&data.Version).
			Validate(validateVersion),
		huh.NewInput().
			Title("Author").
			Description("Project author name").
			Value(&data.Author),
		huh.NewInput().
			Title("Author URL").
			Description("Author's website or profile URL").
			Value(&data.AuthorURL),
		huh.NewSelect[string]().
			Title("Template").
			Description("Project template to use").
			Options(huh.NewOption("Basic", "basic")).
			Value(&data.Template),
		createModeField(data),
	}
}

func createModeField(data *ProjectFormData) huh.Field {
	selectField := huh.NewSelect[string]().
		Title("Mode").
		Description(modeHelpText(data.Mode)).
		Options(
			huh.NewOption(modeDisplayLabels[modeMemory], modeMemory),
			huh.NewOption(modeDisplayLabels[modePersistent], modePersistent),
			huh.NewOption(modeDisplayLabels[modeDistributed], modeDistributed),
		).
		Value(&data.Mode).
		Validate(validateMode)
	selectField.DescriptionFunc(func() string {
		return modeHelpText(data.Mode)
	}, data)
	return selectField
}

// addConditionalFields adds conditional fields based on system state
func addConditionalFields(fields []huh.Field, data *ProjectFormData) []huh.Field {
	if !runtime.IsBunAvailable() {
		fields = append(fields, createBunInstallField(data))
	}
	fields = append(fields, createDockerField(data))
	return fields
}

// createBunInstallField creates the Bun installation field
func createBunInstallField(data *ProjectFormData) huh.Field {
	return huh.NewConfirm().
		Title("Install Bun runtime?").
		Description("Bun is required to run Compozy projects but was not found in your PATH.\n" +
			"Would you like to install it automatically? (curl -fsSL https://bun.sh/install | bash)").
		WithButtonAlignment(lipgloss.Left).
		Value(&data.InstallBun).
		Affirmative("Yes, install Bun").
		Negative("No, I'll install it manually")
}

// createDockerField creates the Docker configuration field
func createDockerField(data *ProjectFormData) huh.Field {
	confirm := huh.NewConfirm().
		Title("Include Docker configuration?").
		WithButtonAlignment(lipgloss.Left).
		Value(&data.IncludeDocker).
		Affirmative("Yes").
		Negative("No")
	themeState := huh.ThemeCharm()
	enabledTheme := cloneTheme(themeState)
	disabledTheme := deriveDisabledConfirmTheme(themeState)
	confirm.WithTheme(themeState)
	applyDockerToggleState(confirm, data, themeState, enabledTheme, disabledTheme)
	confirm.Description(dockerHelpText(data.Mode))
	confirm.DescriptionFunc(func() string {
		applyDockerToggleState(confirm, data, themeState, enabledTheme, disabledTheme)
		return dockerHelpText(data.Mode)
	}, data)
	return confirm
}

func applyDockerToggleState(
	confirm *huh.Confirm,
	data *ProjectFormData,
	themeState, enabledTheme, disabledTheme *huh.Theme,
) {
	disabled := data.Mode != modeDistributed
	if disabled {
		data.IncludeDocker = false
		*themeState = *disabledTheme
		confirm.WithKeyMap(disabledConfirmKeyMap())
		return
	}
	*themeState = *enabledTheme
	confirm.WithKeyMap(huh.NewDefaultKeyMap())
}

func deriveDisabledConfirmTheme(enabled *huh.Theme) *huh.Theme {
	disabled := cloneTheme(enabled)
	muted := lipgloss.Color("240")
	disabled.Focused.Title = disabled.Focused.Title.Foreground(muted)
	disabled.Focused.Description = disabled.Focused.Description.Foreground(muted)
	disabled.Focused.FocusedButton = disabled.Focused.FocusedButton.Foreground(muted).Background(lipgloss.Color("236"))
	disabled.Focused.BlurredButton = disabled.Focused.BlurredButton.Foreground(muted).Background(lipgloss.Color("236"))
	disabled.Blurred = disabled.Focused
	return disabled
}

func cloneTheme(theme *huh.Theme) *huh.Theme {
	clone := *theme
	return &clone
}

func disabledConfirmKeyMap() *huh.KeyMap {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Confirm.Toggle.SetEnabled(false)
	keyMap.Confirm.Accept.SetEnabled(false)
	keyMap.Confirm.Reject.SetEnabled(false)
	return keyMap
}

func modeHelpText(mode string) string {
	if help, ok := modeHelpTexts[mode]; ok {
		return help
	}
	return modeHelpTexts[modeMemory]
}

func dockerHelpText(mode string) string {
	if mode == modeDistributed {
		return "Generate docker-compose.yaml for external services"
	}
	return "Docker not needed for embedded mode"
}

func isValidMode(mode string) bool {
	switch mode {
	case modeMemory, modePersistent, modeDistributed:
		return true
	default:
		return false
	}
}

func validateMode(mode string) error {
	if !isValidMode(mode) {
		return fmt.Errorf("invalid mode selection")
	}
	return nil
}

// Validation functions
func validateProjectName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("project name is required")
	}
	if len(s) > 50 {
		return fmt.Errorf("project name must be 50 characters or less")
	}
	return nil
}

func validateDescription(s string) error {
	if len(s) > 200 {
		return fmt.Errorf("description must be 200 characters or less")
	}
	return nil
}

func validateVersion(s string) error {
	if len(s) > 20 {
		return fmt.Errorf("version must be 20 characters or less")
	}
	return nil
}
