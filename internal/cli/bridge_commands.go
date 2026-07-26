package cli

import "github.com/spf13/cobra"

func registerBridgeCommands(cmd *cobra.Command, deps commandDeps) {
	cmd.AddCommand(newBridgeListCommand(deps))
	cmd.AddCommand(newBridgeGetCommand(deps))
	cmd.AddCommand(newBridgeCreateCommand(deps))
	cmd.AddCommand(newBridgeUpdateCommand(deps))
	cmd.AddCommand(newBridgeEnableCommand(deps))
	cmd.AddCommand(newBridgeDisableCommand(deps))
	cmd.AddCommand(newBridgeRestartCommand(deps))
	cmd.AddCommand(newBridgeRoutesCommand(deps))
	cmd.AddCommand(newBridgeTargetsCommand(deps))
	cmd.AddCommand(newBridgeResolveCommand(deps))
	cmd.AddCommand(newBridgeSecretBindingsCommand(deps))
	cmd.AddCommand(newBridgeTestDeliveryCommand(deps))
	cmd.AddCommand(newBridgeManifestCommand(deps))
	cmd.AddCommand(newBridgeSetupCommand(deps))
	cmd.AddCommand(newBridgeVerifyCommand(deps))
	cmd.AddCommand(newBridgeSendTestCommand(deps))
}
