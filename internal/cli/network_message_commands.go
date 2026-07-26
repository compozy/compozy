package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNetworkWorkCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work",
		Short: "Inspect lifecycle-bearing network work",
	}
	cmd.AddCommand(newNetworkWorkLookupCommand(deps, workspaceRef, "lookup"))
	cmd.AddCommand(newNetworkWorkLookupCommand(deps, workspaceRef, networkStatusKey))
	return cmd
}

func newNetworkWorkLookupCommand(deps commandDeps, workspaceRef *string, use string) *cobra.Command {
	var workID string
	cmd := &cobra.Command{
		Use:   use,
		Short: "Show one network work item",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			work, err := client.NetworkWork(cmd.Context(), workspace, strings.TrimSpace(workID))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkWorkBundle(work))
		},
	}
	cmd.Flags().StringVar(&workID, "work", "", "Network work id")
	mustMarkFlagRequired(cmd, "work")
	return cmd
}

type networkSendFlags struct {
	sessionID    string
	channel      string
	surface      string
	threadID     string
	directID     string
	kind         string
	to           string
	mentions     []string
	bodyRaw      string
	workID       string
	replyTo      string
	traceID      string
	causationID  string
	expiresAtRaw string
	id           string
	extRaw       string
}

func newNetworkSendCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var flags networkSendFlags
	cmd := &cobra.Command{
		Use:   networkSendKey,
		Short: "Send one envelope through the daemon-owned network runtime",
		Long: "Send one envelope through the daemon-owned network runtime.\n\n" +
			"For surface=thread, choose an ID matching `" + networkThreadIDPattern + "`. " +
			"The first valid send opens a new public thread; later sends reuse that ID. " +
			"For surface=direct, resolve the room first and reuse its direct_id.",
		Example: `  agh network send --workspace ws_example --session sess_example --channel launch-war-room \
    --surface thread --thread thread_launch_brief --kind say --body '{"text":"Launch status"}'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := parseNetworkJSONValue("--body", flags.bodyRaw)
			if err != nil {
				return err
			}
			ext, err := parseNetworkJSONObjectMap("--ext", flags.extRaw)
			if err != nil {
				return err
			}
			if err := validateNetworkSendNoRawClaimToken(body, ext); err != nil {
				return err
			}
			expiresAt, err := parseNetworkExpiresAt(flags.expiresAtRaw)
			if err != nil {
				return err
			}
			if err := validateNetworkSendFlags(flags); err != nil {
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

			message, err := client.NetworkSend(cmd.Context(), NetworkSendRequest{
				WorkspaceID: workspace,
				SessionID:   strings.TrimSpace(flags.sessionID),
				Channel:     strings.TrimSpace(flags.channel),
				Surface:     strings.TrimSpace(flags.surface),
				ThreadID:    strings.TrimSpace(flags.threadID),
				DirectID:    strings.TrimSpace(flags.directID),
				Kind:        strings.TrimSpace(flags.kind),
				To:          strings.TrimSpace(flags.to),
				Mentions:    trimSpawnAtoms(flags.mentions),
				Body:        body,
				WorkID:      strings.TrimSpace(flags.workID),
				ReplyTo:     strings.TrimSpace(flags.replyTo),
				TraceID:     strings.TrimSpace(flags.traceID),
				CausationID: strings.TrimSpace(flags.causationID),
				ExpiresAt:   expiresAt,
				ID:          strings.TrimSpace(flags.id),
				Ext:         ext,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkSendBundle(message))
		},
	}

	registerNetworkSendFlags(cmd, &flags)
	mustMarkFlagRequired(cmd, "session")
	mustMarkFlagRequired(cmd, networkChannelKey)
	mustMarkFlagRequired(cmd, networkKindKey)
	mustMarkFlagRequired(cmd, "body")
	return cmd
}

func registerNetworkSendFlags(cmd *cobra.Command, flags *networkSendFlags) {
	cmd.Flags().StringVar(&flags.sessionID, "session", "", "Local source session id")
	cmd.Flags().StringVar(&flags.channel, networkChannelKey, "", "Target channel")
	cmd.Flags().StringVar(
		&flags.surface,
		networkSurfaceKey,
		"",
		"Conversation surface: thread or direct; required for say/capability/receipt/trace",
	)
	cmd.Flags().StringVar(
		&flags.threadID,
		networkSurfaceThread,
		"",
		"Public thread id matching "+networkThreadIDPattern+"; the first valid send opens it",
	)
	cmd.Flags().StringVar(
		&flags.directID,
		networkSurfaceDirect,
		"",
		"Existing direct room id from network directs resolve",
	)
	cmd.Flags().StringVar(&flags.kind, networkKindKey, "", "Envelope kind")
	cmd.Flags().StringVar(
		&flags.to,
		"to",
		"",
		"Directed target peer id; required for capability and say with --work",
	)
	cmd.Flags().StringArrayVar(&flags.mentions, "mention", nil, "Mention target peer id (repeatable)")
	cmd.Flags().StringVar(&flags.bodyRaw, "body", "", "Kind-specific JSON object; say requires a non-empty text field")
	cmd.Flags().StringVar(
		&flags.workID,
		"work",
		"",
		"Required for capability, receipt, and trace; optional for lifecycle-bearing say",
	)
	cmd.Flags().StringVar(&flags.replyTo, "reply-to", "", "Optional reply-to message id")
	cmd.Flags().StringVar(&flags.traceID, "trace-id", "", "Optional trace id")
	cmd.Flags().StringVar(&flags.causationID, "causation-id", "", "Optional causation id")
	cmd.Flags().StringVar(&flags.expiresAtRaw, "expires-at", "", "Optional expiry as unix seconds or RFC3339")
	cmd.Flags().StringVar(&flags.id, "id", "", "Optional explicit message id")
	cmd.Flags().StringVar(&flags.extRaw, "ext", "", "Optional JSON object of extension metadata")
}

func newNetworkInboxCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "Show queued inbound messages for one session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}

			messages, err := client.NetworkInbox(cmd.Context(), workspace, strings.TrimSpace(sessionID))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkInboxBundle(messages))
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Target session id")
	mustMarkFlagRequired(cmd, "session")
	return cmd
}
