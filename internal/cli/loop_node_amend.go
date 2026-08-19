package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newLoopNodeAmendCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, nodeID, payload, reason string
	var itemIndex int
	cmd := &cobra.Command{
		Use: loopAmendKey, Short: "Amend one parked Loop node output", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			raw := json.RawMessage(strings.TrimSpace(payload))
			if len(raw) == 0 || !json.Valid(raw) {
				return errors.New("cli: --payload must be valid JSON")
			}
			response, err := client.AmendLoopNode(cmd.Context(), workspaceID,
				strings.TrimSpace(runID), strings.TrimSpace(nodeID), contract.LoopNodeAmendRequest{
					ItemIndex: itemIndex, Payload: raw, Reason: strings.TrimSpace(reason),
				}, agentCredentialsFromEnv(deps))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response,
				fmt.Sprintf("amended · %s · original preserved in history", response.Amendment.NodeID)))
		},
	}
	addLoopNodeIdentityFlags(cmd, &workspaceRef, &runID, &nodeID)
	cmd.Flags().StringVar(&payload, loopPayloadKey, "", "JSON replacement output")
	cmd.Flags().StringVar(&reason, loopReasonKey, "", "Operator reason")
	cmd.Flags().IntVar(&itemIndex, loopItemKey, 0, "Fan-out item index")
	mustMarkFlagRequired(cmd, loopPayloadKey)
	return cmd
}
