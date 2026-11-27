package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

const loopRunTerminalCausesSQL = `
SELECT event.loop_run_id, event.payload_json, CAST(event.at AS TEXT)
FROM loop_run_events AS event
JOIN (
	SELECT loop_run_id, MAX(seq) AS seq
	FROM loop_run_events
	WHERE workspace_id = ? AND kind = 'status_changed' AND loop_run_id IN (%s)
	GROUP BY loop_run_id
) AS latest ON latest.loop_run_id = event.loop_run_id AND latest.seq = event.seq
WHERE event.kind = 'status_changed'`

type loopRunTerminalCausePayload struct {
	Status     string `json:"status"`
	Cause      string `json:"cause"`
	ReasonCode string `json:"reason_code"`
	GateID     string `json:"gate_id"`
	NodeID     string `json:"node_id"`
	Message    string `json:"message"`
}

// ListLoopRunTerminalCauses projects the settled terminal cause from each run's
// last status change. Non-terminal runs are omitted.
func (g *LoopRepo) ListLoopRunTerminalCauses(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runIDs []looppkg.RunID,
) (causes map[looppkg.RunID]looppkg.RunTerminalCause, err error) {
	if err := g.checkReady(ctx, "list loop run terminal causes"); err != nil {
		return nil, err
	}
	if len(runIDs) == 0 {
		return map[looppkg.RunID]looppkg.RunTerminalCause{}, nil
	}
	placeholders, args := loopRunSummaryQueryArgs(workspaceID, runIDs)
	// dynamic-sql: the selected run IDs define the placeholder count; every value is parameterized.
	// #nosec G201 -- the SQL body is constant and the only interpolation is generated question marks.
	query := fmt.Sprintf(loopRunTerminalCausesSQL, strings.Join(placeholders, ","))
	rows, err := g.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop run terminal causes: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "loop run terminal causes query")
	}()
	causes = make(map[looppkg.RunID]looppkg.RunTerminalCause, len(placeholders))
	for rows.Next() {
		var runID, payloadJSON, at string
		if scanErr := rows.Scan(&runID, &payloadJSON, &at); scanErr != nil {
			return nil, fmt.Errorf("store: scan Loop run terminal cause: %w", scanErr)
		}
		var payload loopRunTerminalCausePayload
		if unmarshalErr := json.Unmarshal([]byte(payloadJSON), &payload); unmarshalErr != nil {
			return nil, fmt.Errorf("store: decode Loop run %q terminal cause: %w", runID, unmarshalErr)
		}
		status := looppkg.Status(strings.TrimSpace(payload.Status))
		if !status.Terminal() {
			continue
		}
		atTime, timeErr := parseLoopRunSummaryTimestamp(at)
		if timeErr != nil {
			return nil, fmt.Errorf("store: decode Loop run %q terminal cause time: %w", runID, timeErr)
		}
		causes[looppkg.RunID(runID)] = looppkg.RunTerminalCause{
			Status:     status,
			Cause:      looppkg.TransitionCause(strings.TrimSpace(payload.Cause)),
			ReasonCode: strings.TrimSpace(payload.ReasonCode),
			GateID:     looppkg.NodeID(strings.TrimSpace(payload.GateID)),
			NodeID:     looppkg.NodeID(strings.TrimSpace(payload.NodeID)),
			Message:    strings.TrimSpace(payload.Message),
			At:         atTime,
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("store: list Loop run terminal causes: %w", rowsErr)
	}
	return causes, nil
}
