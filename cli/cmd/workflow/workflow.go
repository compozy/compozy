package workflow

import (
	"github.com/spf13/cobra"
)

// Cmd creates the workflow command group
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Workflow management commands",
		Long:  "Manage workflows: list, get details, and execute workflows.",
	}
	cmd.AddCommand(
		ListCmd(),
		GetCmd(),
		ExecuteCmd(),
	)
	return cmd
}
