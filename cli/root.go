package cli

import (
	"github.com/compozy/compozy/cli/auth"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "compozy",
		Short: "Compozy CLI tool",
	}

	root.AddCommand(
		DevCmd(),
		MCPProxyCmd(),
		ConfigCmd(),
		auth.Cmd(),
	)

	return root
}
