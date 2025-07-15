package cli

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/auth"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
)

// InitOptions holds the configuration for the init command
type InitOptions struct {
	Path        string `validate:"required"`
	Name        string `validate:"required"`
	Description string
	Version     string
	Template    string
	Author      string
	AuthorURL   string
	Interactive bool
}

// ProjectConfig represents the structure of compozy.yaml
type ProjectConfig struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Author      *AuthorConfig     `yaml:"author,omitempty"`
	Workflows   []WorkflowRef     `yaml:"workflows,omitempty"`
	Models      []ModelConfig     `yaml:"models,omitempty"`
	Runtime     *RuntimeConfig    `yaml:"runtime,omitempty"`
	Autoload    *AutoloadConfig   `yaml:"autoload,omitempty"`
	Templates   map[string]string `yaml:"templates,omitempty"`
}

// AuthorConfig represents the author section
type AuthorConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url,omitempty"`
}

// WorkflowRef represents a workflow reference
type WorkflowRef struct {
	Source string `yaml:"source"`
}

// ModelConfig represents a model configuration
type ModelConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key,omitempty"`
	APIURL   string `yaml:"api_url,omitempty"`
}

// RuntimeConfig represents the runtime configuration
type RuntimeConfig struct {
	Type        string   `yaml:"type"`
	Entrypoint  string   `yaml:"entrypoint"`
	Permissions []string `yaml:"permissions,omitempty"`
}

// AutoloadConfig represents the autoload configuration
type AutoloadConfig struct {
	Enabled bool     `yaml:"enabled"`
	Strict  bool     `yaml:"strict"`
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// Embedded template files
//
//go:embed templates/compozy.yaml.tmpl
var compozyTemplate string

//go:embed templates/entrypoint.ts.tmpl
var entrypointTemplate string

//go:embed templates/workflow.yaml.tmpl
var workflowTemplate string

//go:embed templates/readme.md.tmpl
var readmeTemplate string

// Form field indices for maintainability
const (
	formFieldName = iota
	formFieldDescription
	formFieldVersion
	formFieldAuthor
	formFieldAuthorURL
	formFieldTemplate
	formFieldCount
)

// Form field configuration for consolidation
type formFieldConfig struct {
	Placeholder  string
	CharLimit    int
	Width        int
	DefaultValue string
}

// Form field configurations
var formFieldConfigs = map[int]formFieldConfig{
	formFieldName: {
		Placeholder: "Enter project name",
		CharLimit:   50,
		Width:       30,
	},
	formFieldDescription: {
		Placeholder: "Enter project description",
		CharLimit:   200,
		Width:       50,
	},
	formFieldVersion: {
		Placeholder:  "Enter project version",
		CharLimit:    20,
		Width:        20,
		DefaultValue: "0.1.0",
	},
	formFieldAuthor: {
		Placeholder: "Enter author name",
		CharLimit:   50,
		Width:       30,
	},
	formFieldAuthorURL: {
		Placeholder: "Enter author URL",
		CharLimit:   100,
		Width:       40,
	},
	formFieldTemplate: {
		Placeholder:  "Enter template name",
		CharLimit:    30,
		Width:        20,
		DefaultValue: "basic",
	},
}

// initFormModel represents the interactive form model
type initFormModel struct {
	models.BaseModel
	inputs    []textinput.Model
	focused   int
	submitted bool
	opts      *InitOptions
}

// newInitForm creates a new interactive form
func newInitForm(opts *InitOptions) *initFormModel {
	inputs := make([]textinput.Model, formFieldCount)

	// Get option values map for easier access
	optionValues := map[int]string{
		formFieldName:        opts.Name,
		formFieldDescription: opts.Description,
		formFieldVersion:     opts.Version,
		formFieldAuthor:      opts.Author,
		formFieldAuthorURL:   opts.AuthorURL,
		formFieldTemplate:    opts.Template,
	}

	// Configure all form fields using the consolidated configuration
	for i := 0; i < formFieldCount; i++ {
		config := formFieldConfigs[i]
		inputs[i] = textinput.New()
		inputs[i].Placeholder = config.Placeholder
		inputs[i].CharLimit = config.CharLimit
		inputs[i].Width = config.Width

		// Set value from options or default
		if value := optionValues[i]; value != "" {
			inputs[i].SetValue(value)
		} else if config.DefaultValue != "" {
			inputs[i].SetValue(config.DefaultValue)
		}
	}

	// Focus the first input
	inputs[formFieldName].Focus()

	return &initFormModel{
		BaseModel: models.NewBaseModel(context.Background(), models.ModeTUI),
		inputs:    inputs,
		focused:   0,
		opts:      opts,
	}
}

// Init initializes the form
func (m *initFormModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles form updates
func (m *initFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.focused == len(m.inputs)-1 {
				m.submitted = true
				return m, tea.Quit
			}
			m.nextInput()
		case "tab", "shift+tab", "up", "down":
			if keyMsg.String() == "up" || keyMsg.String() == "shift+tab" {
				m.prevInput()
			} else {
				m.nextInput()
			}
		}
	}

	// Update the focused input
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the form
func (m *initFormModel) View() string {
	if m.submitted {
		return "✅ Form submitted successfully!\n"
	}

	var b strings.Builder
	b.WriteString("🚀 Initialize New Compozy Project\n\n")

	fields := []string{
		"Project Name:",
		"Description:",
		"Version:",
		"Author:",
		"Author URL:",
		"Template:",
	}

	for i, field := range fields {
		focused := i == m.focused
		if focused {
			b.WriteString("❯ ")
		} else {
			b.WriteString("  ")
		}

		b.WriteString(field)
		b.WriteString("\n  ")
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n\n")
	}

	b.WriteString("Press Enter to continue, Tab to navigate, Ctrl+C to quit\n")
	return b.String()
}

// nextInput focuses the next input
func (m *initFormModel) nextInput() {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused + 1) % len(m.inputs)
	m.inputs[m.focused].Focus()
}

