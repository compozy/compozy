package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

// readLoopWatchEventsCursor returns the latest durable position in the same
// eligible population that readLoopWatchEvents can return.
func (g *WatchEventsRepo) readLoopWatchEventsCursor(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) (int64, error) {
	placeholders, args := loopWatchEventsEligibility(query.workspaceID, query.kinds)
	// #nosec G202 -- IN placeholders are generated from normalized kind count; values are parameterized.
	return scanWatchEventCursor(
		g.db.QueryRowContext(
			ctx,
			`SELECT COALESCE(MAX(lre.watch_seq), 0)
			   FROM loop_run_events lre
			   JOIN loop_runs lr
			     ON lr.id = lre.loop_run_id
			    AND lr.workspace_id = lre.workspace_id
			  WHERE `+loopWatchEventsEligibilitySQL(placeholders),
			args...,
		),
		looppkg.WatchEventsLoopStream,
	)
}

// readLoopWatchEvents returns loop-stream ledger rows strictly after the
// stream cursor, projecting each row's watch sequence as the cursor value.
func (g *WatchEventsRepo) readLoopWatchEvents(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) ([]looppkg.WatchEvent, error) {
	placeholders, args := loopWatchEventsEligibility(query.workspaceID, query.kinds)
	args = append(args, query.streams[looppkg.WatchEventsLoopStream], query.limit)
	// #nosec G202 -- IN placeholders are generated from normalized kind count; values are parameterized.
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT
			lre.watch_seq,
			lre.kind,
			lre.loop_run_id,
			lre.workspace_id,
			lre.payload_json,
			lre.at,
			COALESCE(lr.loop_name, '')
		   FROM loop_run_events lre
		   JOIN loop_runs lr
		     ON lr.id = lre.loop_run_id
		    AND lr.workspace_id = lre.workspace_id
		  WHERE `+loopWatchEventsEligibilitySQL(placeholders)+`
		    AND lre.watch_seq > ?
		  ORDER BY lre.watch_seq ASC
		  LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: read loop watch-events: %w", err)
	}
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

// loopWatchEventsEligibility is shared by cursor and row reads so a row that
// advances the gap cursor is always readable by the replay path.
func loopWatchEventsEligibility(workspaceID string, kinds []string) (string, []any) {
	placeholders, kindArgs := sqlInPlaceholders(kinds)
	args := append([]any{workspaceID}, kindArgs...)
	args = append(args,
		loopRunEventStatusChanged,
		string(looppkg.StatusDone),
		string(looppkg.StatusNoOp),
		string(looppkg.StatusBlocked),
		string(looppkg.StatusFailed),
		string(looppkg.StatusExhausted),
		string(looppkg.StatusStalled),
		string(looppkg.StatusCanceled),
	)
	return placeholders, args
}

func loopWatchEventsEligibilitySQL(placeholders string) string {
	return `lre.workspace_id = ?
	    AND lre.kind IN (` + placeholders + `)
	    AND (
		lre.kind <> ?
		OR COALESCE(
			json_extract(lre.payload_json, '$.to'),
			json_extract(lre.payload_json, '$.status')
		) IN (?, ?, ?, ?, ?, ?, ?)
	    )`
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
