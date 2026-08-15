package cli

import (
	"fmt"

	"github.com/compozy/compozy/extensions/connectivity/tailscale"
	forgegithub "github.com/compozy/compozy/extensions/forge/github"
	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
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
			case tailscale.Name:
				return tailscale.RunProvider(
					cmd.Context(),
					cmd.InOrStdin(),
					cmd.OutOrStdout(),
					cmd.ErrOrStderr(),
				)
			case forgegithub.Name:
				return forgegithub.RunProvider(
					cmd.Context(),
					cmd.InOrStdin(),
					cmd.OutOrStdout(),
					cmd.ErrOrStderr(),
				)
			case speccycle.Name:
				return speccycle.RunProvider(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
			default:
				return fmt.Errorf("unknown internal extension provider %q", args[0])
			}
		},
	}
	return cmd
}
