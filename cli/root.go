package cli

import (
	"context"
	"fmt"

	authcmd "github.com/compozy/compozy/cli/cmd/auth"
	configcmd "github.com/compozy/compozy/cli/cmd/config"
	"github.com/compozy/compozy/cli/cmd/dev"
	initcmd "github.com/compozy/compozy/cli/cmd/init"
	mcpproxycmd "github.com/compozy/compozy/cli/cmd/mcp_proxy"
	workflowcmd "github.com/compozy/compozy/cli/cmd/workflow"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "compozy",
		Short: "Compozy CLI tool for workflow orchestration",
		Long: `Compozy is a powerful workflow orchestration engine for AI agents.
This CLI provides complete control over workflows, executions, schedules,
and events with both interactive TUI and automation-friendly JSON output.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return SetupGlobalConfig(cmd)
		},
	}

	// Add comprehensive global flags
	helpers.AddGlobalFlags(root)
	root.AddCommand(
		initcmd.NewInitCommand(),
		dev.NewDevCommand(),
		mcpproxycmd.NewMCPProxyCommand(),
		configcmd.NewConfigCommand(),
		authcmd.Cmd(),
		workflowcmd.Cmd(),
	)

	return root
}

// SetupGlobalConfig configures the global configuration from flags and config file
func SetupGlobalConfig(cmd *cobra.Command) error {
	// Load environment file if specified
	if err := helpers.LoadEnvironmentFile(cmd); err != nil {
		return fmt.Errorf("failed to load environment file: %w", err)
	}

	// Extract CLI flag overrides
	cliFlags, err := helpers.ExtractCLIFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to extract CLI flags: %w", err)
	}

	// Create configuration service
	service := config.NewService()

	// Build sources with proper precedence: defaults -> env -> config file -> CLI flags
	sources := []config.Source{
		config.NewDefaultProvider(),
	}

	// Add config file if specified
	if configFile, err := cmd.Flags().GetString("config"); err == nil && configFile != "" {
		sources = append(sources, config.NewYAMLProvider(configFile))
	}

	sources = append(sources, config.NewEnvProvider())
	// Add CLI flags as highest precedence
	if len(cliFlags) > 0 {
		sources = append(sources, config.NewCLIProvider(cliFlags))
	}

	// Load configuration
	cfg, err := service.Load(context.Background(), sources...)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Setup logger based on configuration
	logLevel := logger.InfoLevel
	if cfg.CLI.Quiet {
		// In quiet mode, disable all logging to suppress non-essential output
		logLevel = logger.DisabledLevel
	} else if cfg.CLI.Debug {
		logLevel = logger.DebugLevel
	}

	log := logger.SetupLogger(logLevel, false, cfg.CLI.Debug)
	ctx := logger.ContextWithLogger(context.Background(), log)

	// Store config in context for subcommands
	ctx = context.WithValue(ctx, helpers.ConfigKey, cfg)
	cmd.SetContext(ctx)

	return nil
}
