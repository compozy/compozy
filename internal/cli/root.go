package cli

import (
	"log/slog"
	"sync"

	extcli "github.com/compozy/compozy/internal/cli/extension"
	"github.com/compozy/compozy/internal/core/agent"

	// Register the extension-backed run-scope factory used by kernel dispatchers.
	_ "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/spf13/cobra"
)

type commandKind string

const (
	commandKindFetchReviews commandKind = "reviews fetch"
	commandKindFixReviews   commandKind = "reviews fix"
	commandKindExec         commandKind = "exec"
	commandKindArchive      commandKind = "archive"
	commandKindStart        commandKind = "tasks run"
	commandKindSync         commandKind = "sync"
)

var validateRootDispatcher = kernel.ValidateDefaultRegistry

func newRootDispatcher() *kernel.Dispatcher {
	deps := kernel.KernelDeps{
		Logger:        slog.Default(),
		EventBus:      events.New[events.Event](0),
		Workspace:     workspace.Context{},
		AgentRegistry: agent.DefaultRegistry(),
	}

	dispatcher := kernel.BuildDefault(deps)
	if err := validateRootDispatcher(dispatcher); err != nil {
		slog.Default().Error("kernel dispatcher validation failed", "error", err)
	}
	return dispatcher
}

func newLazyRootDispatcher() func() *kernel.Dispatcher {
	return sync.OnceValue(newRootDispatcher)
}

// NewRootCommand returns the reusable compozy Cobra command.
func NewRootCommand() *cobra.Command {
	return newRootCommandWithDefaults(newLazyRootDispatcher(), defaultCommandStateDefaults())
}

func newRootCommandWithDefaults(dispatcher func() *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	root := &cobra.Command{
		Use:          "compozy",
		Short:        "Run AI review remediation and PRD task workflows",
		SilenceUsage: true,
		Long: `Compozy manages review rounds and PRD execution workflows.

Defaults can be stored in ~/.compozy/config.toml and overridden per workspace in
.compozy/config.toml. Explicit CLI flags always override values loaded from config files.

Use explicit workflow subcommands:
  compozy setup         Install bundled public skills for supported agents
  compozy agents        Discover and inspect reusable agents
  compozy upgrade       Update the CLI to the latest release
  compozy ext           Manage bundled, user, and workspace extensions
  compozy migrate       Convert legacy workflow artifacts to frontmatter
  compozy daemon        Manage the home-scoped daemon bootstrap lifecycle
  compozy workspaces    Inspect daemon workspace registrations
  compozy tasks         Inspect, validate, and run task workflows
  compozy reviews       Fetch, inspect, and remediate review workflows
  compozy runs          Inspect and clean persisted daemon run artifacts
  compozy sync          Reconcile workflow artifacts into global.db
  compozy archive       Move fully completed workflows into .compozy/tasks/_archived/
  compozy exec          Execute one ad hoc prompt through the shared ACP runtime`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(
		newSetupCommand(nil),
		newAgentsCommand(),
		newUpgradeCommand(),
		extcli.NewExtCommand(nil),
		newMigrateCommand(dispatcher),
		newDaemonCommand(),
		newWorkspacesCommand(),
		newTasksCommand(nil, defaults),
		newReviewsCommandWithDefaults(defaults),
		newRunsCommandWithDefaults(defaults),
		newSyncCommand(dispatcher),
		newArchiveCommand(dispatcher),
		newExecCommandWithDefaults(defaults),
		newMCPServeCommand(),
	)
	return root
}
