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
	knowledgecmd "github.com/compozy/compozy/cli/cmd/knowledge"
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
	root := newRootCommand(&showVersion)
	configureRootFlags(root, &showVersion)
	registerRootSubcommands(root)
	return root
}

// SetupGlobalConfig configures the global configuration from flags and config file
func SetupGlobalConfig(cmd *cobra.Command) error {
	if err := helpers.LoadEnvironmentFile(cmd); err != nil {
		return fmt.Errorf("failed to load environment file: %w", err)
	}
	ctx := cmd.Context()
	cliFlags, err := helpers.ExtractCLIFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to extract CLI flags: %w", err)
	}
	sources := buildConfigSources(cmd, cliFlags)
	newCtx, err := loadConfigManager(ctx, sources)
	if err != nil {
		return err
	}
	ctx = newCtx
	cfg := config.FromContext(ctx)
	ensureDefaultCWD(cfg)
	ctx = attachLogger(ctx, cfg)
	cmd.SetContext(ctx)
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

func newRootCommand(showVersion *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "compozy",
		Short: "Compozy CLI tool for workflow orchestration",
		Long: `Compozy is a powerful Next-level Agentic Orchestration Platform.
	This CLI provides complete control over workflows, executions, schedules,
	and events with both interactive TUI and automation-friendly JSON output.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return SetupGlobalConfig(cmd)
		},
		RunE: rootRunHandler(showVersion),
	}
}

func configureRootFlags(root *cobra.Command, showVersion *bool) {
	root.Flags().BoolVarP(showVersion, "version", "v", false, "Show version information")
	helpers.AddGlobalFlags(root)
	helpers.SetupCategorizedHelp(root)
}

func registerRootSubcommands(root *cobra.Command) {
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
		knowledgecmd.Cmd(),
		mcpcmd.Cmd(),
		schemascmd.Cmd(),
		modelscmd.Cmd(),
		memoriescmd.Cmd(),
		projectcmd.Cmd(),
		buildVersionCommand(),
	)
}

func rootRunHandler(showVersion *bool) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if showVersion != nil && *showVersion {
			renderVersion()
			return nil
		}
		cmd.HelpFunc()(cmd, []string{})
		return nil
	}
}

func renderVersion() {
	info := version.Get()
	fmt.Printf("compozy version %s\n", info.Version)
	fmt.Printf("commit: %s\n", info.CommitHash)
	fmt.Printf("built: %s\n", info.BuildDate)
}

func buildVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(_ *cobra.Command, _ []string) {
			renderVersion()
		},
	}
}

func buildConfigSources(cmd *cobra.Command, cliFlags map[string]any) []config.Source {
	sources := []config.Source{
		config.NewDefaultProvider(),
		config.NewEnvProvider(),
	}
	if configFile := resolveConfigFile(cmd); configFile != "" {
		sources = append(sources, config.NewYAMLProvider(configFile))
	} else if _, err := os.Stat("compozy.yaml"); err == nil {
		sources = append(sources, config.NewYAMLProvider("compozy.yaml"))
	}
	if len(cliFlags) > 0 {
		sources = append(sources, config.NewCLIProvider(cliFlags))
	}
	return sources
}

func resolveConfigFile(cmd *cobra.Command) string {
	if flag := cmd.PersistentFlags().Lookup("config"); flag != nil {
		if value, err := cmd.PersistentFlags().GetString("config"); err == nil {
			return value
		}
	}
	if flag := cmd.Flags().Lookup("config"); flag != nil {
		if value, err := cmd.Flags().GetString("config"); err == nil {
			return value
		}
	}
	return ""
}

func loadConfigManager(ctx context.Context, sources []config.Source) (context.Context, error) {
	mgr := config.NewManager(ctx, nil)
	if _, err := mgr.Load(ctx, sources...); err != nil {
		return nil, fmt.Errorf("failed to initialize configuration: %w", err)
	}
	ctx = config.ContextWithManager(ctx, mgr)
	return ctx, nil
}

func attachLogger(ctx context.Context, cfg *config.Config) context.Context {
	if cfg == nil {
		return ctx
	}
	level := logger.InfoLevel
	if cfg.CLI.Quiet {
		level = logger.DisabledLevel
	} else if cfg.CLI.Debug {
		level = logger.DebugLevel
	}
	log := logger.SetupLogger(level, false, cfg.CLI.Debug)
	return logger.ContextWithLogger(ctx, log)
}