// prevInput focuses the previous input
func (m *initFormModel) prevInput() {
	m.inputs[m.focused].Blur()
	m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
	m.inputs[m.focused].Focus()
}

// Getter methods for form data
func (m *initFormModel) getName() string {
	return strings.TrimSpace(m.inputs[formFieldName].Value())
}

func (m *initFormModel) getDescription() string {
	return strings.TrimSpace(m.inputs[formFieldDescription].Value())
}

func (m *initFormModel) getVersion() string {
	return strings.TrimSpace(m.inputs[formFieldVersion].Value())
}

func (m *initFormModel) getAuthor() string {
	return strings.TrimSpace(m.inputs[formFieldAuthor].Value())
}

func (m *initFormModel) getAuthorURL() string {
	return strings.TrimSpace(m.inputs[formFieldAuthorURL].Value())
}

func (m *initFormModel) getTemplate() string {
	return strings.TrimSpace(m.inputs[formFieldTemplate].Value())
}

// InitCmd creates the init command
func InitCmd() *cobra.Command {
	opts := &InitOptions{
		Version: "0.1.0",
	}

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new Compozy project",
		Long: `Initialize a new Compozy project with the specified structure.
Creates a new project directory with compozy.yaml, workflows/, tools/, and agents/ directories.

Examples:
  compozy init my-project
  compozy init --template basic ./my-project
  compozy init --name "My Project" --description "A workflow project"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			log := logger.FromContext(ctx)

			// Set path from args or use current directory
			if len(args) > 0 {
				opts.Path = args[0]
			} else {
				opts.Path = "."
			}

			// Make path absolute
			absPath, err := filepath.Abs(opts.Path)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}
			opts.Path = absPath

			// Execute command using CommandExecutor pattern
			executor := &initExecutor{
				opts:      opts,
				validator: validator.New(),
			}

			// Detect mode
			mode := auth.DetectMode(cmd)
			log.Debug("detected mode", "mode", mode)

			switch mode {
			case models.ModeJSON:
				return executor.executeJSON(ctx, cmd)
			case models.ModeTUI:
				return executor.executeTUI(ctx, cmd)
			default:
				return fmt.Errorf("unsupported mode: %s", mode)
			}
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&opts.Name, "name", "n", "", "Project name")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "Project description")
	cmd.Flags().StringVarP(&opts.Version, "version", "v", "0.1.0", "Project version")
	cmd.Flags().StringVarP(&opts.Template, "template", "t", "basic", "Project template")
	cmd.Flags().StringVar(&opts.Author, "author", "", "Author name")
	cmd.Flags().StringVar(&opts.AuthorURL, "author-url", "", "Author URL")
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Force interactive mode")

	// Add mode detection flags
	auth.AddModeFlags(cmd)

	return cmd
}

// initExecutor implements the command execution logic
type initExecutor struct {
	opts      *InitOptions
	validator *validator.Validate
}

// executeJSON handles non-interactive JSON mode
func (e *initExecutor) executeJSON(ctx context.Context, _ *cobra.Command) error {
	log := logger.FromContext(ctx)
	log.Debug("executing init command in JSON mode")

	// Validate required fields for non-interactive mode
	if e.opts.Name == "" {
		return fmt.Errorf("project name is required in non-interactive mode (use --name flag)")
	}

	// Validate options
	if err := e.validator.Struct(e.opts); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create project configuration
	projectConfig := e.createProjectConfig()

	// Create project structure
	if err := e.createProjectStructure(ctx, projectConfig); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	// Output JSON response
	response := map[string]any{
		"success": true,
		"message": "Project initialized successfully",
		"path":    e.opts.Path,
		"name":    e.opts.Name,
		"version": e.opts.Version,
	}

	return outputInitJSON(response)
}

// executeTUI handles interactive TUI mode
func (e *initExecutor) executeTUI(ctx context.Context, _ *cobra.Command) error {
	log := logger.FromContext(ctx)
	log.Debug("executing init command in TUI mode")

	// If name is not provided OR interactive flag is set, start interactive form
	if e.opts.Interactive || e.opts.Name == "" {
		if err := e.runInteractiveForm(ctx); err != nil {
			return fmt.Errorf("interactive form failed: %w", err)
		}
	}

	// Validate options
	if err := e.validator.Struct(e.opts); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create project configuration
	projectConfig := e.createProjectConfig()

	// Create project structure
	if err := e.createProjectStructure(ctx, projectConfig); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	// Display success message
	fmt.Printf("🎉 Project '%s' initialized successfully!\n", e.opts.Name)
	fmt.Printf("📁 Location: %s\n", e.opts.Path)
	fmt.Printf("\n📋 Next steps:\n")
	fmt.Printf("  1. cd %s\n", e.opts.Path)
	fmt.Printf("  2. Edit compozy.yaml to configure your project\n")
	fmt.Printf("  3. Add your workflows to the workflows/ directory\n")
	fmt.Printf("  4. Add your tools to the tools/ directory\n")
	fmt.Printf("  5. Run 'compozy dev' to start the development server\n")

	return nil
}

// runInteractiveForm runs the interactive form to collect project information
func (e *initExecutor) runInteractiveForm(_ context.Context) error {
	// Create and run the interactive form
	form := newInitForm(e.opts)

	program := tea.NewProgram(form, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run interactive form: %w", err)
	}

	// Get the final form data
	if formModel, ok := finalModel.(*initFormModel); ok {
		e.opts.Name = formModel.getName()
		e.opts.Description = formModel.getDescription()
		e.opts.Version = formModel.getVersion()
		e.opts.Author = formModel.getAuthor()
		e.opts.AuthorURL = formModel.getAuthorURL()
		e.opts.Template = formModel.getTemplate()
	}

	return nil
}

// createProjectConfig creates the project configuration from options
func (e *initExecutor) createProjectConfig() *ProjectConfig {
	config := &ProjectConfig{
		Name:        e.opts.Name,
		Version:     e.opts.Version,
		Description: e.opts.Description,
		Workflows: []WorkflowRef{
			{Source: "./workflows/main.yaml"},
		},
		Models: []ModelConfig{
			{
				Provider: "groq",
				Model:    "llama-3.3-70b-versatile",
				APIKey:   "{{ .env.GROQ_API_KEY }}",
			},
		},
		Runtime: &RuntimeConfig{
			Type:       "bun",
			Entrypoint: "./entrypoint.ts",
			Permissions: []string{
				"--allow-read",
				"--allow-net",
				"--allow-write",
			},
		},
		Autoload: &AutoloadConfig{
			Enabled: true,
			Strict:  true,
			Include: []string{
				"agents/*.yaml",
				"tools/*.yaml",
			},
			Exclude: []string{
				"**/*~",
				"**/*.bak",
				"**/*.tmp",
			},
		},
	}

	// Add author if provided
	if e.opts.Author != "" {
		config.Author = &AuthorConfig{
			Name: e.opts.Author,
			URL:  e.opts.AuthorURL,
		}
	}

	return config
}

// createProjectStructure creates the project directory structure and files
func (e *initExecutor) createProjectStructure(ctx context.Context, config *ProjectConfig) error {
	log := logger.FromContext(ctx)
	log.Debug("creating project structure", "path", e.opts.Path)

	// Check if project already exists to prevent data loss
	compozyConfigPath := filepath.Join(e.opts.Path, "compozy.yaml")
	if _, err := os.Stat(compozyConfigPath); err == nil {
		return fmt.Errorf("project already exists at %s - aborting to prevent overwrite", e.opts.Path)
	}

	// Create project directory if it doesn't exist
	if err := os.MkdirAll(e.opts.Path, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"workflows", "tools", "agents"}
	for _, subdir := range subdirs {
		dirPath := filepath.Join(e.opts.Path, subdir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", subdir, err)
		}
	}

	// Create compozy.yaml
	if err := e.createCompozyYAML(config); err != nil {
		return fmt.Errorf("failed to create compozy.yaml: %w", err)
	}

	// Create example files based on template
	if err := e.createTemplateFiles(ctx, config); err != nil {
		return fmt.Errorf("failed to create template files: %w", err)
	}

	return nil
}

// createCompozyYAML creates the compozy.yaml file
func (e *initExecutor) createCompozyYAML(config *ProjectConfig) error {
	return e.createFromTemplate("compozy.yaml", compozyTemplate, config)
}

// createTemplateFiles creates template-specific files
func (e *initExecutor) createTemplateFiles(ctx context.Context, config *ProjectConfig) error {
	log := logger.FromContext(ctx)
	log.Debug("creating template files", "template", e.opts.Template)

	if err := e.createEntrypoint(config); err != nil {
		return fmt.Errorf("failed to create entrypoint: %w", err)
	}

	if err := e.createWorkflow(config); err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	if err := e.createReadme(config); err != nil {
		return fmt.Errorf("failed to create README: %w", err)
	}

	return nil
}

// createEntrypoint creates the entrypoint.ts file
func (e *initExecutor) createEntrypoint(config *ProjectConfig) error {
	if err := e.createFromTemplate("entrypoint.ts", entrypointTemplate, config); err != nil {
		return err
	}
	// Set executable permissions for the entrypoint file
	entrypointPath := filepath.Join(e.opts.Path, "entrypoint.ts")
	if err := os.Chmod(entrypointPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}
	return nil
}

// createWorkflow creates the main workflow file
func (e *initExecutor) createWorkflow(config *ProjectConfig) error {
	workflowPath := filepath.Join("workflows", "main.yaml")
	return e.createFromTemplate(workflowPath, workflowTemplate, config)
}

// createReadme creates the README.md file
func (e *initExecutor) createReadme(config *ProjectConfig) error {
	return e.createFromTemplate("README.md", readmeTemplate, config)
}

// createFromTemplate creates a file from a template with enhanced escaping
func (e *initExecutor) createFromTemplate(fileName, templateContent string, config *ProjectConfig) error {
	// Create custom template functions with enhanced escaping
	funcMap := sprig.TxtFuncMap()
	funcMap["jsEscape"] = func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\"")
	}
	funcMap["yamlEscape"] = func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\":", "\\\"")
	}
	funcMap["htmlEscape"] = html.EscapeString

	// Parse template with enhanced functions
	tmpl, err := template.New(fileName).Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse %s template: %w", fileName, err)
	}

	// Create the file
	filePath := filepath.Join(e.opts.Path, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", fileName, err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, config); err != nil {
		return fmt.Errorf("failed to execute %s template: %w", fileName, err)
	}

	return nil
}

// outputInitJSON outputs a JSON response for init command
func outputInitJSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
