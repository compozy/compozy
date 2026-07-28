package cli

import "github.com/spf13/cobra"

func newAutomationCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "automation",
		Short: "Manage automation jobs, triggers, suggestions, and runs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAutomationJobsCommand(deps))
	cmd.AddCommand(newAutomationTriggersCommand(deps))
	cmd.AddCommand(newAutomationSuggestionsCommand(deps))
	cmd.AddCommand(newAutomationRunsCommand(deps))
	return cmd
}
