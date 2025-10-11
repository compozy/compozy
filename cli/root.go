package cli

import (
	"context"
	"fmt"
	"os"

	agentscmd "github.com/compozy/compozy/cli/cmd/agents"
	authcmd "github.com/compozy/compozy/cli/cmd/auth"
	configcmd "github.com/compozy/compozy/cli/cmd/config"
	"github.com/compozy/compozy/cli/cmd/dev"
	initcmd "github.com/compozy/compozy/cli/cmd/init"
	mcpproxycmd "github.com/compozy/compozy/cli/cmd/mcpproxy"
	mcpcmd "github.com/compozy/compozy/cli/cmd/mcps"
	memoriescmd "github.com/compozy/compozy/cli/cmd/memories"
	modelscmd "github.com/compozy/compozy/cli/cmd/models"
	projectcmd "github.com/compozy/compozy/cli/cmd/project"
	schemascmd "github.com/compozy/compozy/cli/cmd/schemas"
	"github.com/compozy/compozy/cli/cmd/start"
	taskscmd "github.com/compozy/compozy/cli/cmd/tasks"
	toolscmd "github.com/compozy/compozy/cli/cmd/tools"
	workflowcmd "github.com/compozy/compozy/cli/cmd/workflow"
	workflowscmd "github.com/compozy/compozy/cli/cmd/workflows"
	"github.com/compozy/compozy/cli/helpers"

	// Ensure native builtin tools register themselves.
	_ "github.com/compozy/compozy/engine/tool/builtin/imports"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/version"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	var showVersion bool

	root := &cobra.Command{
		Use:   "compozy",
		Short: "Compozy CLI tool for workflow orchestration",
		Long: `Compozy is a powerful Next-level Agentic Orchestration Platform.
This CLI provides complete control over workflows, executions, schedules,
and events with both interactive TUI and automation-friendly JSON output.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return SetupGlobalConfig(cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Handle --version flag when no subcommand is provided
			if showVersion {
				info := version.Get()
				fmt.Printf("compozy version %s\n", info.Version)
				fmt.Printf("commit: %s\n", info.CommitHash)
				fmt.Printf("built: %s\n", info.BuildDate)
				return nil
			}
			// Show help if no subcommand is provided
			cmd.HelpFunc()(cmd, []string{})
			return nil
		},
	}

	// Add --version flag
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	// Add comprehensive global flags
	helpers.AddGlobalFlags(root)

	// Set up categorized help
	helpers.SetupCategorizedHelp(root)

	// Add version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			info := version.Get()
			fmt.Printf("compozy version %s\n", info.Version)
			fmt.Printf("commit: %s\n", info.CommitHash)
			fmt.Printf("built: %s\n", info.BuildDate)
		},
	}

	root.AddCommand(
		initcmd.NewInitCommand(),
		dev.NewDevCommand(),
		start.NewStartCommand(),
		mcpproxycmd.NewMCPProxyCommand(),
		configcmd.NewConfigCommand(),
		authcmd.Cmd(),
		agentscmd.Cmd(),
		workflowcmd.Cmd(),
		workflowscmd.Cmd(),
		taskscmd.Cmd(),
		toolscmd.Cmd(),
		mcpcmd.Cmd(),
		schemascmd.Cmd(),
		modelscmd.Cmd(),
		memoriescmd.Cmd(),
		projectcmd.Cmd(),
		versionCmd,
	)

	return root
}

// SetupGlobalConfig configures the global configuration from flags and config file
func SetupGlobalConfig(cmd *cobra.Command) error {
	// Load environment file if specified
	if err := helpers.LoadEnvironmentFile(cmd); err != nil {
		return fmt.Errorf("failed to load environment file: %w", err)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Extract CLI flag overrides
	cliFlags, err := helpers.ExtractCLIFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to extract CLI flags: %w", err)
	}

	// Build sources with proper precedence: defaults -> env -> config file -> CLI flags
	sources := []config.Source{
		config.NewDefaultProvider(),
		config.NewEnvProvider(),
	}

	// Add config file if specified (support root-level persistent flags)
	var configFile string
	if f := cmd.PersistentFlags().Lookup("config"); f != nil {
		if v, err := cmd.PersistentFlags().GetString("config"); err == nil {
			configFile = v
		}
	} else if f := cmd.Flags().Lookup("config"); f != nil {
		if v, err := cmd.Flags().GetString("config"); err == nil {
			configFile = v
		}
	}
	if configFile != "" {
		sources = append(sources, config.NewYAMLProvider(configFile))
	} else {
		// Default to ./compozy.yaml if exists
		if _, err := os.Stat("compozy.yaml"); err == nil {
			sources = append(sources, config.NewYAMLProvider("compozy.yaml"))
		}
	}

	// Add CLI flags as highest precedence
	if len(cliFlags) > 0 {
		sources = append(sources, config.NewCLIProvider(cliFlags))
	}

	// Build a manager and attach it to context
	mgr := config.NewManager(nil)
	if _, err := mgr.Load(ctx, sources...); err != nil {
		return fmt.Errorf("failed to initialize configuration: %w", err)
	}
	ctx = config.ContextWithManager(ctx, mgr)

	// Application mode removed in greenfield cleanup; no mode injection.

	// Ensure a single source of truth for working directory
	cfg := config.FromContext(ctx)
	ensureDefaultCWD(cfg)

	// Setup logger based on configuration
	logLevel := logger.InfoLevel
	if cfg.CLI.Quiet {
		logLevel = logger.DisabledLevel
	} else if cfg.CLI.Debug {
		logLevel = logger.DebugLevel
	}

	log := logger.SetupLogger(logLevel, false, cfg.CLI.Debug)
	ctx = logger.ContextWithLogger(ctx, log)
	cmd.SetContext(ctx)

	// Optional: Register OnChange for hot-reload if needed (e.g., for long-running commands)
	mgr.OnChange(func(_ *config.Config) {
		// Update logger or other runtime settings if necessary
	})

	return nil
}

// ensureDefaultCWD guarantees cfg.CLI.CWD is set to an absolute path.
func ensureDefaultCWD(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.CLI.CWD != "" {
		return
	}
	if wd, err := os.Getwd(); err == nil {
		cfg.CLI.CWD = wd
	}
}
