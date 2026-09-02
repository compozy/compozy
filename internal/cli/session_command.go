package cli

import "github.com/spf13/cobra"

func newSessionCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   sessionSessionKey,
		Short: "Manage CompozyOS sessions",
	}

	cmd.AddCommand(newSessionCreateCommand(deps))
	cmd.AddCommand(newSessionListCommand(deps))
	cmd.AddCommand(newSessionInteractionsCommand(deps))
	cmd.AddCommand(newSessionStopCommand(deps))
	cmd.AddCommand(newSessionRenameCommand(deps))
	cmd.AddCommand(newSessionArchiveCommand(deps))
	cmd.AddCommand(newSessionUnarchiveCommand(deps))
	cmd.AddCommand(newSessionRemoveCommand(deps))
	cmd.AddCommand(newSessionSoulCommand(deps))
	cmd.AddCommand(newSessionHealthCommand(deps))
	cmd.AddCommand(newSessionStatusCommand(deps))
	cmd.AddCommand(newSessionCommandsCommand(deps))
	cmd.AddCommand(newSessionUsageCommand(deps))
	cmd.AddCommand(newSessionInspectCommand(deps))
	cmd.AddCommand(newSessionResumeCommand(deps))
	cmd.AddCommand(newSessionRecapCommand(deps))
	cmd.AddCommand(newSessionRepairCommand(deps))
	cmd.AddCommand(newSessionRewindCommand(deps))
	cmd.AddCommand(newSessionApproveCommand(deps))
	cmd.AddCommand(newSessionClarifyCommand(deps))
	cmd.AddCommand(newSessionWaitCommand(deps))
	cmd.AddCommand(newSessionPromptCancelCommand(deps))
	cmd.AddCommand(newSessionPromptCommand(deps))
	cmd.AddCommand(newSessionInputCommand(deps))
	cmd.AddCommand(newSessionAttachmentsCommand(deps))
	cmd.AddCommand(newSessionEventsCommand(deps))
	cmd.AddCommand(newSessionHistoryCommand(deps))
	cmd.AddCommand(newSessionRuntimeCommand(deps))
	cmd.AddCommand(newSessionGoalCommand(deps))
	configureSessionProfileCommands(cmd, deps)

	return cmd
}

func configureSessionProfileCommands(cmd *cobra.Command, deps commandDeps) {
	for _, child := range cmd.Commands() {
		if child.HasSubCommands() {
			configureSessionProfileCommands(child, deps)
		}
		if child.RunE == nil || child.Flags().Lookup(allProfilesFlagName) != nil {
			continue
		}
		configureSingleProfileCommand(child, deps)
	}
}
