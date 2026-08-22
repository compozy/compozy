package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	var generation, itemIndex int
	cmd := &cobra.Command{
		Use: loopRequestKey, Short: "Inspect one human request", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.GetLoopRequest(cmd.Context(), workspaceID,
				strings.TrimSpace(runID), generation, strings.TrimSpace(nodeID), itemIndex)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response,
				fmt.Sprintf("%s · %s · run %s", response.State, response.NodeID, response.LoopRunID)))
		},
	}
	addLoopRequestFlags(cmd, &workspaceRef, &runID, &generation, &nodeID, &itemIndex)
	return cmd
}

func newLoopRespondCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, nodeID, decision, payload, note string
	var payloadFromStdin bool
	var generation, itemIndex int
	cmd := &cobra.Command{
		Use: loopRespondKey, Short: "Answer or decide one human request", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			normalizedDecision := strings.TrimSpace(decision)
			raw, err := loopRespondPayloadInput(cmd, normalizedDecision, payload, payloadFromStdin)
			if err != nil {
				return err
			}
			response, err := client.RespondLoopRequest(cmd.Context(), workspaceID,
				strings.TrimSpace(runID), strings.TrimSpace(nodeID), contract.RespondLoopRequest{
					Generation: generation, ItemIndex: itemIndex, Decision: normalizedDecision, Payload: raw,
					Note: strings.TrimSpace(note),
				}, agentCredentialsFromEnv(deps))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopOutputBundle(response,
				fmt.Sprintf("answered · %s · run %s resumed", response.NodeID, response.RunID)))
		},
	}
	addLoopRequestFlags(cmd, &workspaceRef, &runID, &generation, &nodeID, &itemIndex)
	cmd.Flags().StringVar(&decision, loopDecisionKey, "", "Request decision")
	cmd.Flags().StringVar(&payload, loopPayloadKey, "", "JSON response payload")
	cmd.Flags().BoolVar(&payloadFromStdin, "payload-stdin", false, "Read the JSON response payload from stdin")
	cmd.MarkFlagsMutuallyExclusive(loopPayloadKey, "payload-stdin")
	cmd.Flags().StringVar(&note, "note", "", "Resolution note")
	return cmd
}

func loopRespondPayloadInput(
	cmd *cobra.Command,
	decision string,
	payload string,
	fromStdin bool,
) (json.RawMessage, error) {
	if !fromStdin {
		return loopRespondPayload(decision, payload)
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), "Response JSON: "); err != nil {
		return nil, fmt.Errorf("cli: write Loop response prompt: %w", err)
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("cli: read Loop response payload from stdin: %w", err)
	}
	return loopRespondPayload(decision, line)
}

func loopRespondPayload(decision string, payload string) (json.RawMessage, error) {
	decision = strings.TrimSpace(decision)
	payload = strings.TrimSpace(payload)
	requiresPayload := decision == "" || decision == loopEditKey || decision == loopRespondKey
	if payload == "" {
		if requiresPayload {
			return nil, errors.New("cli: --payload must be valid JSON")
		}
		return json.RawMessage(`null`), nil
	}
	if !json.Valid([]byte(payload)) {
		return nil, errors.New("cli: --payload must be valid JSON")
	}
	return json.RawMessage(payload), nil
}

func addLoopRequestFlags(
	cmd *cobra.Command,
	workspaceRef *string,
	runID *string,
	generation *int,
	nodeID *string,
	itemIndex *int,
) {
	cmd.Flags().StringVar(workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(runID, loopRunIDKey, "", "Loop run ID")
	cmd.Flags().IntVar(generation, loopGenerationKey, 0, "Loop generation")
	cmd.Flags().StringVar(nodeID, loopNodeKey, "", "Request node ID")
	cmd.Flags().IntVar(itemIndex, "item", 0, "Fan-out item index")
	mustMarkFlagRequired(cmd, loopGenerationKey)
	mustMarkFlagRequired(cmd, loopRunIDKey)
	mustMarkFlagRequired(cmd, loopNodeKey)
}
