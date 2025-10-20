package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/cmd/init/components"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/engine/runtime"
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
	InstallBun  bool
}

// ensureTemplatesRegistered is called before template operations
func ensureTemplatesRegistered() error {
	if err := basic.Register(); err != nil {
		if err.Error() == `template "basic" already registered` {
			return nil
		}
		return fmt.Errorf("failed to register basic template: %w", err)
	}
	return nil
}

// NewInitCommand creates the init command using the unified command pattern
func NewInitCommand() *cobra.Command {
	opts := defaultInitOptions()
	command := &cobra.Command{
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
			if err := prepareInitOptions(cobraCmd, opts, args); err != nil {
				return err
			}
			return executeInitCommand(cobraCmd, opts, args)
		},
	}
	applyInitFlags(command, opts)
	return command
}

func defaultInitOptions() *Options {
	return &Options{Version: "0.1.0"}
}

func applyInitFlags(command *cobra.Command, opts *Options) {
	command.Flags().StringVarP(&opts.Name, "name", "n", "", "Project name")
	command.Flags().StringVarP(&opts.Description, "description", "d", "", "Project description")
	command.Flags().StringVarP(&opts.Version, "version", "v", "0.1.0", "Project version")
	command.Flags().StringVarP(&opts.Template, "template", "t", "basic", "Project template")
	command.Flags().StringVar(&opts.Author, "author", "", "Author name")
	command.Flags().StringVar(&opts.AuthorURL, "author-url", "", "Author URL")
	command.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Force interactive mode")
	command.Flags().BoolVar(&opts.DockerSetup, "docker", false, "Include Docker Compose setup")
}

func prepareInitOptions(cobraCmd *cobra.Command, opts *Options, args []string) error {
	if len(args) > 0 {
		opts.Path = args[0]
	} else {
		opts.Path = "."
	}
	absPath, err := filepath.Abs(opts.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	opts.Path = absPath
	if opts.Name == "" && !cobraCmd.Flags().Changed("format") {
		opts.Interactive = true
	}
	return nil
}

// executeInitCommand handles the init command execution using the unified executor pattern
func executeInitCommand(cobraCmd *cobra.Command, opts *Options, args []string) error {
	executor, err := cmd.NewCommandExecutor(cobraCmd, cmd.ExecutorOptions{
		RequireAuth: false,
	})
	if err != nil {
		return cmd.HandleCommonErrors(err, helpers.DetectMode(cobraCmd))
	}
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
	logger.FromContext(ctx).Debug("executing init command in JSON mode")
	logDebugMode(ctx)
	if err := ensureNameProvided(opts); err != nil {
		return err
	}
	if err := validateProjectOptions(opts); err != nil {
		return err
	}
	if err := installBunIfNeeded(ctx, opts); err != nil {
		return err
	}
	if err := generateProjectStructure(opts); err != nil {
		return err
	}
	envFileName := determineEnvExampleFile(opts.Path)
	return outputInitJSON(buildInitJSONResponse(opts, envFileName))
}

// runInitTUI handles interactive TUI mode
func runInitTUI(ctx context.Context, _ *cobra.Command, _ *cmd.CommandExecutor, opts *Options) error {
	logger.FromContext(ctx).Debug("executing init command in TUI mode")
	logDebugMode(ctx)
	if err := runInteractiveForm(ctx, opts); err != nil {
		return fmt.Errorf("interactive form failed: %w", err)
	}
	if err := validateProjectOptions(opts); err != nil {
		return err
	}
	if err := installBunIfNeeded(ctx, opts); err != nil {
		return err
	}
	if err := generateProjectStructure(opts); err != nil {
		return err
	}
	envFileName := determineEnvExampleFile(opts.Path)
	printTUISuccess(opts, envFileName)
	return nil
}

func logDebugMode(ctx context.Context) {
	if cfg := config.FromContext(ctx); cfg != nil && cfg.CLI.Debug {
		logger.FromContext(ctx).Debug("debug mode enabled from global config")
	}
}

func ensureNameProvided(opts *Options) error {
	if opts.Name != "" {
		return nil
	}
	return fmt.Errorf("project name is required in non-interactive mode (use --name flag)")
}

func validateProjectOptions(opts *Options) error {
	if err := validator.New().Struct(opts); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return nil
}

func installBunIfNeeded(ctx context.Context, opts *Options) error {
	if !opts.InstallBun || runtime.IsBunAvailable() {
		return nil
	}
	if err := installBun(ctx); err != nil {
		return fmt.Errorf("failed to install Bun: %w", err)
	}
	return nil
}

func generateProjectStructure(opts *Options) error {
	if err := ensureTemplatesRegistered(); err != nil {
		return fmt.Errorf("failed to initialize templates: %w", err)
	}
	if err := template.GetService().Generate(opts.Template, buildGenerateOptions(opts)); err != nil {
		return fmt.Errorf("failed to generate project: %w", err)
	}
	return nil
}

func buildGenerateOptions(opts *Options) *template.GenerateOptions {
	return &template.GenerateOptions{
		Path:        opts.Path,
		Name:        opts.Name,
		Description: opts.Description,
		Version:     opts.Version,
		Author:      opts.Author,
		AuthorURL:   opts.AuthorURL,
		DockerSetup: opts.DockerSetup,
	}
}

func determineEnvExampleFile(projectPath string) string {
	envFileName := "env.example"
	if _, err := os.Stat(filepath.Join(projectPath, "env-compozy.example")); err == nil {
		envFileName = "env-compozy.example"
	}
	return envFileName
}

func buildInitJSONResponse(opts *Options, envFileName string) map[string]any {
	return map[string]any{
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
			"http":     "api.http",
			"workflow": "workflows/main.yaml",
		},
	}
}

