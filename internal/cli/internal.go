package cli

import (
	"fmt"

	connectivitytailscale "github.com/compozy/compozy/extensions/connectivity-tailscale"
	devcycle "github.com/compozy/compozy/extensions/dev-cycle"
	"github.com/spf13/cobra"
)

func newInternalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "__internal",
		Hidden: true,
	}
	cmd.AddCommand(newInternalExtensionProviderCommand())
	return cmd
}

func newInternalExtensionProviderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "extension-provider <name>",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case connectivitytailscale.Name:
				return connectivitytailscale.RunProvider(
					cmd.Context(),
					cmd.InOrStdin(),
					cmd.OutOrStdout(),
					cmd.ErrOrStderr(),
				)
			case devcycle.Name:
				return devcycle.RunProvider(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
			default:
				return fmt.Errorf("unknown internal extension provider %q", args[0])
			}
		},
	}
	return cmd
}
