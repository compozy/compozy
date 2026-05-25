package cli

import (
	"fmt"

	"github.com/compozy/compozy/internal/update"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

func newUpgradeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "upgrade",
		Short:        "Upgrade compozy to the latest release",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Upgrade compozy using the appropriate installation flow for this machine.

Package-manager installs run the correct package manager command. Direct binary installs
perform an in-place self-update.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signalCommandContext(cmd)
			defer stop()

			if err := update.Upgrade(ctx, version.Version, cmd.OutOrStdout()); err != nil {
				return fmt.Errorf("upgrade compozy: %w", err)
			}
			return nil
		},
	}

	return cmd
}
