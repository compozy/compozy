package cli

import "github.com/spf13/cobra"

func newAutomationCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   automationRootKey,
		Short: "Manage automation jobs, triggers, suggestions, and runs",
	}

	cmd.AddCommand(newAutomationJobsCommand(deps))
	cmd.AddCommand(newAutomationTriggersCommand(deps))
	cmd.AddCommand(newAutomationSuggestionsCommand(deps))
	cmd.AddCommand(newAutomationRunsCommand(deps))
	return cmd
}
