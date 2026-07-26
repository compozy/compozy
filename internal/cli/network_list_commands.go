package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNetworkThreadsListCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkThreadsFlags
	cmd := &cobra.Command{
		Use:   networkListKey,
		Short: "List public threads in a channel",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.NetworkThreads(cmd.Context(), NetworkThreadsQuery{
				WorkspaceRef: workspace,
				Channel:      strings.TrimSpace(flags.channel),
				Query:        strings.TrimSpace(flags.query),
				SessionID:    strings.TrimSpace(flags.participantSessionID),
				Sort:         strings.TrimSpace(flags.sort),
				HasWork:      optionalNetworkHasWork(cmd, flags.hasWork),
				Limit:        flags.limit,
				After:        strings.TrimSpace(flags.after),
			})
			if err != nil {
				return err
			}
			return writeCommandOutputWithJSONL(cmd, networkThreadsBundle(response), response.Threads)
		},
	}
	registerNetworkConversationListFlags(
		cmd,
		&flags.channel,
		&flags.query,
		&flags.participantSessionID,
		&flags.sort,
		&flags.hasWork,
		&flags.limit,
		&flags.after,
		"threads",
	)
	return cmd
}

func newNetworkDirectsListCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkDirectsFlags
	cmd := &cobra.Command{
		Use:   networkListKey,
		Short: "List direct rooms in a channel",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.NetworkDirects(cmd.Context(), NetworkDirectsQuery{
				WorkspaceRef: workspace,
				Channel:      strings.TrimSpace(flags.channel),
				Query:        strings.TrimSpace(flags.query),
				SessionID:    strings.TrimSpace(flags.participantSessionID),
				Sort:         strings.TrimSpace(flags.sort),
				HasWork:      optionalNetworkHasWork(cmd, flags.hasWork),
				Limit:        flags.limit,
				After:        strings.TrimSpace(flags.after),
			})
			if err != nil {
				return err
			}
			return writeCommandOutputWithJSONL(cmd, networkDirectsBundle(response), response.Directs)
		},
	}
	registerNetworkConversationListFlags(
		cmd,
		&flags.channel,
		&flags.query,
		&flags.participantSessionID,
		&flags.sort,
		&flags.hasWork,
		&flags.limit,
		&flags.after,
		"direct rooms",
	)
	return cmd
}

func registerNetworkConversationListFlags(
	cmd *cobra.Command,
	channel *string,
	search *string,
	sessionID *string,
	sortOrder *string,
	hasWork *bool,
	limit *int,
	after *string,
	label string,
) {
	cmd.Flags().StringVar(channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(search, "query", "", "Filter by title, peer, or preview text")
	cmd.Flags().StringVar(sessionID, "session", "", "Participant session id filter")
	cmd.Flags().StringVar(sortOrder, "sort", "", "Order by recent_activity, created, or alphabetical")
	cmd.Flags().BoolVar(hasWork, "has-work", false, "Filter by whether open work exists")
	cmd.Flags().IntVar(limit, "limit", 0, "Maximum number of "+label+" to return")
	cmd.Flags().StringVar(after, "after", "", "Stable id cursor after which to continue")
	mustMarkFlagRequired(cmd, networkChannelKey)
}

func optionalNetworkHasWork(cmd *cobra.Command, value bool) *bool {
	if !cmd.Flags().Changed("has-work") {
		return nil
	}
	return &value
}
