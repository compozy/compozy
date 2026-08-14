package cli

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const sessionCreateExample = `  # Start a session in the current workspace using the configured default agent
  compozy session new

  # Start a named session for a specific registered workspace and agent
  compozy session new --workspace checkout-api --agent reviewer --name review-api

  # Auto-register an absolute workspace path before creating the session
  compozy session new --cwd "$PWD" --agent reviewer`

const sessionNewWorktreeAutoValue = "__compozy_auto__"

func newSessionCreateCommand(deps commandDeps) *cobra.Command {
	var (
		agentName    string
		cwd          string
		name         string
		workspaceRef string
		parentID     string
		worktreeRef  string
		newWorktree  string
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
			request := CreateSessionRequest{
				AgentName:            agentName,
				Name:                 name,
				Workspace:            workspace,
				WorkspacePath:        workspacePath,
				Worktree:             strings.TrimSpace(worktreeRef),
				ParentSessionID:      strings.TrimSpace(parentID),
				NetworkParticipation: participationRequest,
			}
			if cmd.Flags().Changed("new-worktree") {
				name := strings.TrimSpace(newWorktree)
				if name == sessionNewWorktreeAutoValue {
					name = ""
				}
				request.NewWorktree = &contract.NewSessionWorktreeRequest{Name: name}
			}
			created, err := client.CreateSession(cmd.Context(), request)
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
	cmd.Flags().StringVar(&worktreeRef, "worktree", "", "Start in an existing ready worktree")
	cmd.Flags().
		StringVar(&newWorktree, "new-worktree", "", "Create a worktree before starting; optionally provide its name")
	cmd.Flags().Lookup("new-worktree").NoOptDefVal = sessionNewWorktreeAutoValue
	cmd.Flags().StringVar(&name, sessionNameKey, "", "Optional session label")
	cmd.Flags().StringVar(&parentID, "parent", "", "Record a same-workspace parent session as creation provenance")
	bindNamedNetworkParticipationFlags(cmd, &networkFlags)
	cmd.MarkFlagsMutuallyExclusive("cwd", "worktree", "new-worktree")
	return cmd
}
