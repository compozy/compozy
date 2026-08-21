package cli

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/spf13/cobra"
)

func newLoopNodesCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, state, loopName, runID, inventoryRunID, cursor string
	var limit, generation int
	var all bool
	cmd := &cobra.Command{
		Use:   loopNodesKey,
		Short: "List workspace Loop node state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(runID) != "" {
				if err := validateRunRosterFlags(
					state,
					cursor,
					limit,
					generation,
					all,
					cmd.Flags().Changed("limit"),
				); err != nil {
					return err
				}
				client, workspaceID, err := loopReadClient(cmd, deps, workspaceRef)
				if err != nil {
					return err
				}
				query := LoopRunNodesQuery{
					State: strings.TrimSpace(state), Generation: generation,
					Cursor: strings.TrimSpace(cursor), Limit: limit,
				}
				response, err := loadLoopRunNodes(
					cmd, client, workspaceID, strings.TrimSpace(runID), query, all,
				)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, loopRunNodesOutputBundle(response))
			}
			if err := validateInventoryNodeState(state); err != nil {
				return withCommandExitCode(2, errors.New(
					err.Error(),
				))
			}
			if all || generation != 0 {
				return errors.New("cli: --all and --generation require --run")
			}
			if strings.TrimSpace(state) == "" {
				return errors.New("cli: --state requires --run or an inventory state")
			}
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.ListLoopNodes(cmd.Context(), workspaceID, LoopNodeListQuery{
				State:    strings.TrimSpace(state),
				LoopName: strings.TrimSpace(loopName),
				RunID:    strings.TrimSpace(inventoryRunID),
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
	cmd.Flags().StringVar(&state, loopStateKey, "", "Roster state filter")
	cmd.Flags().StringVar(&loopName, loopLoopKey, "", "Filter by Loop name")
	cmd.Flags().StringVar(&runID, "run", "", "Loop run ID")
	cmd.Flags().StringVar(&inventoryRunID, loopRunIDKey, "", "Filter workspace inventory by Loop run ID")
	cmd.Flags().BoolVar(&all, "all", false, "Show the complete run roster")
	cmd.Flags().IntVar(&generation, loopGenerationKey, 0, "Filter by generation")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Opaque continuation cursor")
	cmd.Flags().IntVar(&limit, "limit", 0, "Page size from 1 to 500; defaults to 50")
	return cmd
}

func validateRunRosterFlags(
	state string,
	cursor string,
	limit int,
	generation int,
	all bool,
	limitSet bool,
) error {
	state = strings.TrimSpace(state)
	if state != "" && !stringAllowed(state, looppkg.NodeStateFilterValues()) {
		return withCommandExitCode(2, fmt.Errorf(
			"invalid --state %q; allowed: %s",
			state,
			strings.Join(looppkg.NodeStateFilterValues(), "|"),
		))
	}
	if (limitSet && limit < 1) || limit > 500 {
		return withCommandExitCode(2, errors.New("--limit must be between 1 and 500"))
	}
	if generation < 0 {
		return withCommandExitCode(2, errors.New("--generation must be nonnegative"))
	}
	if all && strings.TrimSpace(cursor) != "" {
		return withCommandExitCode(2, errors.New("--cursor cannot be combined with --all"))
	}
	return nil
}

func validateInventoryNodeState(state string) error {
	state = strings.TrimSpace(state)
	if state == string(looppkg.NodeStateRunning) {
		return errors.New("--state running requires --run (workspace inventory tracks exception states only)")
	}
	allowed := []string{
		string(looppkg.NodeStateWaiting),
		string(looppkg.NodeStateQuarantined),
		configAttentionKey,
		string(looppkg.NodeStateRetrying),
	}
	if state != "" && !stringAllowed(state, allowed) {
		return fmt.Errorf("invalid --state %q; allowed: %s", state, strings.Join(allowed, "|"))
	}
	return nil
}

func stringAllowed(value string, allowed []string) bool {
	return slices.Contains(allowed, value)
}

func loadLoopRunNodes(
	cmd *cobra.Command,
	client loopRunReadClient,
	workspaceID string,
	runID string,
	query LoopRunNodesQuery,
	all bool,
) (contract.LoopRunNodesResponse, error) {
	if !all {
		return client.GetLoopRunNodes(cmd.Context(), workspaceID, runID, query)
	}
	query.Cursor = ""
	query.Limit = 500
	complete := contract.LoopRunNodesResponse{}
	for {
		page, err := client.GetLoopRunNodes(cmd.Context(), workspaceID, runID, query)
		if err != nil {
			return contract.LoopRunNodesResponse{}, err
		}
		if complete.RunID == "" {
			complete = page
			complete.Nodes = nil
		}
		complete.Nodes = append(complete.Nodes, page.Nodes...)
		if page.NextCursor == "" {
			complete.NextCursor = ""
			return complete, nil
		}
		query.Cursor = page.NextCursor
	}
}

func loopRunNodesOutputBundle(response contract.LoopRunNodesResponse) outputBundle {
	bundle := listBundle(
		response,
		response.Nodes,
		"Loop run nodes",
		[]string{loopRoundHeader, "STEP", strings.ToUpper(stateKey), "ATTEMPT", loopDurationHeader, "SESSION"},
		"loop_run_nodes",
		[]string{"generation", loopNodeIDJSONKey, loopItemIndexKey, stateKey, "attempt", "session_id", "cell_task_id"},
		loopRunNodeHumanRow,
		loopRunNodeTOONRow,
	)
	bundle.human = func() (string, error) {
		rows := make([][]string, 0, len(response.Nodes))
		for _, node := range response.Nodes {
			rows = append(rows, loopRunNodeHumanRow(node))
		}
		return renderLoopReadTable(
			[]string{loopRoundHeader, "STEP", "STATE", "ATTEMPT", loopDurationHeader, "SESSION"},
			rows,
		), nil
	}
	return bundle
}

func loopRunNodeHumanRow(item looppkg.RosterNode) []string {
	attempt := "—"
	if item.Attempt > 0 {
		attempt = strconv.Itoa(item.Attempt)
	}
	duration := "—"
	if item.StartedAt != nil && item.EndedAt != nil {
		duration = formatLoopReadDuration(item.EndedAt.Sub(*item.StartedAt))
	}
	sessionID := item.SessionID
	if sessionID == "" {
		sessionID = "—"
	}
	return []string{
		"g" + strconv.Itoa(item.Generation),
		string(item.NodeID),
		string(item.State),
		attempt,
		duration,
		sessionID,
	}
}

func loopRunNodeTOONRow(item looppkg.RosterNode) []string {
	return []string{
		strconv.Itoa(item.Generation),
		string(item.NodeID),
		strconv.Itoa(item.ItemIndex),
		string(item.State),
		strconv.Itoa(item.Attempt),
		item.SessionID,
		item.CellTaskID,
	}
}

func loopNodesOutputBundle(response contract.LoopNodeInventoryResponse) outputBundle {
	return listBundle(
		response,
		response.Items,
		"Loop nodes",
		[]string{"STATE", "RUN", "LOOP", "GEN", "NODE", "ITEM", "STATE AT"},
		"loop_nodes",
		[]string{
			stateKey,
			"loop_run_id",
			"loop_name",
			loopGenerationKey,
			loopNodeIDJSONKey,
			loopItemIndexKey,
			"state_at",
		},
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
