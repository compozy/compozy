package globaldb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
)

const (
	watchEventsParentTaskIDPayloadKey = "parent_task_id"

	watchEventsPayloadAgentNameKey            = "agent_name"
	watchEventsPayloadAttemptKey              = "attempt"
	watchEventsPayloadCausationIDKey          = "causation_id"
	watchEventsPayloadChannelKey              = "channel"
	watchEventsPayloadCoordinatorSessionIDKey = "coordinator_session_id"
	watchEventsPayloadDecisionKey             = "decision"
	watchEventsPayloadDecisionKindKey         = "decision_kind"
	watchEventsPayloadDirectIDKey             = "direct_id"
	watchEventsPayloadDirectionKey            = "direction"
	watchEventsPayloadDurationMSKey           = "duration_ms"
	watchEventsPayloadErrorKey                = "error"
	watchEventsPayloadJobIDKey                = "job_id"
	watchEventsPayloadMessageIDKey            = "message_id"
	watchEventsPayloadModelKey                = "model"
	watchEventsPayloadPeerFromKey             = "peer_from"
	watchEventsPayloadPeerToKey               = "peer_to"
	watchEventsPayloadProviderKey             = "provider"
	watchEventsPayloadRecordTypeKey           = "record_type"
	watchEventsPayloadSequenceKey             = "sequence"
	watchEventsPayloadSessionIDKey            = "session_id"
	watchEventsPayloadSurfaceKey              = "surface"
	watchEventsPayloadStopReasonKey           = "stop_reason"
	watchEventsPayloadThreadIDKey             = "thread_id"
	watchEventsPayloadTraceIDKey              = "trace_id"
	watchEventsPayloadTurnIDKey               = "turn_id"
	watchEventsPayloadTriggerIDKey            = "trigger_id"
	watchEventsPayloadWillRetryKey            = "will_retry"
	watchEventsPayloadWorkIDKey               = "work_id"
	watchEventsPayloadWorkStateKey            = "work_state"
	watchEventsPayloadWorkflowIDKey           = "workflow_id"
)

type normalizedWatchEventsQuery struct {
	workspaceID string
	streams     map[string]int64
	kinds       []string
	limit       int
}

// ReadCursors returns the current max replay cursor for each requested stream.
func (g *WatchEventsRepo) ReadCursors(
	ctx context.Context,
	query looppkg.WatchEventsQuery,
) (map[string]int64, error) {
	if err := g.checkReady(ctx, "read watch-events cursors"); err != nil {
		return nil, err
	}
	normalized, err := normalizeWatchEventsQuery(query)
	if err != nil {
		return nil, err
	}
	cursors := make(map[string]int64, len(normalized.streams))
	for stream := range normalized.streams {
		cursor, err := g.readWatchEventsCursor(ctx, normalized, stream)
		if err != nil {
			return nil, err
		}
		cursors[stream] = cursor
	}
	return cursors, nil
}

// ReadMatches returns replay rows strictly after each stream cursor.
func (g *WatchEventsRepo) ReadMatches(
	ctx context.Context,
	query looppkg.WatchEventsQuery,
) ([]looppkg.WatchEvent, error) {
	if err := g.checkReady(ctx, "read watch-events matches"); err != nil {
		return nil, err
	}
	normalized, err := normalizeWatchEventsQuery(query)
	if err != nil {
		return nil, err
	}
	events := make([]looppkg.WatchEvent, 0)
	for _, stream := range sortedWatchEventStreams(normalized.streams) {
		streamEvents, err := g.readWatchEventsStreamMatches(ctx, normalized, stream)
		if err != nil {
			return nil, err
		}
		events = append(events, streamEvents...)
	}
	return events, nil
}