func printTUISuccess(opts *Options, envFileName string) {
	fmt.Printf("🎉 Project '%s' initialized successfully!\n", opts.Name)
	fmt.Printf("📁 Location: %s\n", opts.Path)
	if opts.InstallBun {
		fmt.Printf("🏃 Bun runtime installed successfully!\n")
	}
	if opts.DockerSetup {
		fmt.Printf("\n🐳 Docker setup included:\n")
		fmt.Printf("  • docker-compose.yaml - Infrastructure services\n")
		fmt.Printf("  • %s - Environment variables template\n", envFileName)
	} else {
		fmt.Printf("\n📄 Configuration files created:\n")
		fmt.Printf("  • %s - Environment variables template\n", envFileName)
	}
	fmt.Printf("  • api.http - API test requests\n")
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
}

// runInteractiveForm runs the interactive form to collect project information
func runInteractiveForm(_ context.Context, opts *Options) error {
	projectData := &components.ProjectFormData{
		Name:          opts.Name,
		Description:   opts.Description,
		Version:       opts.Version,
		Author:        opts.Author,
		AuthorURL:     opts.AuthorURL,
		Template:      opts.Template,
		IncludeDocker: opts.DockerSetup,
		InstallBun:    opts.InstallBun,
	}
	initModel := components.NewInitModel(projectData)
	p := tea.NewProgram(initModel, tea.WithAltScreen(), tea.WithMouseAllMotion())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run project form: %w", err)
	}
	if m, ok := finalModel.(*components.InitModel); ok {
		if m.IsCanceled() {
			return fmt.Errorf("initialization canceled by user")
		}
	}
	opts.Name = projectData.Name
	opts.Description = projectData.Description
	opts.Version = projectData.Version
	opts.Author = projectData.Author
	opts.AuthorURL = projectData.AuthorURL
	opts.Template = projectData.Template
	opts.DockerSetup = projectData.IncludeDocker
	opts.InstallBun = projectData.InstallBun
	return nil
}

// installBun installs Bun using the official installer
func installBun(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Installing Bun runtime...")
	cmd := exec.CommandContext(ctx, "bash", "-c", "curl -fsSL https://bun.sh/install | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Bun: %w", err)
	}
	if !runtime.IsBunAvailable() {
		return fmt.Errorf("bun installation completed but executable not found in PATH. " +
			"You may need to restart your terminal or source your shell configuration")
	}
	log.Info("Bun installed successfully!")
	return nil
}

// outputInitJSON outputs a JSON response for init command
func outputInitJSON(data any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
