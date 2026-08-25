package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

const (
	terminalCommandKey            = "terminal"
	terminalOpenCommandKey        = "open"
	terminalListCommandKey        = "list"
	terminalStreamModeRead        = "read"
	terminalStreamModeWrite       = "write"
	terminalStreamFlowAck         = "ack"
	terminalStreamFlowDrop        = "drop"
	terminalModeKey               = "mode"
	terminalForceKey              = "force"
	terminalClientHTTPProtocol    = "http"
	terminalControllerHumanKind   = "human"
	terminalControllerAvailable   = "available"
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
		Use: terminalCommandKey, Short: "Open and manage workspace terminals", Args: cobra.NoArgs,
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

func terminalClientFromDeps(deps commandDeps) (TerminalClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	terminalClient, ok := client.(TerminalClient)
	if !ok {
		return nil, errors.New("cli: terminal client is unavailable")
	}
	return terminalClient, nil
}

func terminalClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (TerminalClient, string, error) {
	client, err := terminalClientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(), cmd, deps, client, workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return nil, "", err
	}
	return client, resolution.ID, nil
}

func addTerminalWorkspaceFlag(command *cobra.Command, target *string) {
	command.Flags().StringVar(target, workspaceFlagName, "", "Override workspace (ID, name, or path)")
}
