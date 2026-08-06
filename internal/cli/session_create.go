package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

const sessionCreateExample = `  # Start a session in the current workspace using the configured default agent
  compozy session new

  # Start a named session for a specific registered workspace and agent
  compozy session new --workspace checkout-api --agent reviewer --name review-api

  # Auto-register an absolute workspace path before creating the session
  compozy session new --cwd "$PWD" --agent reviewer`

func newSessionCreateCommand(deps commandDeps) *cobra.Command {
	var (
		agentName    string
		cwd          string
		name         string
		workspaceRef string
		parentID     string
		networkFlags networkParticipationFlags
	)

	cmd := &cobra.Command{
		Use:     sessionNewKey,
		Short:   "Create a new session",
		Example: sessionCreateExample,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			workspace, workspacePath, err := resolveSessionCreateWorkspace(cmd, deps, client, workspaceRef, cwd)
			if err != nil {
				return err
			}
			participationRequest, err := networkFlags.namedRequest()
			if err != nil {
				return err
			}
			created, err := client.CreateSession(cmd.Context(), CreateSessionRequest{
				AgentName:            agentName,
				Name:                 name,
				Workspace:            workspace,
				WorkspacePath:        workspacePath,
				ParentSessionID:      strings.TrimSpace(parentID),
				NetworkParticipation: participationRequest,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(created, deps.now))
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "Agent definition name (defaults to config default)")
	cmd.Flags().
		StringVar(&workspaceRef, workspaceSkillSource, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Absolute workspace directory to auto-register")
	cmd.Flags().StringVar(&name, sessionNameKey, "", "Optional session label")
	cmd.Flags().StringVar(&parentID, "parent", "", "Record a same-workspace parent session as creation provenance")
	bindNamedNetworkParticipationFlags(cmd, &networkFlags)
	return cmd
}
