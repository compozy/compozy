package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type networkSubscriptionFlags struct {
	channel   string
	threadID  string
	sessionID string
	limit     int
}

func newNetworkSubscriptionsCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "Inspect network delivery preferences",
	}
	cmd.AddCommand(newNetworkSubscriptionsListCommand(deps, workspaceRef))
	return cmd
}

func newNetworkSubscriptionsListCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkSubscriptionFlags
	cmd := &cobra.Command{
		Use:   networkListKey,
		Short: "List network delivery preferences",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			subscriptions, err := client.ListNetworkSubscriptions(cmd.Context(), NetworkSubscriptionsQuery{
				WorkspaceRef: workspace,
				Channel:      strings.TrimSpace(flags.channel),
				ThreadID:     strings.TrimSpace(flags.threadID),
				SessionID:    strings.TrimSpace(flags.sessionID),
				Limit:        flags.limit,
			})
			if err != nil {
				return err
			}
			return writeCommandOutputWithJSONL(cmd, networkSubscriptionsBundle(subscriptions), subscriptions)
		},
	}
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(&flags.threadID, networkSurfaceThread, "", "Optional thread id")
	cmd.Flags().StringVar(&flags.sessionID, "session", "", "Optional session id filter")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Maximum number of preferences to return")
	mustMarkFlagRequired(cmd, networkChannelKey)
	return cmd
}

func newNetworkSubscriptionModeCommand(
	deps commandDeps,
	workspaceRef *string,
	use string,
	mode string,
	short string,
) *cobra.Command {
	var flags networkSubscriptionFlags
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			subscription, err := client.SetNetworkSubscription(
				cmd.Context(),
				workspace,
				strings.TrimSpace(flags.channel),
				NetworkSubscriptionRequest{
					ThreadID:  strings.TrimSpace(flags.threadID),
					SessionID: strings.TrimSpace(flags.sessionID),
					Mode:      mode,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkSubscriptionBundle(subscription))
		},
	}
	registerNetworkSubscriptionMutationFlags(cmd, &flags)
	return cmd
}

func newNetworkUnmuteCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkSubscriptionFlags
	cmd := &cobra.Command{
		Use:   "unmute",
		Short: "Remove one network delivery preference",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			if err := client.DeleteNetworkSubscription(
				cmd.Context(),
				workspace,
				strings.TrimSpace(flags.channel),
				strings.TrimSpace(flags.sessionID),
				strings.TrimSpace(flags.threadID),
			); err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				networkSubscriptionDeleteBundle(flags.channel, flags.threadID, flags.sessionID),
			)
		},
	}
	registerNetworkSubscriptionMutationFlags(cmd, &flags)
	return cmd
}

func registerNetworkSubscriptionMutationFlags(
	cmd *cobra.Command,
	flags *networkSubscriptionFlags,
) {
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(&flags.threadID, networkSurfaceThread, "", "Optional thread id")
	cmd.Flags().StringVar(&flags.sessionID, "session", "", "Target session id")
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, "session")
}
