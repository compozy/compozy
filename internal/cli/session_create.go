package cli

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const sessionCreateExample = `  # Start a session in the current workspace using the configured default agent
  agh session new

  # Start a named session for a specific registered workspace and agent
  agh session new --workspace checkout-api --agent reviewer --name review-api

  # Override provider, model, and reasoning effort for this session only
  agh session new --provider codex --model gpt-5.6-sol --reasoning-effort max

  # Create the session and dispatch its first prompt after runtime activation
  agh session new --prompt "Inspect the failing build and propose a fix"

  # Auto-register an absolute workspace path before creating the session
  agh session new --cwd "$PWD" --agent reviewer`

func newSessionCreateCommand(deps commandDeps) *cobra.Command {
	var (
		agentName       string
		cwd             string
		name            string
		provider        string
		model           string
		prompt          string
		reasoningEffort string
		workspaceRef    string
		noWait          bool
		networkFlags    networkParticipationFlags
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

			workspace, workspacePath, err := resolveSessionCreateWorkspace(deps, workspaceRef, cwd)
			if err != nil {
				return err
			}
			participationRequest, err := networkFlags.namedRequest()
			if err != nil {
				return err
			}

			created, err := client.CreateSession(cmd.Context(), CreateSessionRequest{
				AgentName:            agentName,
				Provider:             strings.TrimSpace(provider),
				Model:                strings.TrimSpace(model),
				ReasoningEffort:      contract.ReasoningEffort(strings.TrimSpace(reasoningEffort)),
				Prompt:               strings.TrimSpace(prompt),
				Name:                 name,
				Workspace:            workspace,
				WorkspacePath:        workspacePath,
				NetworkParticipation: participationRequest,
			})
			if err != nil {
				return err
			}
			if !noWait {
				created, err = waitForSessionStartup(cmd.Context(), client, created, deps.pollInterval)
				if err != nil {
					return err
				}
			}

			return writeCommandOutput(cmd, sessionBundle(created, deps.now))
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "Agent definition name (defaults to config default)")
	cmd.Flags().StringVar(&workspaceRef, workspaceSkillSource, "", "Registered workspace name or ID")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Absolute workspace directory to auto-register")
	cmd.Flags().StringVar(&name, sessionNameKey, "", "Optional session label")
	bindNamedNetworkParticipationFlags(cmd, &networkFlags)
	cmd.Flags().StringVar(&provider, sessionProviderKey, "", "Optional provider override for this session")
	cmd.Flags().StringVar(&model, "model", "", "Optional model override for this session")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Initial prompt to dispatch after the session becomes active")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Return after the durable starting session is accepted")
	cmd.Flags().StringVar(
		&reasoningEffort,
		"reasoning-effort",
		"",
		"Optional reasoning effort (none|minimal|low|medium|high|xhigh|max); the active adapter must advertise it",
	)
	return cmd
}
