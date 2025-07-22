package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/cmd/init/components"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/template"
	"github.com/compozy/compozy/pkg/template/templates/basic"
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

// ensureTemplatesRegistered is called before template operations
func ensureTemplatesRegistered() error {
	// Register default templates (idempotent operation)
	if err := basic.Register(); err != nil {
		// Check if it's already registered (not an error)
		if err.Error() == `template "basic" already registered` {
			return nil
		}
		return fmt.Errorf("failed to register basic template: %w", err)
	}
	return nil
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
	// Use standard mode detection without custom overrides

	// Create executor using standard mode detection
	executor, err := cmd.NewCommandExecutor(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
	})
	if err != nil {
		return cmd.HandleCommonErrors(err, helpers.DetectMode(cobraCmd))
	}

	// Use normal mode-based execution
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

	// Ensure templates are registered
	if err := ensureTemplatesRegistered(); err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}

	// Create project using template service
	templateService := template.GetService()
	generateOpts := &template.GenerateOptions{
		Path:        opts.Path,
		Name:        opts.Name,
		Description: opts.Description,
		Version:     opts.Version,
		Author:      opts.Author,
		AuthorURL:   opts.AuthorURL,
		DockerSetup: opts.DockerSetup,
	}

	if err := templateService.Generate(opts.Template, generateOpts); err != nil {
		return fmt.Errorf("failed to generate project: %w", err)
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

	// Ensure templates are registered
	if err := ensureTemplatesRegistered(); err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}

	// Create project using template service
	templateService := template.GetService()
	generateOpts := &template.GenerateOptions{
		Path:        opts.Path,
		Name:        opts.Name,
		Description: opts.Description,
		Version:     opts.Version,
		Author:      opts.Author,
		AuthorURL:   opts.AuthorURL,
		DockerSetup: opts.DockerSetup,
	}

	if err := templateService.Generate(opts.Template, generateOpts); err != nil {
		return fmt.Errorf("failed to generate project: %w", err)
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

// outputInitJSON outputs a JSON response for init command
func outputInitJSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
