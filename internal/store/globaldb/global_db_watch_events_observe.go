package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
)

func (g *WatchEventsRepo) readObserveWatchEventsCursor(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) (int64, error) {
	if len(query.kinds) == 0 {
		return 0, nil
	}
	// dynamic-sql: caller-selected observability event kinds require a variable-width IN list.
	placeholders, kindArgs := sqlInPlaceholders(query.kinds)
	args := append([]any{query.workspaceID}, kindArgs...)
	// #nosec G202 -- IN placeholders are generated from normalized kind count; values are parameterized.
	return scanWatchEventCursor(
		g.db.QueryRowContext(
			ctx,
			`SELECT COALESCE(MAX(rowid), 0)
			   FROM event_summaries
			  WHERE workspace_id = ?
			    AND type IN (`+placeholders+`)`,
			args...,
		),
		looppkg.WatchEventsObserveStream,
	)
}

func (g *WatchEventsRepo) readObserveWatchEvents(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) ([]looppkg.WatchEvent, error) {
	if len(query.kinds) == 0 {
		return nil, nil
	}
	// dynamic-sql: caller-selected observability event kinds require a variable-width IN list.
	placeholders, kindArgs := sqlInPlaceholders(query.kinds)
	args := append([]any{
		query.workspaceID,
		query.streams[looppkg.WatchEventsObserveStream],
	}, kindArgs...)
	args = append(args, query.limit)
	// #nosec G202 -- IN placeholders are generated from normalized kind count; values are parameterized.
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT
			rowid,
			type,
			workspace_id,
			task_id,
			run_id,
			workflow_id,
			coordinator_session_id,
			agent_name,
			provider,
			content_json,
			timestamp
		   FROM event_summaries
		  WHERE workspace_id = ?
		    AND rowid > ?
		    AND type IN (`+placeholders+`)
		  ORDER BY rowid ASC
		  LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: read observe watch-events: %w", err)
	}
	defer rows.Close()

	events := make([]looppkg.WatchEvent, 0)
	for rows.Next() {
		event, scanErr := scanObserveWatchEvent(rows)
		if scanErr != nil {
			return nil, joinRowsCloseError(rows, scanErr, "observe watch-events query")
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, joinRowsCloseError(
			rows,
			fmt.Errorf("store: iterate observe watch-events: %w", err),
			"observe watch-events query",
		)
	}
	if err := joinRowsCloseError(rows, nil, "observe watch-events query"); err != nil {
		return nil, err
	}
	return events, nil
}

func scanObserveWatchEvent(row rowScanner) (looppkg.WatchEvent, error) {
	var (
		event        looppkg.WatchEvent
		payloadRaw   string
		timestampRaw string
		workflowID   string
		agentName    string
		provider     string
	)
	if err := row.Scan(
		&event.Seq,
		&event.Kind,
		&event.WorkspaceID,
		&event.TaskID,
		&event.RunID,
		&workflowID,
		&event.SessionID,
		&agentName,
		&provider,
		&payloadRaw,
		&timestampRaw,
	); err != nil {
		return looppkg.WatchEvent{}, fmt.Errorf("store: scan observe watch-event: %w", err)
	}
	payload, err := decodeWatchEventPayload(payloadRaw)
	if err != nil {
		return looppkg.WatchEvent{}, err
	}
	addObservePayloadDefault(payload, watchEventsPayloadWorkflowIDKey, workflowID)
	addObservePayloadDefault(payload, watchEventsPayloadAgentNameKey, agentName)
	addObservePayloadDefault(payload, watchEventsPayloadProviderKey, provider)
	addObservePayloadDefault(payload, watchEventsPayloadSessionIDKey, event.SessionID)
	at, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		return looppkg.WatchEvent{}, fmt.Errorf("store: parse observe watch-event timestamp: %w", err)
	}
	event.Stream = looppkg.WatchEventsObserveStream
	event.LedgerKind = strings.TrimSpace(event.Kind)
	event.Payload = payload
	event.At = formatWatchEventAt(at)
	return event, nil
}

func addObservePayloadDefault(payload map[string]any, key string, value string) {
	if _, ok := payload[key]; ok {
		return
	}
	payload[key] = strings.TrimSpace(value)
}
