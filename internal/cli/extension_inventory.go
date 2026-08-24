package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newExtensionInventoryCommand(deps commandDeps) *cobra.Command {
	command := &cobra.Command{
		Use:   "inventory <name>",
		Short: "Show shipped and live resources for an extension",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := extensionInventory(cmd.Context(), deps, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionInventoryBundle(payload))
		},
	}
	configureSingleProfileReadCommand(command, deps)
	return command
}

func extensionInventory(
	ctx context.Context,
	deps commandDeps,
	name string,
) (ExtensionInventoryRecord, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return ExtensionInventoryRecord{}, err
	}
	return client.ExtensionInventory(ctx, name)
}
