package cli

import (
	"strings"

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

func sessionInteractionsBundle(response SessionInteractionsRecord) outputBundle {
	return listBundle(
		response,
		response.Interactions,
		"Pending Session Interactions",
		[]string{"INTERACTION", cliKindHeader, "REQUEST", cliStatusHeader, "TITLE"},
		"interactions",
		[]string{
			"interaction_id", agentKernelKindKey, "provider_request_id", automationStatusKey, networkTitleKey,
		},
		sessionInteractionOutputRow,
		sessionInteractionOutputRow,
	)
}

func sessionInteractionOutputRow(item contract.PendingInteractionPayload) []string {
	return []string{
		stringOrDash(item.InteractionID),
		stringOrDash(item.Kind),
		stringOrDash(item.ProviderRequestID),
		stringOrDash(item.Status),
		stringOrDash(item.Title),
	}
}
