package init

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	html_template "html/template"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/cmd/init/components"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
)

// Options holds the configuration for the init command
type Options struct {
	Path        string `validate:"required"`
	Name        string `validate:"required"`
	Description string
	Version     string
	Template    string
	Author      string
	AuthorURL   string
	Interactive bool
	DockerSetup bool
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
//go:embed templates/basic/compozy.yaml.tmpl
var basicCompozyTemplate string

//go:embed templates/basic/entrypoint.ts.tmpl
var basicEntrypointTemplate string

//go:embed templates/basic/workflow.yaml.tmpl
var basicWorkflowTemplate string

//go:embed templates/basic/greeting_tool.ts.tmpl
var basicGreetingToolTemplate string

//go:embed templates/basic/docker-compose.yaml.tmpl
var basicDockerComposeTemplate string

//go:embed templates/basic/env.example.tmpl
var basicEnvExampleTemplate string

//go:embed templates/basic/compozy.http.tmpl
var basicCompozyHTTPTemplate string

// templateSet holds all templates for a specific template type
type templateSet struct {
	compozy       string
	entrypoint    string
	workflow      string
	greetingTool  string
	dockerCompose string
	envExample    string
	compozyHTTP   string
}

// getTemplateSet returns the appropriate template set based on the template name
// To add a new template:
// 1. Create a new directory under templates/ (e.g., templates/advanced/)
// 2. Add all template files to that directory
// 3. Add go:embed directives for each template file
// 4. Add a new case in this switch statement
// 5. Update the form in components/project_form.go to include the new option
func getTemplateSet(templateName string) (*templateSet, error) {
	switch templateName {
	case "basic":
		return &templateSet{
			compozy:       basicCompozyTemplate,
			entrypoint:    basicEntrypointTemplate,
			workflow:      basicWorkflowTemplate,
			greetingTool:  basicGreetingToolTemplate,
			dockerCompose: basicDockerComposeTemplate,
			envExample:    basicEnvExampleTemplate,
			compozyHTTP:   basicCompozyHTTPTemplate,
		}, nil
	default:
		return nil, fmt.Errorf("unknown template: %s", templateName)
	}
}

// getRequiredDirectories returns the directories that should be created for a given template
func getRequiredDirectories(templateName string) []string {
	switch templateName {
	case "basic":
		// Basic template only needs workflows directory
		// The autoload configuration in compozy.yaml handles agents and tools
		// but we don't create empty directories
		return []string{"workflows"}
	default:
		// Default to creating all directories for unknown templates
		return []string{"workflows", "tools", "agents"}
	}
}

// NewInitCommand creates the init command using the unified command pattern
func NewInitCommand() *cobra.Command {
	opts := &Options{
		Version: "0.1.0",
	}

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new Compozy project",
		Long: `Initialize a new Compozy project with the specified structure.
Creates a new project directory with compozy.yaml and workflows directory.

Examples:
  compozy init my-project
  compozy init --template basic ./my-project
  compozy init --name "My Project" --description "A workflow project"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
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

			// Force interactive mode if no name is provided and not explicitly non-interactive
			if opts.Name == "" && !cobraCmd.Flags().Changed("format") {
				opts.Interactive = true
			}
			return executeInitCommand(cobraCmd, opts, args)
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
	cmd.Flags().BoolVar(&opts.DockerSetup, "docker", false, "Include Docker Compose setup")

	return cmd
}

// executeInitCommand handles the init command execution using the unified executor pattern
func executeInitCommand(cobraCmd *cobra.Command, opts *Options, args []string) error {
	// For init command, we want to use interactive mode by default when name is not provided
	// unless the user explicitly sets format to json
	shouldBeInteractive := opts.Interactive || (opts.Name == "" && !cobraCmd.Flags().Changed("format"))

	// Create executor manually to control mode detection
	executor, err := cmd.NewCommandExecutor(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
	})
	if err != nil {
		return cmd.HandleCommonErrors(err, helpers.DetectMode(cobraCmd))
	}

	// If we should be interactive but mode was detected as JSON, switch to TUI
	if shouldBeInteractive && executor.GetMode() == models.ModeJSON {
		// Use TUI handler directly
		return runInitTUI(cobraCmd.Context(), cobraCmd, executor, opts)
	}

	// Otherwise use normal mode-based execution
	return cmd.HandleCommonErrors(executor.Execute(cobraCmd.Context(), cobraCmd, cmd.ModeHandlers{
		JSON: func(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return runInitJSON(ctx, cobraCmd, executor, opts)
		},
		TUI: func(ctx context.Context, cobraCmd *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return runInitTUI(ctx, cobraCmd, executor, opts)
		},
	}, args), executor.GetMode())
}

// runInitJSON handles non-interactive JSON mode
func runInitJSON(ctx context.Context, _ *cobra.Command, _ *cmd.CommandExecutor, opts *Options) error {
	log := logger.FromContext(ctx)
	log.Debug("executing init command in JSON mode")

	// Access global configuration from executor
	cfg := config.Get()
	if cfg.CLI.Debug {
		log.Debug("debug mode enabled from global config")
	}

	// Validate required fields for non-interactive mode
	if opts.Name == "" {
		return fmt.Errorf("project name is required in non-interactive mode (use --name flag)")
	}

	// Validate options
	validator := validator.New()
	if err := validator.Struct(opts); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create project configuration
	projectConfig := createProjectConfig(opts)

	// Create project structure
	if err := createProjectStructure(ctx, opts, projectConfig); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	// Check which env example file was created
	envFileName := "env.example"
	envCompozyPath := filepath.Join(opts.Path, "env-compozy.example")
	if _, err := os.Stat(envCompozyPath); err == nil {
		envFileName = "env-compozy.example"
	}

	// Output JSON response
	response := map[string]any{
		"success": true,
		"message": "Project initialized successfully",
		"path":    opts.Path,
		"name":    opts.Name,
		"version": opts.Version,
		"envFile": envFileName,
		"docker":  opts.DockerSetup,
		"files": map[string]string{
			"config":   "compozy.yaml",
			"env":      envFileName,
			"http":     "compozy.http",
			"workflow": "workflows/main.yaml",
		},
	}

	return outputInitJSON(response)
}

// runInitTUI handles interactive TUI mode
func runInitTUI(ctx context.Context, _ *cobra.Command, _ *cmd.CommandExecutor, opts *Options) error {
	log := logger.FromContext(ctx)
	log.Debug("executing init command in TUI mode")

	// Access global configuration from executor
	cfg := config.Get()
	if cfg.CLI.Debug {
		log.Debug("debug mode enabled from global config")
	}

	// Always run interactive form in TUI mode since we're in runInitTUI
	if err := runInteractiveForm(ctx, opts); err != nil {
		return fmt.Errorf("interactive form failed: %w", err)
	}

	// Validate options
	validator := validator.New()
	if err := validator.Struct(opts); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create project configuration
	projectConfig := createProjectConfig(opts)

	// Create project structure
	if err := createProjectStructure(ctx, opts, projectConfig); err != nil {
		return fmt.Errorf("failed to create project structure: %w", err)
	}

	// Display success message
	fmt.Printf("🎉 Project '%s' initialized successfully!\n", opts.Name)
	fmt.Printf("📁 Location: %s\n", opts.Path)

	// Check which env example file was created
	envFileName := "env.example"
	envCompozyPath := filepath.Join(opts.Path, "env-compozy.example")
	if _, err := os.Stat(envCompozyPath); err == nil {
		// env-compozy.example was created
		envFileName = "env-compozy.example"
	}

	if opts.DockerSetup {
		fmt.Printf("\n🐳 Docker setup included:\n")
		fmt.Printf("  • docker-compose.yaml - Infrastructure services\n")
		fmt.Printf("  • %s - Environment variables template\n", envFileName)
	} else {
		fmt.Printf("\n📄 Configuration files created:\n")
		fmt.Printf("  • %s - Environment variables template\n", envFileName)
	}

	fmt.Printf("  • compozy.http - API test requests\n")

	fmt.Printf("\n📋 Next steps:\n")
	fmt.Printf("  1. cd %s\n", opts.Path)
	fmt.Printf("  2. Copy %s to .env and add your API keys\n", envFileName)
	if opts.DockerSetup {
		fmt.Printf("  3. Run 'docker-compose up -d' to start services\n")
		fmt.Printf("  4. Run 'compozy dev' to start the development server\n")
	} else {
		fmt.Printf("  3. Edit compozy.yaml to configure your project\n")
		fmt.Printf("  4. Modify the example workflow in workflows/main.yaml\n")
		fmt.Printf("  5. Run 'compozy dev' to start the development server\n")
	}

	return nil
}

// runInteractiveForm runs the interactive form to collect project information
func runInteractiveForm(_ context.Context, opts *Options) error {
	// Create project form data from existing options
	projectData := &components.ProjectFormData{
		Name:          opts.Name,
		Description:   opts.Description,
		Version:       opts.Version,
		Author:        opts.Author,
		AuthorURL:     opts.AuthorURL,
		Template:      opts.Template,
		IncludeDocker: opts.DockerSetup,
	}

	// Create and run the init model with header and form
	initModel := components.NewInitModel(projectData)

	p := tea.NewProgram(initModel, tea.WithAltScreen(), tea.WithMouseAllMotion())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run project form: %w", err)
	}

	// Check if form was canceled
	if m, ok := finalModel.(*components.InitModel); ok {
		if m.IsCanceled() {
			return fmt.Errorf("initialization canceled by user")
		}
	}

	// Copy data back to options
	opts.Name = projectData.Name
	opts.Description = projectData.Description
	opts.Version = projectData.Version
	opts.Author = projectData.Author
	opts.AuthorURL = projectData.AuthorURL
	opts.Template = projectData.Template
	opts.DockerSetup = projectData.IncludeDocker

	return nil
}

// createProjectConfig creates the project configuration from options
func createProjectConfig(opts *Options) *ProjectConfig {
	config := &ProjectConfig{
		Name:        opts.Name,
		Version:     opts.Version,
		Description: opts.Description,
		Workflows: []WorkflowRef{
			{Source: "./workflows/main.yaml"},
		},
		Models: []ModelConfig{
			{
				Provider: "openai",
				Model:    "gpt-4.1-2025-04-14",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
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
	if opts.Author != "" {
		config.Author = &AuthorConfig{
			Name: opts.Author,
			URL:  opts.AuthorURL,
		}
	}

	return config
}

// createProjectStructure creates the project directory structure and files
func createProjectStructure(ctx context.Context, opts *Options, config *ProjectConfig) error {
	log := logger.FromContext(ctx)
	log.Debug("creating project structure", "path", opts.Path)

	// Check if project already exists to prevent data loss
	compozyConfigPath := filepath.Join(opts.Path, "compozy.yaml")
	if _, err := os.Stat(compozyConfigPath); err == nil {
		return fmt.Errorf("project already exists at %s - aborting to prevent overwrite", opts.Path)
	}

	// Create project directory if it doesn't exist
	if err := os.MkdirAll(opts.Path, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create subdirectories based on template
	subdirs := getRequiredDirectories(opts.Template)
	for _, subdir := range subdirs {
		dirPath := filepath.Join(opts.Path, subdir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", subdir, err)
		}
	}

	// Create compozy.yaml
	if err := createCompozyYAML(opts, config); err != nil {
		return fmt.Errorf("failed to create compozy.yaml: %w", err)
	}

	// Create example files based on template
	if err := createTemplateFiles(ctx, opts, config); err != nil {
		return fmt.Errorf("failed to create template files: %w", err)
	}

	return nil
}

// createCompozyYAML creates the compozy.yaml file
func createCompozyYAML(opts *Options, config *ProjectConfig) error {
	// Get the appropriate template set
	templates, err := getTemplateSet(opts.Template)
	if err != nil {
		return fmt.Errorf("failed to get template set: %w", err)
	}
	return createFromTemplate(opts, "compozy.yaml", templates.compozy, config)
}

// createTemplateFiles creates template-specific files
func createTemplateFiles(ctx context.Context, opts *Options, config *ProjectConfig) error {
	log := logger.FromContext(ctx)
	log.Debug("creating template files", "template", opts.Template)

	// Get the appropriate template set
	templates, err := getTemplateSet(opts.Template)
	if err != nil {
		return fmt.Errorf("failed to get template set: %w", err)
	}

	if err := createEntrypoint(opts, config, templates); err != nil {
		return fmt.Errorf("failed to create entrypoint: %w", err)
	}

	if err := createWorkflow(opts, config, templates); err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	if err := createGreetingTool(opts, config, templates); err != nil {
		return fmt.Errorf("failed to create greeting tool: %w", err)
	}

	if err := createCompozyHTTP(opts, config, templates); err != nil {
		return fmt.Errorf("failed to create compozy.http: %w", err)
	}

	// Always create env.example file
	if err := createEnvExample(opts, config, templates); err != nil {
		return fmt.Errorf("failed to create env example: %w", err)
	}

	// Create Docker setup files if requested
	if opts.DockerSetup {
		if err := createDockerCompose(opts, config, templates); err != nil {
			return fmt.Errorf("failed to create docker-compose.yaml: %w", err)
		}
	}

	return nil
}

// createEntrypoint creates the entrypoint.ts file
func createEntrypoint(opts *Options, config *ProjectConfig, templates *templateSet) error {
	if err := createFromTemplate(opts, "entrypoint.ts", templates.entrypoint, config); err != nil {
		return err
	}
	// Set executable permissions for the entrypoint file
	entrypointPath := filepath.Join(opts.Path, "entrypoint.ts")
	if err := os.Chmod(entrypointPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}
	return nil
}

// createWorkflow creates the main workflow file
func createWorkflow(opts *Options, config *ProjectConfig, templates *templateSet) error {
	workflowPath := filepath.Join("workflows", "main.yaml")
	return createFromTemplate(opts, workflowPath, templates.workflow, config)
}

// createGreetingTool creates the greeting_tool.ts file
func createGreetingTool(opts *Options, config *ProjectConfig, templates *templateSet) error {
	return createFromTemplate(opts, "greeting_tool.ts", templates.greetingTool, config)
}

// createCompozyHTTP creates the compozy.http file
func createCompozyHTTP(opts *Options, config *ProjectConfig, templates *templateSet) error {
	return createFromTemplate(opts, "compozy.http", templates.compozyHTTP, config)
}

// createDockerCompose creates the docker-compose.yaml file
func createDockerCompose(opts *Options, config *ProjectConfig, templates *templateSet) error {
	return createFromTemplate(opts, "docker-compose.yaml", templates.dockerCompose, config)
}

// createEnvExample creates the env.example file
func createEnvExample(opts *Options, config *ProjectConfig, templates *templateSet) error {
	// Check if env.example already exists
	envExamplePath := filepath.Join(opts.Path, "env.example")
	if _, err := os.Stat(envExamplePath); err == nil {
		// File exists, create env-compozy.example instead
		fmt.Printf("⚠️  env.example already exists, creating env-compozy.example instead\n")
		return createFromTemplate(opts, "env-compozy.example", templates.envExample, config)
	}
	// File doesn't exist, create env.example
	return createFromTemplate(opts, "env.example", templates.envExample, config)
}

// createFromTemplate creates a file from a template with enhanced escaping
func createFromTemplate(opts *Options, fileName, templateContent string, config *ProjectConfig) error {
	// Create custom template functions with enhanced escaping
	funcMap := sprig.TxtFuncMap()
	funcMap["jsEscape"] = html_template.JSEscapeString
	funcMap["yamlEscape"] = func(s string) string {
		// For YAML, properly escape quotes and special characters
		if strings.ContainsAny(s, "\"'\\:\n\r\t") {
			// Quote the string and escape internal quotes
			escaped := strings.ReplaceAll(s, "\"", "\\\"")
			return "\"" + escaped + "\""
		}
		return s
	}
	funcMap["htmlEscape"] = html.EscapeString

	// Parse template with enhanced functions
	tmpl, err := template.New(fileName).Funcs(funcMap).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse %s template: %w", fileName, err)
	}

	// Create the file with path validation
	cleanFileName := filepath.Clean(fileName)
	if strings.Contains(cleanFileName, "..") {
		return fmt.Errorf("invalid file name: %s contains path traversal", fileName)
	}
	filePath := filepath.Join(opts.Path, cleanFileName)
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
