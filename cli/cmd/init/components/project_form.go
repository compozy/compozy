package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/engine/runtime"
)

// ProjectFormData holds the project initialization data
type ProjectFormData struct {
	Name          string
	Description   string
	Version       string
	Author        string
	AuthorURL     string
	Template      string
	IncludeDocker bool
	InstallBun    bool // Whether to install Bun if not available
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
	}
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
	return huh.NewConfirm().
		Title("Include Docker configuration?").
		Description("This will create a docker-compose.yaml with Redis, Postgres\n" +
			"and Temporal including, and a .env.example file.").
		WithButtonAlignment(lipgloss.Left).
		Value(&data.IncludeDocker).
		Affirmative("Yes").
		Negative("No")
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
