package cli

import (
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "compozy",
		Short: "Compozy CLI tool",
	}

	root.AddCommand(
		InitCmd(),
		BuildCmd(),
		DeployCmd(),
		DevCmd(),
	)

	return root
}
