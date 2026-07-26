package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	modelpkg "github.com/compozy/agh/internal/automation/model"
	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
)

type automationWatchEventRow struct {
	seq         int64
	runID       string
	jobID       string
	triggerID   string
	sessionID   string
	status      string
	attempt     int
	startedAt   sql.NullString
	endedAt     sql.NullString
	errorText   string
	agentName   string
	workspaceID string
	retryRaw    string
}

func (g *WatchEventsRepo) readAutomationWatchEventsCursor(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) (int64, error) {
	statuses := automationWatchStatusesForKinds(query.kinds)
	if len(statuses) == 0 {
		return 0, nil
	}
	// dynamic-sql: requested automation hooks map to a variable-width status set.
	placeholders, statusArgs := sqlInPlaceholders(statuses)
	args := append([]any{query.workspaceID}, statusArgs...)
	// #nosec G202 -- IN placeholders are generated from normalized status count; values are parameterized.
	return scanWatchEventCursor(
		g.db.QueryRowContext(
			ctx,
			`SELECT COALESCE(MAX(awe.seq), 0)
			   FROM automation_watch_events awe
			  WHERE awe.workspace_id = ?
			    AND awe.status IN (`+placeholders+`)`,
			args...,
		),
		looppkg.WatchEventsAutomationStream,
	)
}

func (g *WatchEventsRepo) readAutomationWatchEvents(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) ([]looppkg.WatchEvent, error) {
	statuses := automationWatchStatusesForKinds(query.kinds)
	if len(statuses) == 0 {
		return nil, nil
	}
	// dynamic-sql: requested automation hooks map to a variable-width status set.
	placeholders, statusArgs := sqlInPlaceholders(statuses)
	args := append([]any{
		query.workspaceID,
		query.streams[looppkg.WatchEventsAutomationStream],
	}, statusArgs...)
	args = append(args, query.limit)
	// #nosec G202 -- IN placeholders are generated from normalized status count; values are parameterized.
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT
			awe.seq,
			awe.run_id,
			awe.job_id,
			awe.trigger_id,
			awe.session_id,
			awe.status,
			awe.attempt,
			awe.started_at,
			awe.ended_at,
			awe.error,
			awe.agent_name,
			awe.workspace_id,
			awe.retry_json
		   FROM automation_watch_events awe
		  WHERE awe.workspace_id = ?
		    AND awe.seq > ?
		    AND awe.status IN (`+placeholders+`)
		  ORDER BY awe.seq ASC
		  LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("store: read automation watch-events: %w", err)
	}
	defer rows.Close()

	events := make([]looppkg.WatchEvent, 0)
	for rows.Next() {
		row, scanErr := scanAutomationWatchEventRow(rows)
		if scanErr != nil {
			return nil, joinRowsCloseError(rows, scanErr, "automation watch-events query")
		}
		event, eventErr := automationWatchEventFromRow(row)
		if eventErr != nil {
			return nil, joinRowsCloseError(rows, eventErr, "automation watch-events query")
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, joinRowsCloseError(
			rows,
			fmt.Errorf("store: iterate automation watch-events: %w", err),
			"automation watch-events query",
		)
	}
	if err := joinRowsCloseError(rows, nil, "automation watch-events query"); err != nil {
		return nil, err
	}
	return events, nil
}

func scanAutomationWatchEventRow(row rowScanner) (automationWatchEventRow, error) {
	var event automationWatchEventRow
	if err := row.Scan(
		&event.seq,
		&event.runID,
		&event.jobID,
		&event.triggerID,
		&event.sessionID,
		&event.status,
		&event.attempt,
		&event.startedAt,
		&event.endedAt,
		&event.errorText,
		&event.agentName,
		&event.workspaceID,
		&event.retryRaw,
	); err != nil {
		return automationWatchEventRow{}, fmt.Errorf("store: scan automation watch-event: %w", err)
	}
	return event, nil
}

