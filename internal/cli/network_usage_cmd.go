package cli

import (
	"github.com/compozy/agh/internal/store"
	"github.com/spf13/cobra"
)

func newNetworkUsageCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var query NetworkUsageQuery
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show workspace-scoped network wake usage from the ledger",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			payload, err := client.GetNetworkUsage(cmd.Context(), workspaceID, query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, networkUsageOutputBundle(payload))
		},
	}
	cmd.Flags().StringVar(&query.OwnerKind, "owner-kind", "", "Filter by owner kind")
	cmd.Flags().StringVar(&query.OwnerID, "owner-id", "", "Filter by owner id")
	cmd.Flags().StringVar(&query.RunID, "run-id", "", "Filter by task or loop run id")
	cmd.Flags().StringVar(&query.Channel, "channel", "", "Filter by channel")
	cmd.Flags().StringVar(&query.Cursor, "cursor", "", "Continue from an opaque next_cursor")
	cmd.Flags().IntVar(&query.Limit, "limit", store.DefaultNetworkUsageLimit, "Maximum usage rows to return")
	return cmd
}
