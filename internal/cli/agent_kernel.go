package cli

import (
	"github.com/spf13/cobra"
)

const (
	automationSessionIDKey = "session_id"
)

const (
	cliOutputKindValue    = "Kind"
	cliOutputTimestampKey = "timestamp"
)

const (
	agentKernelModelKey = "model"
)

const (
	agentKernelAgentValue   = "Agent"
	agentKernelRootValue    = "Root"
	agentKernelSessionValue = "Session"
	agentKernelAgentNameKey = "agent_name"
	agentKernelContextKey   = "context"
	agentKernelKindKey      = cliKindKey
	agentKernelListKey      = "list"
	agentKernelRunIDKey     = "run_id"
)

func newMeCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "Inspect the current CompozyOS-managed agent session",
		Example: `  # Show the current managed session identity
  compozy me

  # Print machine-readable caller state
  compozy me -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("me"))
			if err != nil {
				return err
			}
			record, err := client.AgentMe(cmd.Context(), credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentMeBundle(record))
		},
	}
	cmd.AddCommand(newMeContextCommand(deps))
	return cmd
}

func newMeContextCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   agentKernelContextKey,
		Short: "Inspect the bounded situation context for the current agent session",
		Example: `  # Show the bounded situation context injected for this session
  compozy me context

  # Read the context payload as JSON
  compozy me context -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("me.context"))
			if err != nil {
				return err
			}
			record, err := client.AgentContext(cmd.Context(), credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentContextBundle(&record))
		},
	}
}
