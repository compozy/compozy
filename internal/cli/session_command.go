package cli

import "github.com/spf13/cobra"

func newSessionCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   sessionSessionKey,
		Short: "Manage AGH sessions",
	}

	cmd.AddCommand(newSessionCreateCommand(deps))
	cmd.AddCommand(newSessionListCommand(deps))
	cmd.AddCommand(newSessionStopCommand(deps))
	cmd.AddCommand(newSessionRemoveCommand(deps))
	cmd.AddCommand(newSessionSoulCommand(deps))
	cmd.AddCommand(newSessionHealthCommand(deps))
	cmd.AddCommand(newSessionStatusCommand(deps))
	cmd.AddCommand(newSessionUsageCommand(deps))
	cmd.AddCommand(newSessionInspectCommand(deps))
	cmd.AddCommand(newSessionResumeCommand(deps))
	cmd.AddCommand(newSessionRecapCommand(deps))
	cmd.AddCommand(newSessionRepairCommand(deps))
	cmd.AddCommand(newSessionApproveCommand(deps))
	cmd.AddCommand(newSessionClarifyCommand(deps))
	cmd.AddCommand(newSessionWaitCommand(deps))
	cmd.AddCommand(newSessionPromptCommand(deps))
	cmd.AddCommand(newSessionEventsCommand(deps))
	cmd.AddCommand(newSessionHistoryCommand(deps))

	return cmd
}