func normalizeWatchEventsQuery(query looppkg.WatchEventsQuery) (normalizedWatchEventsQuery, error) {
	workspaceID := strings.TrimSpace(query.WorkspaceID)
	if workspaceID == "" {
		return normalizedWatchEventsQuery{}, fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	streams := make(map[string]int64, len(query.Streams))
	for stream, cursor := range query.Streams {
		trimmed := strings.TrimSpace(stream)
		switch trimmed {
		case looppkg.WatchEventsTaskStream,
			looppkg.WatchEventsLoopStream,
			looppkg.WatchEventsAutomationStream,
			looppkg.WatchEventsNetworkStream,
			looppkg.WatchEventsObserveStream:
		case "":
			return normalizedWatchEventsQuery{}, fmt.Errorf(
				"%w: watch-events stream is required",
				looppkg.ErrValidation,
			)
		default:
			if _, ok := looppkg.WatchEventsSessionIDFromStream(trimmed); !ok {
				return normalizedWatchEventsQuery{}, fmt.Errorf(
					"%w: watch-events stream is unsupported: %q",
					looppkg.ErrValidation,
					stream,
				)
			}
		}
		if cursor < 0 {
			return normalizedWatchEventsQuery{}, fmt.Errorf(
				"%w: watch-events cursor for %q must be non-negative",
				looppkg.ErrValidation,
				trimmed,
			)
		}
		streams[trimmed] = cursor
	}
	if len(streams) == 0 {
		return normalizedWatchEventsQuery{}, fmt.Errorf("%w: watch-events streams are required", looppkg.ErrValidation)
	}
	kinds := uniqueTrimmedStrings(query.Kinds)
	limit := query.Limit
	if limit <= 0 || limit > looppkg.LoopMaxFanoutWidth {
		limit = looppkg.LoopMaxFanoutWidth
	}
	return normalizedWatchEventsQuery{
		workspaceID: workspaceID,
		streams:     streams,
		kinds:       kinds,
		limit:       limit,
	}, nil
}

func (g *WatchEventsRepo) readWatchEventsCursor(
	ctx context.Context,
	query normalizedWatchEventsQuery,
	stream string,
) (int64, error) {
	if len(query.kinds) == 0 {
		return 0, nil
	}
	// dynamic-sql: task and loop cursors use caller-selected kind sets with variable-width IN lists.
	placeholders, args := sqlInPlaceholders(query.kinds)
	switch stream {
	case looppkg.WatchEventsTaskStream:
		args = append([]any{query.workspaceID}, args...)
		return scanWatchEventCursor(
			g.db.QueryRowContext(
				ctx,
				`SELECT COALESCE(MAX(te.event_seq), 0)
				   FROM task_events te
				   JOIN tasks t ON t.id = te.task_id
				  WHERE COALESCE(t.workspace_id, '') = ?
				    AND te.event_type IN (`+placeholders+`)`,
				args...,
			),
			stream,
		)
	case looppkg.WatchEventsLoopStream:
		args = append([]any{query.workspaceID}, args...)
		return scanWatchEventCursor(
			g.db.QueryRowContext(
				ctx,
				`SELECT COALESCE(MAX(lre.seq), 0)
				   FROM loop_run_events lre
				  WHERE lre.workspace_id = ?
				    AND lre.kind IN (`+placeholders+`)`,
				args...,
			),
			stream,
		)
	case looppkg.WatchEventsAutomationStream:
		return g.readAutomationWatchEventsCursor(ctx, query)
	case looppkg.WatchEventsNetworkStream:
		return g.readNetworkWatchEventsCursor(ctx, query)
	case looppkg.WatchEventsObserveStream:
		return g.readObserveWatchEventsCursor(ctx, query)
	default:
		if _, ok := looppkg.WatchEventsSessionIDFromStream(stream); ok {
			return g.readSessionWatchEventsCursor(ctx, query, stream)
		}
		return 0, fmt.Errorf("%w: watch-events stream is unsupported: %q", looppkg.ErrValidation, stream)
	}
}

func scanWatchEventCursor(row rowScanner, stream string) (int64, error) {
	var cursor int64
	if err := row.Scan(&cursor); err != nil {
		return 0, fmt.Errorf("store: scan watch-events cursor %q: %w", stream, err)
	}
	return cursor, nil
}

func watchEventsCursorFromGenerated(raw any, stream string) (int64, error) {
	switch value := raw.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case []byte:
		return parseWatchEventsCursor(string(value), stream)
	case string:
		return parseWatchEventsCursor(value, stream)
	default:
		return 0, fmt.Errorf("store: scan watch-events cursor %q encoded as %T", stream, raw)
	}
}

func parseWatchEventsCursor(raw string, stream string) (int64, error) {
	cursor, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("store: parse watch-events cursor %q: %w", stream, err)
	}
	return cursor, nil
}

