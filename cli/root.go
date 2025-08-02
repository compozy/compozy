package cli

import (
	"context"
	"fmt"
	"os"

	authcmd "github.com/compozy/compozy/cli/cmd/auth"
	configcmd "github.com/compozy/compozy/cli/cmd/config"
	"github.com/compozy/compozy/cli/cmd/dev"
	initcmd "github.com/compozy/compozy/cli/cmd/init"
	"github.com/compozy/compozy/cli/cmd/start"
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
		Long: `Compozy is a powerful Next-level Agentic Orchestration Platform.
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
		start.NewStartCommand(),
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

	// Add config file if specified
	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return fmt.Errorf("failed to get config file: %w", err)
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

	// Initialize global config
	if err := config.Initialize(ctx, nil, sources...); err != nil {
		return fmt.Errorf("failed to initialize global configuration: %w", err)
	}

	// Setup logger based on configuration
	cfg := config.Get()
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
	config.OnChange(func(_ *config.Config) {
		// Update logger or other runtime settings if necessary
	})

	return nil
}
