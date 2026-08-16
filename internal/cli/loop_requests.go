package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newLoopRequestsCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, state, cursor string
	var limit int
	cmd := &cobra.Command{
		Use: loopRequestsKey, Short: "List human requests", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.ListLoopRequests(cmd.Context(), workspaceID, LoopRequestListQuery{
				RunID: strings.TrimSpace(runID), State: strings.TrimSpace(state),
				Cursor: strings.TrimSpace(cursor), Limit: limit,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response,
				fmt.Sprintf("%d requests · %d pending", len(response.Items), response.Aggregates.Pending)))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&runID, loopRunIDKey, "", "Filter by Loop run ID")
	cmd.Flags().StringVar(&state, loopStateKey, "pending", "Filter by pending or resolved")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from a request cursor")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum requests to return")
	return cmd
}

func newLoopRequestCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, nodeID string
	var itemIndex int
	cmd := &cobra.Command{
		Use: loopRequestKey, Short: "Inspect one human request", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.GetLoopRequest(cmd.Context(), workspaceID,
				strings.TrimSpace(runID), strings.TrimSpace(nodeID), itemIndex)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response,
				fmt.Sprintf("%s · %s · run %s", response.State, response.NodeID, response.LoopRunID)))
		},
	}
	addLoopRequestFlags(cmd, &workspaceRef, &runID, &nodeID, &itemIndex)
	return cmd
}

func newLoopRespondCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, nodeID, decision, payload, note string
	var itemIndex int
	cmd := &cobra.Command{
		Use: loopRespondKey, Short: "Answer one human request", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			raw := json.RawMessage(strings.TrimSpace(payload))
			if len(raw) == 0 || !json.Valid(raw) {
				return errors.New("cli: --payload must be valid JSON")
			}
			response, err := client.RespondLoopRequest(cmd.Context(), workspaceID,
				strings.TrimSpace(runID), strings.TrimSpace(nodeID), contract.RespondLoopRequest{
					ItemIndex: itemIndex, Decision: strings.TrimSpace(decision), Payload: raw,
					Note: strings.TrimSpace(note),
				}, agentCredentialsFromEnv(deps))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response,
				fmt.Sprintf("answered · %s · run %s resumed", response.NodeID, response.RunID)))
		},
	}
	addLoopRequestFlags(cmd, &workspaceRef, &runID, &nodeID, &itemIndex)
	cmd.Flags().StringVar(&decision, loopDecisionKey, "", "Request decision")
	cmd.Flags().StringVar(&payload, loopPayloadKey, "", "JSON response payload")
	cmd.Flags().StringVar(&note, "note", "", "Resolution note")
	mustMarkFlagRequired(cmd, loopPayloadKey)
	return cmd
}

func addLoopRequestFlags(
	cmd *cobra.Command,
	workspaceRef *string,
	runID *string,
	nodeID *string,
	itemIndex *int,
) {
	cmd.Flags().StringVar(workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(runID, loopRunIDKey, "", "Loop run ID")
	cmd.Flags().StringVar(nodeID, loopNodeKey, "", "Request node ID")
	cmd.Flags().IntVar(itemIndex, "item", 0, "Fan-out item index")
	mustMarkFlagRequired(cmd, loopRunIDKey)
	mustMarkFlagRequired(cmd, loopNodeKey)
}