func automationWatchEventFromRow(row automationWatchEventRow) (looppkg.WatchEvent, error) {
	kind, ok := automationWatchKindForStatus(row.status)
	if !ok {
		return looppkg.WatchEvent{}, fmt.Errorf("store: unsupported automation watch-events status %q", row.status)
	}
	at, err := automationWatchEventAt(row)
	if err != nil {
		return looppkg.WatchEvent{}, err
	}
	payload := map[string]any{
		watchEventsPayloadJobIDKey:     strings.TrimSpace(row.jobID),
		watchEventsPayloadTriggerIDKey: strings.TrimSpace(row.triggerID),
		watchEventsPayloadAgentNameKey: strings.TrimSpace(row.agentName),
		watchEventsPayloadSessionIDKey: strings.TrimSpace(row.sessionID),
		watchEventsPayloadAttemptKey:   row.attempt,
	}
	if kind == hookspkg.HookAutomationRunCompleted {
		durationMS, durationErr := automationWatchEventDurationMS(row)
		if durationErr != nil {
			return looppkg.WatchEvent{}, durationErr
		}
		payload[watchEventsPayloadDurationMSKey] = durationMS
	} else {
		payload[watchEventsPayloadErrorKey] = strings.TrimSpace(row.errorText)
		willRetry, retryErr := automationWatchEventWillRetry(row)
		if retryErr != nil {
			return looppkg.WatchEvent{}, retryErr
		}
		payload[watchEventsPayloadWillRetryKey] = willRetry
	}
	return looppkg.WatchEvent{
		Kind:        string(kind),
		Seq:         row.seq,
		Stream:      looppkg.WatchEventsAutomationStream,
		At:          formatWatchEventAt(at),
		WorkspaceID: strings.TrimSpace(row.workspaceID),
		RunID:       strings.TrimSpace(row.runID),
		SessionID:   strings.TrimSpace(row.sessionID),
		Payload:     payload,
		LedgerKind:  string(kind),
	}, nil
}

func automationWatchStatusesForKinds(kinds []string) []string {
	statuses := make([]string, 0, 2)
	for _, kind := range kinds {
		switch strings.TrimSpace(kind) {
		case string(hookspkg.HookAutomationRunCompleted):
			statuses = append(statuses, string(modelpkg.RunCompleted))
		case string(hookspkg.HookAutomationRunFailed):
			statuses = append(statuses, string(modelpkg.RunFailed))
		}
	}
	return uniqueTrimmedStrings(statuses)
}

func automationWatchKindForStatus(status string) (hookspkg.HookEvent, bool) {
	switch modelpkg.RunStatus(strings.TrimSpace(status)) {
	case modelpkg.RunCompleted:
		return hookspkg.HookAutomationRunCompleted, true
	case modelpkg.RunFailed:
		return hookspkg.HookAutomationRunFailed, true
	default:
		return "", false
	}
}

func automationWatchEventAt(row automationWatchEventRow) (time.Time, error) {
	for _, raw := range []sql.NullString{row.endedAt, row.startedAt} {
		if !raw.Valid || strings.TrimSpace(raw.String) == "" {
			continue
		}
		parsed, err := store.ParseTimestamp(raw.String)
		if err != nil {
			return time.Time{}, fmt.Errorf("store: parse automation watch-event timestamp: %w", err)
		}
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("store: automation watch-event %q has no timestamp", row.runID)
}

func automationWatchEventDurationMS(row automationWatchEventRow) (int64, error) {
	if !row.startedAt.Valid || !row.endedAt.Valid ||
		strings.TrimSpace(row.startedAt.String) == "" ||
		strings.TrimSpace(row.endedAt.String) == "" {
		return 0, nil
	}
	started, err := store.ParseTimestamp(row.startedAt.String)
	if err != nil {
		return 0, fmt.Errorf("store: parse automation watch-event started_at: %w", err)
	}
	ended, err := store.ParseTimestamp(row.endedAt.String)
	if err != nil {
		return 0, fmt.Errorf("store: parse automation watch-event ended_at: %w", err)
	}
	return ended.UTC().Sub(started.UTC()).Milliseconds(), nil
}

func automationWatchEventWillRetry(row automationWatchEventRow) (bool, error) {
	var retry modelpkg.RetryConfig
	if err := decodeAutomationJSON(row.retryRaw, &retry, "automation.watch_events.retry"); err != nil {
		return false, err
	}
	return retry.Strategy == modelpkg.RetryStrategyBackoff && row.attempt <= retry.MaxRetries, nil
}
