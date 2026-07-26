package cli

import (
	"errors"

	"strings"

	"github.com/spf13/cobra"
)

const (
	toolOperatorPreviewValue = "Preview"
	networkLocalKey          = "local"
)

const (
	networkOpenedByValue = "Opened By"
)

const (
	networkOpenWorkValue = "Open Work"
	networkSentKey       = "sent"
)

const (
	networkChannelValue          = "Channel"
	networkCreatedByValue        = "Created By"
	networkDirectIDValue         = "Direct ID"
	networkEnabledValue          = "Enabled"
	networkFromValue             = "From"
	networkKindValue             = "Kind"
	networkLastActivityValue     = "Last Activity"
	networkMessageValue          = "Message"
	networkMessagesValue         = "Messages"
	networkOpenedAtValue         = "Opened At"
	networkPresenceValue         = "Presence"
	networkStateValue            = "State"
	networkStatusValue           = "Status"
	networkSurfaceValue          = "Surface"
	networkThreadIDValue         = "Thread ID"
	networkTimestampValue        = "Timestamp"
	networkTitleValue            = "Title"
	networkWorkspaceValue        = "Workspace"
	networkChannelKey            = "channel"
	networkChannelsKey           = "channels"
	networkDirectIDKey           = "direct_id"
	networkEnabledKey            = "enabled"
	networkKindKey               = "kind"
	networkLastActivityAtKey     = "last_activity_at"
	networkLastMessagePreviewKey = "last_message_preview"
	networkListKey               = "list"
	networkMessageCountKey       = "message_count"
	networkMessageIDKey          = "message_id"
	networkNetworkKey            = "network"
	networkOpenWorkCountKey      = "open_work_count"
	networkOpenedAtKey           = "opened_at"
	networkOpenedByPeerIDKey     = "opened_by_peer_id"
	networkPeerIDKey             = bridgePeerIDKey
	networkPromoteCommandUse     = "promote"
	networkSendKey               = "send"
	networkShowKey               = "show"
	networkStatusKey             = "status"
	networkSurfaceKey            = "surface"
	networkThreadIDKey           = "thread_id"
	networkTitleKey              = "title"
	networkWorkIDKey             = "work_id"
)

const (
	networkDeletedKey   = bridgeDeletedKey
	networkDeletedValue = toolBoolTrue
	networkModeKey      = bridgeModeKey
	networkModeValue    = bridgeModeValue
)

const (
	networkSurfaceThread  = "thread"
	networkSurfaceDirect  = "direct"
	networkKindSay        = "say"
	networkKindCapability = "capability"
	networkKindReceipt    = "receipt"
	networkKindTrace      = "trace"
	networkKindGreet      = "greet"
	networkKindWhois      = "whois"
)

func newNetworkCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use:   networkNetworkKey,
		Short: "Operate the daemon-owned network runtime",
	}

	cmd.AddCommand(newNetworkStatusCommand(deps))
	cmd.PersistentFlags().
		StringVar(&workspaceRef, "workspace", "", "Workspace root, name, or ID for scoped network data")
	cmd.AddCommand(newNetworkPeersCommand(deps, &workspaceRef))
	cmd.AddCommand(newNetworkChannelsCommand(deps, &workspaceRef))
	cmd.AddCommand(newNetworkThreadsCommand(deps, &workspaceRef))
	cmd.AddCommand(newNetworkDirectsCommand(deps, &workspaceRef))
	cmd.AddCommand(newNetworkWorkCommand(deps, &workspaceRef))
	cmd.AddCommand(newNetworkSendCommand(deps, &workspaceRef))
	cmd.AddCommand(newNetworkInboxCommand(deps, &workspaceRef))
	cmd.AddCommand(newNetworkSubscriptionsCommand(deps, &workspaceRef))
	cmd.AddCommand(
		newNetworkSubscriptionModeCommand(
			deps,
			&workspaceRef,
			"subscribe",
			"full",
			"Subscribe one peer to full delivery",
		),
	)
	cmd.AddCommand(
		newNetworkSubscriptionModeCommand(deps, &workspaceRef, "mute", "mute", "Mute one peer's network delivery"),
	)
	cmd.AddCommand(newNetworkUnmuteCommand(deps, &workspaceRef))
	registerNetworkPublicSurfaceCommands(cmd, deps, &workspaceRef)
	return cmd
}

func resolveNetworkWorkspaceRef(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	workspaceRef *string,
) (string, error) {
	raw := ""
	if workspaceRef != nil {
		raw = strings.TrimSpace(*workspaceRef)
	}
	return resolveCLIWorkspaceRouteRef(cmd.Context(), deps, client, raw)
}

func newNetworkStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   networkStatusKey,
		Short: "Show Network availability, Live participation, and runtime metrics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			status, err := client.NetworkStatus(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkStatusBundle(status))
		},
	}
}

func newNetworkPeersCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	return &cobra.Command{
		Use:   "peers [channel]",
		Short: "List visible local peers and their capability cards",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}

			query := NetworkPeersQuery{WorkspaceRef: workspace}
			if len(args) == 1 {
				query.Channel = strings.TrimSpace(args[0])
			}

			peers, err := client.NetworkPeers(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkPeersBundle(peers))
		},
	}
}

func newNetworkChannelsCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   networkChannelsKey,
		Short: "List active runtime channels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}

			channels, err := client.NetworkChannels(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkChannelsBundle(channels))
		},
	}
	cmd.AddCommand(newNetworkChannelsCreateCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkChannelsUpdateCommand(deps, workspaceRef))
	return cmd
}

type networkCreateChannelFlags struct {
	channel           string
	purpose           string
	fanoutPolicy      string
	coordinatorPeerID string
	agentNames        []string
}

func newNetworkChannelsCreateCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkCreateChannelFlags
	cmd := &cobra.Command{
		Use:   "create [channel]",
		Short: "Create a runtime channel and start one session per selected agent",
		Args:  cobra.MaximumNArgs(1),
		Example: `  # Create a launch channel with two local agents
  agh network --workspace ~/dev/ad8 channels create ad8_launch \
    --purpose "Coordinate launch work" \
    --agent site_copywriter \
    --agent growth_marketer \
    -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			channel, err := resolveNetworkCreateChannelName(args, flags.channel)
			if err != nil {
				return err
			}
			agentNames := trimSpawnAtoms(flags.agentNames)
			if len(agentNames) == 0 {
				return errors.New("cli: at least one --agent is required")
			}
			purpose := strings.TrimSpace(flags.purpose)
			if purpose == "" {
				return errors.New("cli: --purpose cannot be empty")
			}
			created, err := client.CreateNetworkChannel(cmd.Context(), workspace, CreateNetworkChannelRequest{
				Channel:           channel,
				Purpose:           purpose,
				FanoutPolicy:      strings.TrimSpace(flags.fanoutPolicy),
				CoordinatorPeerID: strings.TrimSpace(flags.coordinatorPeerID),
				AgentNames:        agentNames,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkChannelBundle(created))
		},
	}
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Channel name when not passed as an argument")
	cmd.Flags().StringVar(&flags.purpose, "purpose", "", "Human-readable channel purpose")
	cmd.Flags().StringVar(
		&flags.fanoutPolicy,
		"fanout-policy",
		"",
		"Channel routing metadata: capability_match, coordinator, or all_members; never enrolls or wakes executions",
	)
	cmd.Flags().StringVar(
		&flags.coordinatorPeerID,
		"coordinator-peer-id",
		"",
		"Coordinator peer id for coordinator routing metadata",
	)
	cmd.Flags().StringArrayVar(
		&flags.agentNames,
		"agent",
		nil,
		"Agent definition name to launch in the channel (repeatable)",
	)
	mustMarkFlagRequired(cmd, "purpose")
	mustMarkFlagRequired(cmd, "agent")
	return cmd
}

type networkUpdateChannelFlags struct {
	channel           string
	purpose           string
	fanoutPolicy      string
	coordinatorPeerID string
}

func newNetworkChannelsUpdateCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkUpdateChannelFlags
	cmd := &cobra.Command{
		Use:   "update [channel]",
		Short: "Update runtime channel routing metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			channel, err := resolveNetworkCreateChannelName(args, flags.channel)
			if err != nil {
				return err
			}
			request, err := networkChannelUpdateRequest(cmd, flags)
			if err != nil {
				return err
			}
			updated, err := client.UpdateNetworkChannel(cmd.Context(), workspace, channel, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkChannelBundle(updated))
		},
	}
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Channel name when not passed as an argument")
	cmd.Flags().StringVar(&flags.purpose, "purpose", "", "Human-readable channel purpose")
	cmd.Flags().StringVar(
		&flags.fanoutPolicy,
		"fanout-policy",
		"",
		"Channel routing metadata: capability_match, coordinator, or all_members; never enrolls or wakes executions",
	)
	cmd.Flags().StringVar(
		&flags.coordinatorPeerID,
		"coordinator-peer-id",
		"",
		"Coordinator peer id; pass empty to clear",
	)
	return cmd
}

func networkChannelUpdateRequest(
	cmd *cobra.Command,
	flags networkUpdateChannelFlags,
) (UpdateNetworkChannelRequest, error) {
	request := UpdateNetworkChannelRequest{}
	if cmd.Flags().Changed("purpose") {
		value := strings.TrimSpace(flags.purpose)
		request.Purpose = &value
	}
	if cmd.Flags().Changed("fanout-policy") {
		value := strings.TrimSpace(flags.fanoutPolicy)
		request.FanoutPolicy = &value
	}
	if cmd.Flags().Changed("coordinator-peer-id") {
		value := strings.TrimSpace(flags.coordinatorPeerID)
		request.CoordinatorPeerID = &value
	}
	if request.Purpose == nil && request.FanoutPolicy == nil && request.CoordinatorPeerID == nil {
		return UpdateNetworkChannelRequest{}, errors.New(
			"cli: at least one of --purpose, --fanout-policy, or --coordinator-peer-id is required",
		)
	}
	return request, nil
}

func resolveNetworkCreateChannelName(args []string, channelFlag string) (string, error) {
	fromArg := ""
	if len(args) == 1 {
		fromArg = strings.TrimSpace(args[0])
	}
	fromFlag := strings.TrimSpace(channelFlag)
	switch {
	case fromArg == "" && fromFlag == "":
		return "", errors.New("cli: channel is required")
	case fromArg != "" && fromFlag != "" && fromArg != fromFlag:
		return "", errors.New("cli: channel argument and --channel must match")
	case fromArg != "":
		return fromArg, nil
	default:
		return fromFlag, nil
	}
}
