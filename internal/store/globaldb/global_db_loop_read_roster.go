package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *LoopRepo) ListLoopRosterOutputs(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (outputs []looppkg.GenerationOutput, err error) {
	if err := g.checkReady(ctx, "list loop roster outputs"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(runID)) == "" {
		return nil, fmt.Errorf("%w: roster output scope is invalid", looppkg.ErrValidation)
	}
	rows, err := g.db.QueryContext(ctx, `SELECT output.generation, output.node_id, output.item_index,
		output.status, output.output_ref, output.task_run_id, output.child_loop_run_id,
		output.resolved_runtime_json, output.attempt, output.next_attempt_at,
		output.first_scheduled_at, output.epoch, COALESCE(task_run.session_id, '')
		FROM loop_generation_outputs AS output
		JOIN loop_runs AS run ON run.id = output.loop_run_id
		LEFT JOIN task_runs AS task_run ON task_run.id = output.task_run_id
		WHERE run.workspace_id = ? AND output.loop_run_id = ?
		ORDER BY output.generation ASC, output.node_id ASC, output.item_index ASC`, workspaceID, runID)
	if err != nil {
		return nil, fmt.Errorf("store: list loop roster outputs: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "loop roster outputs query")
	}()
	for rows.Next() {
		var row sqlcgen.ListLoopGenerationOutputsRow
		if scanErr := rows.Scan(
			&row.Generation,
			&row.NodeID,
			&row.ItemIndex,
			&row.Status,
			&row.OutputRef,
			&row.TaskRunID,
			&row.ChildLoopRunID,
			&row.ResolvedRuntimeJson,
			&row.Attempt,
			&row.NextAttemptAt,
			&row.FirstScheduledAt,
			&row.Epoch,
			&row.SessionID,
		); scanErr != nil {
			return nil, fmt.Errorf("store: scan loop roster output: %w", scanErr)
		}
		output, mapErr := generationOutputFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		outputs = append(outputs, output)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate loop roster outputs: %w", err)
	}
	return outputs, nil
}

func (g *LoopRepo) ListLoopRosterRouteCauses(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (causes []looppkg.RouteCause, err error) {
	if err := g.checkReady(ctx, "list loop roster route causes"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(runID)) == "" {
		return nil, fmt.Errorf("%w: roster route-cause scope is invalid", looppkg.ErrValidation)
	}
	rows, err := g.db.QueryContext(ctx, `SELECT event.payload_json, event.at
		FROM loop_run_events AS event
		JOIN loop_runs AS run ON run.id = event.loop_run_id
		WHERE run.workspace_id = ? AND event.loop_run_id = ? AND event.kind = 'route_taken'
		ORDER BY event.seq ASC`, workspaceID, runID)
	if err != nil {
		return nil, fmt.Errorf("store: list loop roster route causes: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "loop roster route causes query")
	}()
	for rows.Next() {
		var payload string
		var at time.Time
		if scanErr := rows.Scan(&payload, &at); scanErr != nil {
			return nil, fmt.Errorf("store: scan loop roster route cause: %w", scanErr)
		}
		cause, decodeErr := decodeStoredRouteCause(payload, at, 0)
		if decodeErr != nil {
			return nil, decodeErr
		}
		causes = append(causes, cause)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate loop roster route causes: %w", err)
	}
	return causes, nil
}

func decodeStoredRouteCause(raw string, at time.Time, expectedGeneration int64) (looppkg.RouteCause, error) {
	var payload struct {
		Generation  int64  `json:"generation"`
		NodeID      string `json:"node_id"`
		ItemIndex   int    `json:"item_index"`
		Route       string `json:"route"`
		Cause       string `json:"cause"`
		MatchedWhen string `json:"matched_when"`
		Default     bool   `json:"default"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return looppkg.RouteCause{}, fmt.Errorf("store: decode loop route cause: %w", err)
	}
	payload.NodeID = strings.TrimSpace(payload.NodeID)
	payload.Route = strings.TrimSpace(payload.Route)
	payload.Cause = strings.TrimSpace(payload.Cause)
	if payload.Generation < 1 || (expectedGeneration > 0 && payload.Generation != expectedGeneration) ||
		payload.NodeID == "" || payload.ItemIndex < 0 || payload.Route == "" || payload.Cause == "" {
		return looppkg.RouteCause{}, fmt.Errorf("%w: persisted route cause is invalid", looppkg.ErrValidation)
	}
	return looppkg.RouteCause{
		Generation: payload.Generation,
		NodeID:     looppkg.NodeID(payload.NodeID), ItemIndex: payload.ItemIndex,
		Route: looppkg.NodeID(payload.Route), Cause: payload.Cause,
		MatchedWhen: strings.TrimSpace(payload.MatchedWhen), Default: payload.Default, At: at,
	}, nil
}
