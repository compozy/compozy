package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type networkDirectsFlags struct {
	channel              string
	directID             string
	peerID               string
	query                string
	participantSessionID string
	sort                 string
	hasWork              bool
	session              string
	limit                int
	before               string
	after                string
	kind                 string
	workID               string
}

func newNetworkDirectsCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "directs",
		Short: "Inspect restricted direct rooms",
	}
	cmd.AddCommand(newNetworkDirectsListCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkDirectsResolveCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkDirectsShowCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkDirectsMessagesCommand(deps, workspaceRef))
	return cmd
}

func newNetworkDirectsResolveCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkDirectsFlags
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Create or return the deterministic direct room for two peers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			direct, err := client.NetworkDirectResolve(
				cmd.Context(),
				workspace,
				strings.TrimSpace(flags.channel),
				NetworkDirectResolveRequest{
					SessionID: strings.TrimSpace(flags.session),
					PeerID:    strings.TrimSpace(flags.peerID),
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkDirectBundle(direct))
		},
	}
	cmd.Flags().StringVar(&flags.session, "session", "", "Local source session id")
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(&flags.peerID, "peer", "", "Remote peer id")
	mustMarkFlagRequired(cmd, "session")
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, "peer")
	return cmd
}

func newNetworkDirectsShowCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkDirectsFlags
	cmd := &cobra.Command{
		Use:   networkShowKey,
		Short: "Show one direct room",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			direct, err := client.NetworkDirect(
				cmd.Context(),
				workspace,
				strings.TrimSpace(flags.channel),
				strings.TrimSpace(flags.directID),
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkDirectBundle(direct))
		},
	}
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(&flags.directID, networkSurfaceDirect, "", "Direct room id")
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, networkSurfaceDirect)
	return cmd
}

func newNetworkDirectsMessagesCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkDirectsFlags
	cmd := &cobra.Command{
		Use:   sessionMessagesKey,
		Short: "List messages in one direct room",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.NetworkDirectMessages(cmd.Context(), NetworkConversationMessagesQuery{
				WorkspaceRef: workspace,
				Channel:      strings.TrimSpace(flags.channel),
				DirectID:     strings.TrimSpace(flags.directID),
				Limit:        flags.limit,
				Before:       strings.TrimSpace(flags.before),
				After:        strings.TrimSpace(flags.after),
				Kind:         strings.TrimSpace(flags.kind),
				WorkID:       strings.TrimSpace(flags.workID),
			})
			if err != nil {
				return err
			}
			return writeCommandOutputWithJSONL(cmd, networkDirectMessagesBundle(response), response.Messages)
		},
	}
	registerNetworkMessageReadFlags(
		cmd,
		&flags.channel,
		networkSurfaceDirect,
		"Direct room id",
		&flags.directID,
		&flags.limit,
		&flags.before,
		&flags.after,
		&flags.kind,
		&flags.workID,
	)
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, networkSurfaceDirect)
	return cmd
}

func registerNetworkMessageReadFlags(
	cmd *cobra.Command,
	channel *string,
	containerFlagName string,
	containerUsage string,
	containerID *string,
	limit *int,
	before *string,
	after *string,
	kind *string,
	workID *string,
) {
	cmd.Flags().StringVar(channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(containerID, containerFlagName, "", containerUsage)
	cmd.Flags().IntVar(limit, "limit", 0, "Maximum number of messages to return")
	cmd.Flags().StringVar(before, "before", "", "Cursor before which to list messages")
	cmd.Flags().StringVar(after, "after", "", "Cursor after which to list messages")
	cmd.Flags().StringVar(kind, networkKindKey, "", "Envelope kind filter")
	cmd.Flags().StringVar(workID, "work", "", "Work id filter")
}
