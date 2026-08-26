package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newMessageCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "message", Short: "Send and inspect agent messages"}
	cmd.AddCommand(newMessageSendCommand(deps), newMessageListCommand(deps))
	return cmd
}

func newMessageSendCommand(deps commandDeps) *cobra.Command {
	var workspace, callID string
	cmd := &cobra.Command{
		Use:   "send <session-id> <text>",
		Short: "Send inert text to a lineage session",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := resolveCallClient(cmd, deps, workspace)
			if err != nil {
				return err
			}
			response, err := client.SendCallMessage(cmd.Context(), workspaceID, contract.SendCallMessageRequest{
				To:   contract.CallTargetRequest{SessionID: strings.TrimSpace(args[0])},
				Text: args[1], CallID: strings.TrimSpace(callID),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, messageSendBundle(response))
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Override workspace name or id; omit for global scope")
	cmd.Flags().StringVar(&callID, "call", "", "Related call id")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func newMessageListCommand(deps commandDeps) *cobra.Command {
	var workspace, sessionID, cursor string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List lineage messages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit < 0 {
				return fmt.Errorf("cli: --limit must be zero or positive")
			}
			client, workspaceID, err := resolveCallClient(cmd, deps, workspace)
			if err != nil {
				return err
			}
			response, err := client.ListCallMessages(cmd.Context(), callListQuery{
				WorkspaceID: workspaceID, SessionID: sessionID, Cursor: cursor, Limit: limit,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, messageListBundle(response))
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Override workspace name or id; omit for global scope")
	cmd.Flags().StringVar(&sessionID, "session", "", "Only messages to or from this session")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Opaque page cursor")
	cmd.Flags().IntVar(&limit, "limit", 50, "Page size (maximum 200)")
	configureProfileReadCommand(cmd, deps)
	return cmd
}
