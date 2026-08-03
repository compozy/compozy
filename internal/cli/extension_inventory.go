package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func newExtensionInventoryCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
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
}

func newExtensionPreviewCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "preview <name>",
		Short: "Preview enabling an extension without changing state",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := previewExtensionEnable(cmd.Context(), deps, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionEnablePreviewBundle(payload))
		},
	}
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

func previewExtensionEnable(
	ctx context.Context,
	deps commandDeps,
	name string,
) (ExtensionEnablePreviewRecord, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return ExtensionEnablePreviewRecord{}, err
	}
	return client.PreviewExtensionEnable(ctx, name)
}
