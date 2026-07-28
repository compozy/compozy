package cli

import (
	"encoding/json"

	"strings"

	"github.com/spf13/cobra"
)

type networkThreadsFlags struct {
	channel              string
	threadID             string
	query                string
	participantSessionID string
	sort                 string
	hasWork              bool
	limit                int
	before               string
	after                string
	kind                 string
	workID               string
}

func newNetworkThreadsCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "threads",
		Short: "Inspect public network threads",
	}
	cmd.AddCommand(newNetworkThreadsListCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkThreadsShowCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkThreadsMessagesCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkThreadsPromoteCommand(deps, workspaceRef))
	return cmd
}

func newNetworkThreadsShowCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkThreadsFlags
	cmd := &cobra.Command{
		Use:   networkShowKey,
		Short: "Show one public thread",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			thread, err := client.NetworkThread(
				cmd.Context(),
				workspace,
				strings.TrimSpace(flags.channel),
				strings.TrimSpace(flags.threadID),
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkThreadBundle(thread))
		},
	}
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(&flags.threadID, networkSurfaceThread, "", "Public thread id")
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, networkSurfaceThread)
	return cmd
}

func newNetworkThreadsMessagesCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkThreadsFlags
	cmd := &cobra.Command{
		Use:   sessionMessagesKey,
		Short: "List messages in one public thread",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.NetworkThreadMessages(cmd.Context(), NetworkConversationMessagesQuery{
				WorkspaceRef: workspace,
				Channel:      strings.TrimSpace(flags.channel),
				ThreadID:     strings.TrimSpace(flags.threadID),
				Limit:        flags.limit,
				Before:       strings.TrimSpace(flags.before),
				After:        strings.TrimSpace(flags.after),
				Kind:         strings.TrimSpace(flags.kind),
				WorkID:       strings.TrimSpace(flags.workID),
			})
			if err != nil {
				return err
			}
			return writeCommandOutputWithJSONL(cmd, networkThreadMessagesBundle(response), response.Messages)
		},
	}
	registerNetworkMessageReadFlags(
		cmd,
		&flags.channel,
		networkSurfaceThread,
		"Public thread id",
		&flags.threadID,
		&flags.limit,
		&flags.before,
		&flags.after,
		&flags.kind,
		&flags.workID,
	)
	cmd.Flags().Lookup(networkSurfaceThread).Usage = "Public thread id"
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, networkSurfaceThread)
	return cmd
}

type networkThreadPromoteFlags struct {
	channel         string
	threadID        string
	originMessageID string
	title           string
	description     string
	priority        string
	metadataRaw     string
}

func newNetworkThreadsPromoteCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkThreadPromoteFlags
	cmd := &cobra.Command{
		Use:   networkPromoteCommandUse,
		Short: "Promote one thread message into a durable task",
		RunE: func(cmd *cobra.Command, _ []string) error {
			metadata, err := parseOptionalNetworkMetadata(cmd, flags.metadataRaw)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			promoted, err := client.PromoteNetworkThreadTask(
				cmd.Context(),
				workspace,
				strings.TrimSpace(flags.channel),
				strings.TrimSpace(flags.threadID),
				PromoteNetworkThreadTaskRequest{
					OriginMessageID: strings.TrimSpace(flags.originMessageID),
					Title:           strings.TrimSpace(flags.title),
					Description:     strings.TrimSpace(flags.description),
					Priority:        strings.TrimSpace(flags.priority),
					Metadata:        metadata,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkThreadPromotionBundle(&promoted))
		},
	}
	registerNetworkThreadPromoteFlags(cmd, &flags)
	return cmd
}

func registerNetworkThreadPromoteFlags(cmd *cobra.Command, flags *networkThreadPromoteFlags) {
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(&flags.threadID, networkSurfaceThread, "", "Public thread id")
	cmd.Flags().StringVar(&flags.originMessageID, "origin-message", "", "Origin message id to promote")
	cmd.Flags().StringVar(&flags.title, "title", "", "Optional task title")
	cmd.Flags().StringVar(&flags.description, "description", "", "Optional task description")
	cmd.Flags().StringVar(&flags.priority, "priority", "", "Optional task priority")
	cmd.Flags().StringVar(&flags.metadataRaw, "metadata", "", "Optional JSON metadata for the promoted task")
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, networkSurfaceThread)
	mustMarkFlagRequired(cmd, "origin-message")
}

func parseOptionalNetworkMetadata(cmd *cobra.Command, raw string) (json.RawMessage, error) {
	if !cmd.Flags().Changed("metadata") {
		return nil, nil
	}
	return parseJSONFlag("metadata", raw)
}
