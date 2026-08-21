package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

const loopRunSummaryOutputsSQL = `
SELECT output.loop_run_id, output.generation, output.node_id, output.item_index, output.status
FROM loop_generation_outputs AS output
JOIN loop_runs AS run ON run.id = output.loop_run_id
WHERE run.workspace_id = ?
  AND output.loop_run_id IN (%s)
  AND output.generation = run.generation
ORDER BY output.loop_run_id, output.node_id, output.item_index`

const loopRunSummaryRouteEvidenceSQL = `
SELECT event.loop_run_id, event.kind, event.payload_json
FROM loop_run_events AS event
JOIN loop_runs AS run ON run.id = event.loop_run_id
WHERE run.workspace_id = ?
  AND event.loop_run_id IN (%s)
  AND event.kind IN ('route_taken', 'branch_pruned')
  AND CAST(json_extract(event.payload_json, '$.generation') AS INTEGER) = run.generation
ORDER BY event.loop_run_id, event.seq`

type loopRunSummaryProjection struct {
	source looppkg.RosterSource
}

func newLoopRunSummaryProjection(
	runID looppkg.RunID,
	generation int,
	digest string,
	definitionJSON string,
) (*loopRunSummaryProjection, error) {
	resolved, err := looppkg.LoadExecutedDefinitionSnapshot(json.RawMessage(definitionJSON), digest)
	if err != nil {
		return nil, fmt.Errorf("store: hydrate Loop run summary definition %q: %w", runID, err)
	}
	return &loopRunSummaryProjection{
		source: looppkg.RosterSource{
			Run:         looppkg.Run{ID: runID, Generation: generation},
			Graph:       resolved.Definition.Graph,
			Generations: []looppkg.LoopGeneration{{RunID: string(runID), Generation: int64(generation)}},
			PrunedNodes: make(map[string]bool),
		},
	}, nil
}

func (g *LoopRepo) projectLoopRunSummaryProgress(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	projections map[looppkg.RunID]*loopRunSummaryProjection,
	placeholders []string,
	args []any,
) error {
	if err := g.loadLoopRunSummaryOutputs(ctx, projections, placeholders, args); err != nil {
		return err
	}
	if err := g.loadLoopRunSummaryRouteEvidence(ctx, workspaceID, projections, placeholders, args); err != nil {
		return err
	}
	return nil
}

func (g *LoopRepo) loadLoopRunSummaryOutputs(
	ctx context.Context,
	projections map[looppkg.RunID]*loopRunSummaryProjection,
	placeholders []string,
	args []any,
) (err error) {
	// #nosec G201 -- interpolation is generated placeholders.
	query := fmt.Sprintf(loopRunSummaryOutputsSQL, strings.Join(placeholders, ","))
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("store: list Loop run summary outputs: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "Loop run summary outputs query")
	}()
	for rows.Next() {
		var runID, nodeID, status string
		var generation, itemIndex int
		if scanErr := rows.Scan(&runID, &generation, &nodeID, &itemIndex, &status); scanErr != nil {
			return fmt.Errorf("store: scan Loop run summary output: %w", scanErr)
		}
		projection, ok := projections[looppkg.RunID(runID)]
		if !ok {
			return fmt.Errorf("store: Loop run summary output has unknown run %q", runID)
		}
		projection.source.Outputs = append(projection.source.Outputs, looppkg.GenerationOutput{
			Generation: generation,
			NodeID:     nodeID,
			ItemIndex:  itemIndex,
			Status:     status,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate Loop run summary outputs: %w", err)
	}
	return nil
}

func (g *LoopRepo) loadLoopRunSummaryRouteEvidence(
	ctx context.Context,
	_ looppkg.WorkspaceID,
	projections map[looppkg.RunID]*loopRunSummaryProjection,
	placeholders []string,
	args []any,
) (err error) {
	// #nosec G201 -- interpolation is generated placeholders.
	query := fmt.Sprintf(loopRunSummaryRouteEvidenceSQL, strings.Join(placeholders, ","))
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("store: list Loop run summary route evidence: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "Loop run summary route evidence query")
	}()
	for rows.Next() {
		var runID, kind, payloadJSON string
		if scanErr := rows.Scan(&runID, &kind, &payloadJSON); scanErr != nil {
			return fmt.Errorf("store: scan Loop run summary route evidence: %w", scanErr)
		}
		projection, ok := projections[looppkg.RunID(runID)]
		if !ok {
			return fmt.Errorf("store: Loop run summary route evidence has unknown run %q", runID)
		}
		if err := applyLoopRunSummaryRouteEvidence(&projection.source, kind, payloadJSON); err != nil {
			return fmt.Errorf("store: decode Loop run summary route evidence: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate Loop run summary route evidence: %w", err)
	}
	return nil
}

func applyLoopRunSummaryRouteEvidence(source *looppkg.RosterSource, kind, payloadJSON string) error {
	var payload struct {
		Generation  int64  `json:"generation"`
		NodeID      string `json:"node_id"`
		ItemIndex   int    `json:"item_index"`
		Route       string `json:"route"`
		Cause       string `json:"cause"`
		MatchedWhen string `json:"matched_when"`
		Default     bool   `json:"default"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return err
	}
	if payload.Generation < 1 || strings.TrimSpace(payload.NodeID) == "" {
		return fmt.Errorf("invalid %s payload", kind)
	}
	if kind == string(looppkg.RunEventBranchPruned) {
		source.MarkPrunedNode(int(payload.Generation), looppkg.NodeID(payload.NodeID))
		return nil
	}
	if strings.TrimSpace(payload.Route) == "" || strings.TrimSpace(payload.Cause) == "" || payload.ItemIndex < 0 {
		return fmt.Errorf("invalid route_taken payload")
	}
	source.RouteCauses = append(source.RouteCauses, looppkg.RouteCause{
		Generation:  payload.Generation,
		NodeID:      looppkg.NodeID(payload.NodeID),
		ItemIndex:   payload.ItemIndex,
		Route:       looppkg.NodeID(payload.Route),
		Cause:       payload.Cause,
		MatchedWhen: payload.MatchedWhen,
		Default:     payload.Default,
	})
	return nil
}
