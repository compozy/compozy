package compozy

import (
	"github.com/compozy/compozy/internal/cli"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "compozy",
		Short: "Compozy CLI tool",
	}

	// Add all cli
	root.AddCommand(
		cli.InitCmd(),
		cli.BuildCmd(),
		cli.DeployCmd(),
		cli.DevCmd(),
	)

	return root
}
