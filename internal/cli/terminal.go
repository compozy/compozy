package cli

import "github.com/spf13/cobra"

const (
	terminalCommandKey            = "terminal"
	terminalOpenCommandKey        = "open"
	terminalListCommandKey        = "list"
	terminalStreamModeRead        = "read"
	terminalStreamModeWrite       = "write"
	terminalStreamFlowAck         = "ack"
	terminalStreamFlowDrop        = "drop"
	terminalModeKey               = "mode"
	terminalClientHTTPProtocol    = "http"
	terminalStateRunning          = "running"
	terminalOutcomeAnswered       = "answered"
	terminalOutcomeRejected       = "rejected"
	terminalRequestIDKey          = "request_id"
	terminalNextKey               = "next"
	terminalRecordingStartAction  = "start"
	terminalCurrentWorkspaceLabel = "current"
)

func newTerminalCommand(deps commandDeps) *cobra.Command {
	command := &cobra.Command{
		Use: terminalCommandKey, Short: "Open and manage workspace terminals", Args: terminalArgs(cobra.NoArgs),
	}
	command.AddCommand(newTerminalOpenCommand(deps))
	command.AddCommand(newTerminalListCommand(deps))
	command.AddCommand(newTerminalGetCommand(deps))
	command.AddCommand(newTerminalAttachCommand(deps))
	command.AddCommand(newTerminalKillCommand(deps))
	command.AddCommand(newTerminalExecCommand(deps))
	command.AddCommand(newTerminalSignalCommand(deps))
	command.AddCommand(newTerminalInputRequestsCommand(deps))
	command.AddCommand(newTerminalRespondCommand(deps))
	command.AddCommand(newTerminalJournalCommand(deps))
	command.AddCommand(newTerminalRecordCommand(deps))
	command.AddCommand(newTerminalQuoteCommand(deps))
	return command
}

func terminalClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (TerminalClient, string, error) {
	baseClient, err := clientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	client, ok := baseClient.(TerminalClient)
	if !ok {
		return nil, "", newTerminalTransportError(
			terminalTransportCodeUnavailable, "terminal client is unavailable", nil,
		)
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(), cmd, deps, client, workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return nil, "", err
	}
	if agentIdentityEnvironmentPresent(deps) {
		sessionClient, ok := baseClient.(agentSessionClient)
		if !ok {
			return nil, "", newTerminalTransportError(
				terminalTransportCodeUnavailable, "terminal agent identity lookup is unavailable", nil,
			)
		}
		caller, resolveErr := resolveAgentCallerFromEnvForWorkspace(
			cmd.Context(),
			deps,
			sessionClient,
			cmd.CommandPath(),
			resolution.ID,
		)
		if resolveErr != nil {
			return nil, "", resolveErr
		}
		cmd.SetContext(withClientAgentCredentials(cmd.Context(), caller.Credentials))
	}
	return client, resolution.ID, nil
}

func terminalArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return terminalInvalidRequest(err.Error(), err)
		}
		return nil
	}
}

func addTerminalWorkspaceFlag(command *cobra.Command, target *string) {
	command.Flags().StringVar(target, workspaceFlagName, "", "Override workspace (ID, name, or path)")
}
