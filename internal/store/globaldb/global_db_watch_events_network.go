package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
)

type networkWatchEventRow struct {
	seq         int64
	message     store.NetworkConversationMessage
	at          time.Time
	workOutcome networkWorkOutcome
}

type networkWorkOutcome struct {
	opened       bool
	transitioned bool
	state        string
}

func (g *WatchEventsRepo) readNetworkWatchEventsCursor(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) (int64, error) {
	if _, ok := stringSet(query.kinds)[string(hookspkg.HookNetworkMessagePersisted)]; ok {
		scopeSQL, scopeArgs := networkWatchEventsScopeClause(query.readScope)
		args := append([]any{query.workspaceID}, scopeArgs...)
		return scanWatchEventCursor(
			g.db.QueryRowContext(
				ctx,
				`SELECT COALESCE(MAX(ntl.sequence), 0)
				 `+networkWatchEventsOwnerFromSQL+`
				 WHERE ntl.workspace_id = ?
				   AND `+scopeSQL,
				args...,
			),
			looppkg.WatchEventsNetworkStream,
		)
	}
	return g.readProjectedNetworkWatchEventsCursor(ctx, query)
}

func (g *WatchEventsRepo) readProjectedNetworkWatchEventsCursor(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) (int64, error) {
	after := query.streams[looppkg.WatchEventsNetworkStream]
	rows, ok, err := g.readNetworkWatchEventRows(ctx, query, after)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	cursor := after
	for rows.Next() {
		row, scanErr := scanNetworkWatchEventRow(rows)
		if scanErr != nil {
			return 0, joinRowsCloseError(rows, scanErr, "network watch-events cursor query")
		}
		events, eventErr := g.networkWatchEventsForRow(ctx, &row, query.kinds)
		if eventErr != nil {
			return 0, joinRowsCloseError(rows, eventErr, "network watch-events cursor query")
		}
		if len(events) > 0 && row.seq > cursor {
			cursor = row.seq
		}
	}
	if err := rows.Err(); err != nil {
		return 0, joinRowsCloseError(
			rows,
			fmt.Errorf("store: iterate network watch-events cursor: %w", err),
			"network watch-events cursor query",
		)
	}
	if err := joinRowsCloseError(rows, nil, "network watch-events cursor query"); err != nil {
		return 0, err
	}
	return cursor, nil
}

func (g *WatchEventsRepo) readNetworkWatchEvents(
	ctx context.Context,
	query normalizedWatchEventsQuery,
) ([]looppkg.WatchEvent, error) {
	rows, ok, err := g.readNetworkWatchEventRows(ctx, query, query.streams[looppkg.WatchEventsNetworkStream])
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	events := make([]looppkg.WatchEvent, 0)
	for rows.Next() {
		row, scanErr := scanNetworkWatchEventRow(rows)
		if scanErr != nil {
			return nil, joinRowsCloseError(rows, scanErr, "network watch-events query")
		}
		rowEvents, eventErr := g.networkWatchEventsForRow(ctx, &row, query.kinds)
		if eventErr != nil {
			return nil, joinRowsCloseError(rows, eventErr, "network watch-events query")
		}
		if len(rowEvents) == 0 {
			continue
		}
		events = append(events, rowEvents...)
		if len(events) >= query.limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, joinRowsCloseError(
			rows,
			fmt.Errorf("store: iterate network watch-events: %w", err),
			"network watch-events query",
		)
	}
	if err := joinRowsCloseError(rows, nil, "network watch-events query"); err != nil {
		return nil, err
	}
	return events, nil
}

