package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newDeviceCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "device", Short: "Manage paired gateway devices"}
	cmd.AddCommand(newDeviceListCommand(deps))
	cmd.AddCommand(newDeviceRenameCommand(deps))
	cmd.AddCommand(newDeviceRevokeCommand(deps))
	cmd.AddCommand(newDeviceRemoveCommand(deps))
	return cmd
}

func newDeviceListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   cliOutputListKey,
		Short: "List paired devices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			devices, err := client.ListGatewayDevices(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, gatewayDevicesOutput(devices))
		},
	}
}

func newDeviceRenameCommand(deps commandDeps) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "rename <id>",
		Short: "Rename a paired device",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			device, err := client.RenameGatewayDevice(cmd.Context(), strings.TrimSpace(args[0]), name)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				gatewayDeviceMutationOutput(device, device.ID, device.Name, "renamed"),
			)
		},
	}
	cmd.Flags().StringVar(&name, automationNameKey, "", "New device name")
	mustMarkFlagRequired(cmd, automationNameKey)
	return cmd
}

func newDeviceRevokeCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a paired device and close its live streams",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.RevokeGatewayDevice(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			status := gatewayDeviceStatusUnchanged
			if result.Changed {
				status = gatewayRevokedState
			}
			return writeCommandOutput(
				cmd,
				gatewayDeviceMutationOutput(result, result.Device.ID, result.Device.Name, status),
			)
		},
	}
}

func newDeviceRemoveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a paired device record, revoking it first when it is active",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := gatewayClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.RemoveGatewayDevice(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				gatewayDeviceMutationOutput(
					result,
					result.Device.ID,
					result.Device.Name,
					gatewayRemovedState,
				),
			)
		},
	}
}
