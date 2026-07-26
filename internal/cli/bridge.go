package cli

import "github.com/spf13/cobra"

const (
	taskThreadValue = "Thread"
)

const (
	taskPeerValue       = "Peer"
	bundlePlatformValue = "Platform"
)

const (
	taskGroupValue          = "Group"
	taskBridgeInstanceIDKey = "bridge_instance_id"
)

const (
	bridgeModeKey = "mode"
)

const (
	bridgeAgentValue        = "Agent"
	bridgeBridgeValue       = "Bridge"
	bridgeCreatedValue      = "Created"
	bridgeEnabledValue      = "Enabled"
	bridgeExtensionValue    = "Extension"
	bridgeMessageValue      = "Message"
	bridgeModeValue         = "Mode"
	bridgeAgentNameKey      = "agent_name"
	bridgeBindingNameKey    = "binding_name"
	bridgeBridgeKey         = "bridge"
	bridgeCreateKey         = "create"
	bridgeCreatedAtKey      = "created_at"
	bridgeDeletedKey        = "deleted"
	bridgeDisplayNameKey    = "display_name"
	bridgeEnabledKey        = "enabled"
	bridgeGetIDValue        = "get <id>"
	bridgeGroupIDKey        = "group_id"
	bridgeLastActivityAtKey = "last_activity_at"
	bridgeListKey           = "list"
	bridgeMessageKey        = "message"
	bridgePeerIDKey         = "peer_id"
	bridgePlatformKey       = "platform"
	bridgeResolvedValue     = "resolved"
	bridgeScopeKey          = "scope"
	bridgeStepValue         = "Step"
	bridgeSessionIDKey      = "session_id"
	bridgeStatusKey         = "status"
	bridgeThreadIDKey       = "thread_id"
	bridgeUnresolvedValue   = "unresolved"
	bridgeUpdateIDValue     = "update <id>"
	bridgeUpdatedAtKey      = "updated_at"
	bridgeWorkspaceIDKey    = "workspace_id"
)

func newBridgeCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   bridgeBridgeKey,
		Short: "Manage bridge instances",
	}

	registerBridgeCommands(cmd, deps)
	return cmd
}

func newBridgeGetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   bridgeGetIDValue,
		Short: "Show one bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			item, err := client.GetBridge(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeBundle(item))
		},
	}
}

func newBridgeEnableCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			item, err := client.EnableBridge(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeBundle(item))
		},
	}
}

func newBridgeDisableCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			item, err := client.DisableBridge(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeBundle(item))
		},
	}
}

func newBridgeRestartCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <id>",
		Short: "Restart a bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			item, err := client.RestartBridge(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeBundle(item))
		},
	}
}