func (g *WatchEventsRepo) readWatchEventsStreamMatches(
	ctx context.Context,
	query normalizedWatchEventsQuery,
	stream string,
) ([]looppkg.WatchEvent, error) {
	if len(query.kinds) == 0 {
		return nil, nil
	}
	switch stream {
	case looppkg.WatchEventsTaskStream:
		return g.readTaskWatchEvents(ctx, query)
	case looppkg.WatchEventsLoopStream:
		return g.readLoopWatchEvents(ctx, query)
	case looppkg.WatchEventsAutomationStream:
		return g.readAutomationWatchEvents(ctx, query)
	case looppkg.WatchEventsNetworkStream:
		return g.readNetworkWatchEvents(ctx, query)
	case looppkg.WatchEventsObserveStream:
		return g.readObserveWatchEvents(ctx, query)
	default:
		if _, ok := looppkg.WatchEventsSessionIDFromStream(stream); ok {
			return g.readSessionWatchEvents(ctx, query, stream)
		}
		return nil, fmt.Errorf("%w: watch-events stream is unsupported: %q", looppkg.ErrValidation, stream)
	}
}

func (g *WatchEventsRepo) readTaskWatchEvents(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) ([]looppkg.WatchEvent, error) {
	placeholders, kindArgs := sqlInPlaceholders(query.kinds)
	args := append([]any{query.workspaceID, query.streams[looppkg.WatchEventsTaskStream]}, kindArgs...)
	args = append(args, query.limit)
	// dynamic-sql: caller-selected task event kinds require a variable-width IN list.
	// #nosec G202 -- IN placeholders are generated from normalized kind count; values are parameterized.
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT
			te.event_seq,
			te.event_type,
			te.task_id,
			COALESCE(te.run_id, ''),
			COALESCE(te.payload_json, '{}'),
			te.timestamp,
			COALESCE(t.workspace_id, ''),
			COALESCE(t.parent_task_id, '')
		   FROM task_events te
		   JOIN tasks t ON t.id = te.task_id
		  WHERE COALESCE(t.workspace_id, '') = ?
		    AND te.event_seq > ?
		    AND te.event_type IN (`+placeholders+`)
		  ORDER BY te.event_seq ASC
		  LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: read task watch-events: %w", err)
	}
	defer rows.Close()

	events := make([]looppkg.WatchEvent, 0)
	for rows.Next() {
		event, scanErr := scanTaskWatchEvent(rows)
		if scanErr != nil {
			return nil, joinRowsCloseError(rows, scanErr, "task watch-events query")
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, joinRowsCloseError(
			rows,
			fmt.Errorf("store: iterate task watch-events: %w", err),
			"task watch-events query",
		)
	}
	if err := joinRowsCloseError(rows, nil, "task watch-events query"); err != nil {
		return nil, err
	}
	return events, nil
}

func scanTaskWatchEvent(row rowScanner) (looppkg.WatchEvent, error) {
	var (
		event        looppkg.WatchEvent
		payloadRaw   string
		timestampRaw string
		parentTaskID string
	)
	if err := row.Scan(
		&event.Seq,
		&event.Kind,
		&event.TaskID,
		&event.RunID,
		&payloadRaw,
		&timestampRaw,
		&event.WorkspaceID,
		&parentTaskID,
	); err != nil {
		return looppkg.WatchEvent{}, fmt.Errorf("store: scan task watch-event: %w", err)
	}
	payload, err := decodeWatchEventPayload(payloadRaw)
	if err != nil {
		return looppkg.WatchEvent{}, err
	}
	if _, ok := payload[watchEventsParentTaskIDPayloadKey]; !ok {
		payload[watchEventsParentTaskIDPayloadKey] = strings.TrimSpace(parentTaskID)
	}
	at, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		return looppkg.WatchEvent{}, fmt.Errorf("store: parse task watch-event timestamp: %w", err)
	}
	event.Stream = looppkg.WatchEventsTaskStream
	event.LedgerKind = event.Kind
	event.Payload = payload
	event.At = formatWatchEventAt(at)
	return event, nil
}

func decodeWatchEventPayload(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(bytes.NewBufferString(trimmed))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("store: decode watch-event payload: %w", err)
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}

func formatWatchEventAt(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func sortedWatchEventStreams(streams map[string]int64) []string {
	keys := make([]string, 0, len(streams))
	for key := range streams {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func uniqueTrimmedStrings(values []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	slices.Sort(unique)
	return unique
}

func sqlInPlaceholders(values []string) (string, []any) {
	placeholders := make([]string, len(values))
	args := make([]any, len(values))
	for idx, value := range values {
		placeholders[idx] = "?"
		args[idx] = value
	}
	return strings.Join(placeholders, ", "), args
}