func (g *WatchEventsRepo) readNetworkWatchEventRows(
	ctx context.Context,
	query normalizedWatchEventsQuery,
	after int64,
) (*sql.Rows, bool, error) {
	predicate, ok := networkWatchEventsCandidatePredicate(query.kinds)
	if !ok {
		return nil, false, nil
	}
	scopeSQL, scopeArgs := networkWatchEventsScopeClause(query.readScope)
	args := append([]any{query.workspaceID}, scopeArgs...)
	args = append(args, after)
	// #nosec G202 -- predicates and owner joins are fixed SQL; scope and values are parameterized.
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT
			ntl.sequence,
			ntl.message_id,
			COALESCE(ntl.session_id, ''),
			ntl.workspace_id,
			ntl.channel,
			COALESCE(ntl.surface, ''),
			COALESCE(ntl.thread_id, ''),
			COALESCE(ntl.direct_id, ''),
			ntl.direction,
			ntl.peer_from,
			COALESCE(ntl.peer_to, ''),
			ntl.kind,
			COALESCE(ntl.work_id, ''),
			COALESCE(ntl.reply_to, ''),
			COALESCE(ntl.trace_id, ''),
			COALESCE(ntl.causation_id, ''),
			COALESCE(ntl.intent, ''),
			COALESCE(ntl.text, ''),
			ntl.body_json,
			ntl.timestamp,
			ntl.work_opened,
			ntl.work_transitioned,
			ntl.work_state
		   `+networkWatchEventsOwnerFromSQL+`
		  WHERE ntl.workspace_id = ?
		    AND `+scopeSQL+`
		    AND ntl.sequence > ?
		    AND (`+predicate+`)
		  ORDER BY ntl.sequence ASC`,
		args...,
	)
	if err != nil {
		return nil, false, fmt.Errorf("store: read network watch-events: %w", err)
	}
	return rows, true, nil
}

const networkWatchEventsOwnerFromSQL = `FROM network_timeline_log ntl
	LEFT JOIN network_threads AS owner_thread
		ON owner_thread.workspace_id = ntl.workspace_id
		AND owner_thread.channel = ntl.channel
		AND owner_thread.thread_id = ntl.thread_id
	LEFT JOIN network_direct_rooms AS owner_direct
		ON owner_direct.workspace_id = ntl.workspace_id
		AND owner_direct.channel = ntl.channel
		AND owner_direct.direct_id = ntl.direct_id
	JOIN network_channels AS owner_channel
		ON owner_channel.workspace_id = ntl.workspace_id
		AND owner_channel.channel = ntl.channel`

func networkWatchEventsScopeClause(scope store.ReadScope) (string, []any) {
	clauses, args := store.BuildClauses(store.ReadScopeCoalescedClause(
		[]string{"owner_thread.profile_id", "owner_direct.profile_id", "owner_channel.profile_id"},
		scope,
	))
	if len(clauses) == 0 {
		return watchEventsAllRowsSQL, nil
	}
	return clauses[0], args
}

func scanNetworkWatchEventRow(row rowScanner) (networkWatchEventRow, error) {
	var (
		event            networkWatchEventRow
		body             string
		atRaw            string
		workOpened       int
		workTransitioned int
	)
	if err := row.Scan(
		&event.seq,
		&event.message.MessageID,
		&event.message.SessionID,
		&event.message.WorkspaceID,
		&event.message.Channel,
		&event.message.Surface,
		&event.message.ThreadID,
		&event.message.DirectID,
		&event.message.Direction,
		&event.message.PeerFrom,
		&event.message.PeerTo,
		&event.message.Kind,
		&event.message.WorkID,
		&event.message.ReplyTo,
		&event.message.TraceID,
		&event.message.CausationID,
		&event.message.Intent,
		&event.message.Text,
		&body,
		&atRaw,
		&workOpened,
		&workTransitioned,
		&event.workOutcome.state,
	); err != nil {
		return networkWatchEventRow{}, fmt.Errorf("store: scan network watch-event: %w", err)
	}
	at, err := store.ParseTimestamp(atRaw)
	if err != nil {
		return networkWatchEventRow{}, fmt.Errorf("store: parse network watch-event timestamp: %w", err)
	}
	event.message.Body = json.RawMessage(body)
	event.message.Sequence = event.seq
	event.message.Timestamp = at
	event.at = at
	event.workOutcome.opened = workOpened != 0
	event.workOutcome.transitioned = workTransitioned != 0
	return event, nil
}

func networkWatchEventsCandidatePredicate(kinds []string) (string, bool) {
	kindSet := stringSet(kinds)
	if _, ok := kindSet[string(hookspkg.HookNetworkMessagePersisted)]; ok {
		return watchEventsAllRowsSQL, true
	}
	predicates := make([]string, 0, 3)
	if _, ok := kindSet[string(hookspkg.HookNetworkThreadOpened)]; ok {
		predicates = append(predicates, "ntl.surface = 'thread'")
	}
	if _, ok := kindSet[string(hookspkg.HookNetworkDirectRoomOpened)]; ok {
		predicates = append(predicates, "ntl.surface = 'direct'")
	}
	if networkWorkKindRequested(kindSet) {
		predicates = append(predicates, "COALESCE(ntl.work_id, '') <> ''")
	}
	if len(predicates) == 0 {
		return "", false
	}
	return strings.Join(predicates, " OR "), true
}

func (g *WatchEventsRepo) networkWatchEventsForRow(
	ctx context.Context,
	row *networkWatchEventRow,
	kinds []string,
) ([]looppkg.WatchEvent, error) {
	kindSet := stringSet(kinds)
	events := make([]looppkg.WatchEvent, 0, 5)
	workState := ""
	if _, ok := kindSet[string(hookspkg.HookNetworkMessagePersisted)]; ok {
		if strings.TrimSpace(row.message.WorkID) != "" {
			workState = row.workOutcome.state
		}
		events = append(events, networkWatchEventFromRow(row, hookspkg.HookNetworkMessagePersisted, workState))
	}
	if _, ok := kindSet[string(hookspkg.HookNetworkThreadOpened)]; ok {
		opened, err := g.networkConversationOpenedAtTimelineRow(ctx, row, store.NetworkSurfaceThread)
		if err != nil {
			return nil, err
		}
		if opened {
			events = append(events, networkWatchEventFromRow(row, hookspkg.HookNetworkThreadOpened, ""))
		}
	}
	if _, ok := kindSet[string(hookspkg.HookNetworkDirectRoomOpened)]; ok {
		opened, err := g.networkConversationOpenedAtTimelineRow(ctx, row, store.NetworkSurfaceDirect)
		if err != nil {
			return nil, err
		}
		if opened {
			events = append(events, networkWatchEventFromRow(row, hookspkg.HookNetworkDirectRoomOpened, ""))
		}
	}
	if networkWorkKindRequested(kindSet) {
		outcome := row.workOutcome
		if outcome.opened {
			appendRequestedNetworkWorkEvent(&events, row, kindSet, hookspkg.HookNetworkWorkOpened, outcome.state)
		}
		if outcome.transitioned {
			appendRequestedNetworkWorkEvent(&events, row, kindSet, hookspkg.HookNetworkWorkTransitioned, outcome.state)
		}
		if (outcome.opened || outcome.transitioned) && networkWorkStateIsTerminal(outcome.state) {
			appendRequestedNetworkWorkEvent(&events, row, kindSet, hookspkg.HookNetworkWorkClosed, outcome.state)
		}
	}
	return events, nil
}
