package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
)

func (g *WatchEventsRepo) readLoopWatchEvents(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) ([]looppkg.WatchEvent, error) {
	// dynamic-sql: caller-selected loop event kinds require a variable-width IN list.
	placeholders, kindArgs := sqlInPlaceholders(query.kinds)
	args := append([]any{query.workspaceID, query.streams[looppkg.WatchEventsLoopStream]}, kindArgs...)
	args = append(args,
		loopRunEventStatusChanged,
		string(looppkg.StatusDone),
		string(looppkg.StatusNoOp),
		string(looppkg.StatusBlocked),
		string(looppkg.StatusFailed),
		string(looppkg.StatusExhausted),
		string(looppkg.StatusStalled),
		query.limit,
	)
	// #nosec G202 -- IN placeholders are generated from normalized kind count; values are parameterized.
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT
			lre.seq,
			lre.kind,
			lre.loop_run_id,
			lre.workspace_id,
			lre.payload_json,
			lre.at,
			COALESCE(lr.loop_name, '')
		   FROM loop_run_events lre
		   JOIN loop_runs lr ON lr.id = lre.loop_run_id
		  WHERE lre.workspace_id = ?
		    AND lre.seq > ?
		    AND lre.kind IN (`+placeholders+`)
		    AND (
			lre.kind <> ?
			OR COALESCE(
				json_extract(lre.payload_json, '$.to'),
				json_extract(lre.payload_json, '$.status')
			) IN (?, ?, ?, ?, ?, ?)
		    )
		  ORDER BY lre.seq ASC
		  LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: read loop watch-events: %w", err)
	}
	defer rows.Close()

	events := make([]looppkg.WatchEvent, 0, query.limit)
	for rows.Next() {
		event, scanErr := scanLoopWatchEvent(rows)
		if scanErr != nil {
			return nil, joinRowsCloseError(rows, scanErr, "loop watch-events query")
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, joinRowsCloseError(
			rows,
			fmt.Errorf("store: iterate loop watch-events: %w", err),
			"loop watch-events query",
		)
	}
	if err := joinRowsCloseError(rows, nil, "loop watch-events query"); err != nil {
		return nil, err
	}
	return events, nil
}

func scanLoopWatchEvent(row rowScanner) (looppkg.WatchEvent, error) {
	var (
		event      looppkg.WatchEvent
		payloadRaw string
		atRaw      string
	)
	if err := row.Scan(
		&event.Seq,
		&event.Kind,
		&event.LoopRunID,
		&event.WorkspaceID,
		&payloadRaw,
		&atRaw,
		&event.LoopName,
	); err != nil {
		return looppkg.WatchEvent{}, fmt.Errorf("store: scan loop watch-event: %w", err)
	}
	payload, err := decodeWatchEventPayload(payloadRaw)
	if err != nil {
		return looppkg.WatchEvent{}, err
	}
	at, err := parseLoopRunTimestamp(atRaw)
	if err != nil {
		return looppkg.WatchEvent{}, fmt.Errorf("store: parse loop watch-event at: %w", err)
	}
	event.Stream = looppkg.WatchEventsLoopStream
	event.LedgerKind = event.Kind
	event.Payload = payload
	event.At = formatWatchEventAt(at)
	if taskID, ok := payload["task_id"].(string); ok {
		event.TaskID = strings.TrimSpace(taskID)
	}
	if taskRunID, ok := payload["task_run_id"].(string); ok {
		event.RunID = strings.TrimSpace(taskRunID)
	}
	return event, nil
}
