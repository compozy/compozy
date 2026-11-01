package cli

import (
	"fmt"

	"github.com/compozy/compozy/extensions/connectivity/tailscale"
	forgegithub "github.com/compozy/compozy/extensions/forge/github"
	opendesign "github.com/compozy/compozy/extensions/open-design"
	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	"github.com/compozy/compozy/internal/shellwait"
	"github.com/spf13/cobra"
)

func newInternalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "__internal",
		Hidden: true,
	}
	cmd.AddCommand(newInternalExtensionProviderCommand(), newInternalShellWaitHookCommand())
	return cmd
}

// newInternalShellWaitHookCommand answers a provider PreToolUse hook. It always
// exits zero: hooks fail open, and the deny decision travels on stdout.
func newInternalShellWaitHookCommand() *cobra.Command {
	return &cobra.Command{
		Use:    shellwait.HookCommandName,
		Args:   cobra.NoArgs,
		Hidden: true,
		Run: func(cmd *cobra.Command, _ []string) {
			shellwait.RunHook(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
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
			case opendesign.Name:
				return opendesign.RunProvider(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			case speccycle.Name:
				return speccycle.RunProvider(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
			default:
				return fmt.Errorf("unknown internal extension provider %q", args[0])
			}
		},
	}
	return cmd
}
