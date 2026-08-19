package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newLoopNodesCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, state, loopName, runID, cursor string
	var limit int
	cmd := &cobra.Command{
		Use:   loopNodesKey,
		Short: "List workspace Loop node state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.ListLoopNodes(cmd.Context(), workspaceID, LoopNodeListQuery{
				State:    strings.TrimSpace(state),
				LoopName: strings.TrimSpace(loopName),
				RunID:    strings.TrimSpace(runID),
				Cursor:   strings.TrimSpace(cursor),
				Limit:    limit,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopNodesOutputBundle(response))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&state, loopStateKey, "", "State: waiting, quarantined, attention, or retrying")
	cmd.Flags().StringVar(&loopName, loopLoopKey, "", "Filter by Loop name")
	cmd.Flags().StringVar(&runID, loopRunIDKey, "", "Filter by Loop run ID")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Opaque continuation cursor")
	cmd.Flags().IntVar(&limit, "limit", 0, "Page size from 1 to 200; defaults to 50")
	mustMarkFlagRequired(cmd, loopStateKey)
	return cmd
}

func loopNodesOutputBundle(response contract.LoopNodeInventoryResponse) outputBundle {
	return listBundle(
		response,
		response.Items,
		"Loop nodes",
		[]string{"STATE", "RUN", "LOOP", "GEN", "NODE", "ITEM", "STATE AT"},
		"loop_nodes",
		[]string{stateKey, "loop_run_id", "loop_name", loopGenerationKey, "node_id", "item_index", "state_at"},
		loopNodeInventoryRow,
		loopNodeInventoryRow,
	)
}

func loopNodeInventoryRow(item contract.LoopNodeInventoryItem) []string {
	return []string{
		string(item.State),
		item.LoopRunID,
		item.LoopName,
		strconv.Itoa(item.Generation),
		item.NodeID,
		strconv.Itoa(item.ItemIndex),
		item.StateAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func loopMutationOutputBundle(response contract.LoopMutationResponse, verb string) outputBundle {
	bundle := loopOutputBundle(response, "Loop "+strings.TrimSpace(response.RunID)+" "+strings.TrimSpace(verb))
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLine(cmd, response)
	}
	return bundle
}
