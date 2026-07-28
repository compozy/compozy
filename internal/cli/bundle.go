package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

const (
	bundleModelKey = "model"
)

const (
	bundleAgentValue       = "Agent"
	bundleBundleValue      = "Bundle"
	bundleCreatedValue     = "Created"
	bundleDescriptionValue = "Description"
	bundleEventValue       = "Event"
	bundleKindValue        = "Kind"
	bundleModelValue       = "Model"
	bundleNameValue        = "Name"
	bundleProfileValue     = "Profile"
	bundleUpdatedValue     = "Updated"
	bundleBundleKey        = "bundle"
	bundleExtensionKey     = "extension"
	bundleGlobalKey        = "global"
	bundleHeartbeatKey     = "heartbeat"
	bundleKindKey          = "kind"
	bundleListKey          = "list"
	bundleProfileKey       = "profile"
)

func newBundleCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   bundleBundleKey,
		Short: "Manage extension bundle presets",
	}
	cmd.AddCommand(newBundleCatalogCommand(deps))
	cmd.AddCommand(newBundlePreviewCommand(deps))
	cmd.AddCommand(newBundleActivateCommand(deps))
	cmd.AddCommand(newBundleListCommand(deps))
	cmd.AddCommand(newBundleGetCommand(deps))
	cmd.AddCommand(newBundleUpdateCommand(deps))
	cmd.AddCommand(newBundleDeactivateCommand(deps))
	cmd.AddCommand(newBundleNetworkSettingsCommand(deps))
	return cmd
}

func newBundleCatalogCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "List available extension bundle presets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			items, err := client.ListBundleCatalog(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bundleCatalogBundle(items))
		},
	}
}

func newBundlePreviewCommand(deps commandDeps) *cobra.Command {
	flags := bundleActivationFlags{scope: bundleGlobalKey}
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview a bundle activation without writing resources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request := bundleActivationRequestFromFlags(flags)
			item, err := client.PreviewBundleActivation(cmd.Context(), request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bundleActivationBundle(item))
		},
	}
	addBundleActivationFlags(cmd, &flags)
	return cmd
}

func newBundleActivateCommand(deps commandDeps) *cobra.Command {
	flags := bundleActivationFlags{scope: bundleGlobalKey}
	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Activate a bundle preset",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request := bundleActivationRequestFromFlags(flags)
			item, err := client.ActivateBundle(cmd.Context(), request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bundleActivationBundle(item))
		},
	}
	addBundleActivationFlags(cmd, &flags)
	return cmd
}

func newBundleListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   bundleListKey,
		Short: "List active bundle presets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			items, err := client.ListBundleActivations(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bundleActivationListBundle(items))
		},
	}
}

func newBundleGetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "get <activation-id>",
		Short: "Show one bundle activation",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			item, err := client.GetBundleActivation(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bundleActivationBundle(item))
		},
	}
}

func newBundleDeactivateCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "deactivate <activation-id>",
		Short: "Deactivate a bundle preset and remove owned resources",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.DeactivateBundle(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{
				jsonValue: map[string]string{"deactivated": strings.TrimSpace(args[0])},
				human: func() (string, error) {
					return renderHumanSection("Bundle Deactivated", []keyValue{
						{Label: "Activation", Value: strings.TrimSpace(args[0])},
					}), nil
				},
				toon: func() (string, error) {
					return renderToonObject(
						"bundle_deactivated",
						[]string{"activation_id"},
						[]string{args[0]},
					), nil
				},
			})
		},
	}
}

func newBundleNetworkSettingsCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "network-settings",
		Short: "Show bundle-derived network settings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			settings, err := client.BundleNetworkSettings(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bundleNetworkSettingsBundle(settings))
		},
	}
}
