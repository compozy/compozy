package cli

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newSessionInteractionsCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:     "interactions <session-id>",
		Short:   "List pending session questions and permission requests",
		Args:    exactOneNonBlankArg(),
		Example: "  compozy session interactions sess_123 -o json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ListSessionInteractions(
				cmd.Context(),
				strings.TrimSpace(args[0]),
				nil,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionInteractionsBundle(response))
		},
	}
}

func newSessionPendingCommand(deps commandDeps) *cobra.Command {
	var descendants bool
	cmd := &cobra.Command{
		Use: "pending <session-id>", Short: "List pending session interactions", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ListSessionPendingInteractions(
				cmd.Context(),
				strings.TrimSpace(args[0]),
				descendants,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionInteractionsBundle(response))
		},
	}
	cmd.Flags().BoolVar(&descendants, "descendants", false, "Include descendant sessions")
	return cmd
}

func sessionInteractionsBundle(response SessionInteractionsRecord) outputBundle {
	return listBundle(
		response,
		response.Interactions,
		"Pending Session Interactions",
		[]string{"INTERACTION", cliKindHeader, "REQUEST", cliStatusHeader, "TITLE", cliSessionHeader, "EXPIRES AT"},
		"interactions",
		[]string{
			"interaction_id",
			agentKernelKindKey,
			"provider_request_id",
			automationStatusKey,
			cliOutputTitleKey,
			automationSessionIDKey,
			"expires_at",
		},
		sessionInteractionOutputRow,
		sessionInteractionOutputRow,
	)
}

func sessionInteractionOutputRow(item contract.PendingInteractionPayload) []string {
	expiresAt := "-"
	if item.ExpiresAt != nil {
		expiresAt = item.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return []string{
		stringOrDash(item.InteractionID),
		stringOrDash(item.Kind),
		stringOrDash(item.ProviderRequestID),
		stringOrDash(item.Status),
		stringOrDash(item.Title),
		stringOrDash(item.SessionID),
		expiresAt,
	}
}
