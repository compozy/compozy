package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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
}

// NewProjectForm creates the project initialization form
func NewProjectForm(data *ProjectFormData) *huh.Form {
	// Set defaults if not provided
	if data.Version == "" {
		data.Version = "0.1.0"
	}
	if data.Template == "" {
		data.Template = "basic"
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("The name of your Compozy project").
				Value(&data.Name).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("project name is required")
					}
					if len(s) > 50 {
						return fmt.Errorf("project name must be 50 characters or less")
					}
					return nil
				}),

			huh.NewInput().
				Title("Description").
				Description("A brief description of your project").
				Value(&data.Description).
				Validate(func(s string) error {
					if len(s) > 200 {
						return fmt.Errorf("description must be 200 characters or less")
					}
					return nil
				}),

			huh.NewInput().
				Title("Version").
				Description("Semantic version of your project").
				Value(&data.Version).
				Validate(func(s string) error {
					if len(s) > 20 {
						return fmt.Errorf("version must be 20 characters or less")
					}
					return nil
				}),

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
				Options(
					huh.NewOption("Basic", "basic"),
				).
				Value(&data.Template),

			huh.NewConfirm().
				Title("Include Docker configuration?").
				Description("This will create a docker-compose.yaml with Redis, Postgres\n"+
					"and Temporal including, and a .env.example file.").
				WithButtonAlignment(lipgloss.Left).
				Value(&data.IncludeDocker).
				Affirmative("Yes").
				Negative("No"),
		),
	)
}
