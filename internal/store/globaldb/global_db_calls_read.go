package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/listcursor"
	storepkg "github.com/compozy/compozy/internal/store"
)

const (
	callReadCursorVersion = 1
	callReadCursorKind    = "calls"
	messageCursorKind     = "call-messages"
)

type callReadCursor struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

type callReadFingerprint struct {
	ProfileID      string   `json:"profile_id,omitempty"`
	AllProfiles    bool     `json:"all_profiles,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	WorkspaceID    string   `json:"workspace_id,omitempty"`
	States         []string `json:"states,omitempty"`
	Attention      bool     `json:"attention,omitempty"`
	Caller         string   `json:"caller,omitempty"`
	ChildSessionID string   `json:"child_session_id,omitempty"`
	RootSessionID  string   `json:"root_session_id,omitempty"`
	Agent          string   `json:"agent,omitempty"`
	SessionID      string   `json:"session_id,omitempty"`
}

// ListCalls returns a query-bound keyset page ordered by durable creation identity.
func (g *CallRepo) ListCalls(ctx context.Context, query callspkg.CallListQuery) (callspkg.CallPage, error) {
	if err := g.checkReady(ctx, "list calls"); err != nil {
		return callspkg.CallPage{}, err
	}
	if err := query.ReadScope.Validate(); err != nil {
		return callspkg.CallPage{}, fmt.Errorf("store: list calls read scope: %w", err)
	}
	states, err := normalizeCallStates(query.State)
	if err != nil {
		return callspkg.CallPage{}, err
	}
	filters := callReadFingerprint{
		States: states, Attention: query.Attention, Caller: query.Caller, ChildSessionID: query.ChildSessionID,
		RootSessionID: query.RootSessionID, Agent: query.Agent,
	}
	fingerprint, err := callQueryFingerprint(query.CallReadQuery, filters)
	if err != nil {
		return callspkg.CallPage{}, err
	}
	cursor, err := decodeCallReadCursor(query.Cursor, callReadCursorKind, fingerprint)
	if err != nil {
		return callspkg.CallPage{}, err
	}

	where := ` FROM calls WHERE 1 = 1`
	args := make([]any, 0, 12)
	where, args = appendCallReadScope(where, args, "calls", query.CallReadQuery)
	where, args = appendCallListFilters(where, args, query, states)
	var total int
	if err := g.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return callspkg.CallPage{}, fmt.Errorf("store: count calls: %w", err)
	}
	statement := `SELECT ` + callSelectColumnsSQL + where
	if cursor.ID != "" {
		statement += ` AND (created_at < ? OR (created_at = ? AND call_id < ?))`
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	limit := boundedCallReadLimit(query.Limit)
	statement += ` ORDER BY created_at DESC, call_id DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return callspkg.CallPage{}, fmt.Errorf("store: list calls: %w", err)
	}
	items := make([]callspkg.CallRecord, 0, limit+1)
	for rows.Next() {
		record, scanErr := scanCallRecord(rows)
		if scanErr != nil {
			return callspkg.CallPage{}, errors.Join(
				fmt.Errorf("store: scan call page: %w", scanErr),
				rows.Close(),
			)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return callspkg.CallPage{}, errors.Join(
			fmt.Errorf("store: iterate call page: %w", err),
			rows.Close(),
		)
	}
	if err := rows.Close(); err != nil {
		return callspkg.CallPage{}, fmt.Errorf("store: close call page rows: %w", err)
	}
	page := callspkg.CallPage{Items: items, Total: total}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = listcursor.Encode(
			callReadCursorVersion,
			callReadCursorKind,
			fingerprint,
			callReadCursor{CreatedAt: storepkg.FormatTimestamp(last.CreatedAt), ID: last.CallID},
		)
		if err != nil {
			return callspkg.CallPage{}, fmt.Errorf("store: encode calls cursor: %w", err)
		}
	}
	return page, nil
}

// GetCallRead resolves one detail through an exact or explicit aggregate owner boundary.
func (g *CallRepo) GetCallRead(
	ctx context.Context,
	query callspkg.CallReadQuery,
	callID string,
) (callspkg.CallRecord, error) {
	if err := g.checkReady(ctx, "get call read"); err != nil {
		return callspkg.CallRecord{}, err
	}
	if err := query.ReadScope.Validate(); err != nil {
		return callspkg.CallRecord{}, fmt.Errorf("store: get call read scope: %w", err)
	}
	statement := `SELECT ` + callSelectColumnsSQL + ` FROM calls WHERE call_id = ?`
	args := []any{strings.TrimSpace(callID)}
	statement, args = appendCallReadScope(statement, args, "calls", query)
	record, err := scanCallRecord(g.db.QueryRowContext(ctx, statement, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.CallRecord{}, callNotFound(callID)
	}
	if err != nil {
		return callspkg.CallRecord{}, fmt.Errorf("store: get call read %q: %w", callID, err)
	}
	return record, nil
}

// ListMessages returns a query-bound keyset page with the latest durable delivery projection.
func (g *CallRepo) ListMessages(
	ctx context.Context,
	query callspkg.MessageListQuery,
) (callspkg.MessagePage, error) {
	if err := g.checkReady(ctx, "list call messages"); err != nil {
		return callspkg.MessagePage{}, err
	}
	if err := query.ReadScope.Validate(); err != nil {
		return callspkg.MessagePage{}, fmt.Errorf("store: list messages read scope: %w", err)
	}
	fingerprint, err := callQueryFingerprint(query.CallReadQuery, callReadFingerprint{SessionID: query.SessionID})
	if err != nil {
		return callspkg.MessagePage{}, err
	}
	cursor, err := decodeCallReadCursor(query.Cursor, messageCursorKind, fingerprint)
	if err != nil {
		return callspkg.MessagePage{}, err
	}
	statement := `SELECT message.message_id, message.profile_id, message.scope, message.workspace_id,
		message.from_kind, message.from_id, COALESCE(sender.agent_name, ''), message.to_session_id,
		COALESCE(message.call_id, ''), message.body, message.dedup_hash, message.created_at,
		delivery.state, delivery.reason, delivery.attempts, delivery.delivered_at
		FROM call_messages message
		JOIN call_deliveries delivery ON delivery.kind = 'message' AND delivery.subject_id = message.message_id
		LEFT JOIN sessions sender ON message.from_kind = 'session' AND sender.id = message.from_id
		WHERE 1 = 1`
	args := make([]any, 0, 10)
	statement, args = appendCallReadScope(statement, args, "message", query.CallReadQuery)
	if sessionID := strings.TrimSpace(query.SessionID); sessionID != "" {
		statement += ` AND (message.from_id = ? OR message.to_session_id = ?)`
		args = append(args, sessionID, sessionID)
	}
	if cursor.ID != "" {
		statement += ` AND (message.created_at < ? OR (message.created_at = ? AND message.message_id < ?))`
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	limit := boundedCallReadLimit(query.Limit)
	statement += ` ORDER BY message.created_at DESC, message.message_id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return callspkg.MessagePage{}, fmt.Errorf("store: list call messages: %w", err)
	}
	items := make([]callspkg.MessageRecord, 0, limit+1)
	for rows.Next() {
		record, scanErr := scanCallMessage(rows)
		if scanErr != nil {
			return callspkg.MessagePage{}, errors.Join(
				fmt.Errorf("store: scan message page: %w", scanErr),
				rows.Close(),
			)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return callspkg.MessagePage{}, errors.Join(
			fmt.Errorf("store: iterate message page: %w", err),
			rows.Close(),
		)
	}
	if err := rows.Close(); err != nil {
		return callspkg.MessagePage{}, fmt.Errorf("store: close message page rows: %w", err)
	}
	page := callspkg.MessagePage{Items: items}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor, err = listcursor.Encode(
			callReadCursorVersion,
			messageCursorKind,
			fingerprint,
			callReadCursor{CreatedAt: storepkg.FormatTimestamp(last.CreatedAt), ID: last.MessageID},
		)
		if err != nil {
			return callspkg.MessagePage{}, fmt.Errorf("store: encode messages cursor: %w", err)
		}
	}
	return page, nil
}

func appendCallReadScope(
	statement string,
	args []any,
	alias string,
	query callspkg.CallReadQuery,
) (string, []any) {
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	if !query.ReadScope.AllProfiles {
		statement += ` AND ` + prefix + `profile_id = ?`
		args = append(args, strings.TrimSpace(query.ReadScope.ProfileID))
	}
	if query.Scope != "" {
		statement += ` AND ` + prefix + `scope = ?`
		args = append(args, string(query.Scope))
	}
	if workspaceID := strings.TrimSpace(query.WorkspaceID); workspaceID != "" {
		statement += ` AND ` + prefix + `workspace_id = ?`
		args = append(args, workspaceID)
	}
	return statement, args
}

func appendCallListFilters(
	statement string,
	args []any,
	query callspkg.CallListQuery,
	states []string,
) (string, []any) {
	if len(states) > 0 {
		statement += ` AND state IN (` + callReadPlaceholders(len(states)) + `)`
		for _, state := range states {
			args = append(args, state)
		}
	}
	if query.Attention {
		statement += ` AND state IN ('invalid-result', 'completed-without-result')
			AND child_session_id <> ''
			AND NOT EXISTS (
				SELECT 1 FROM calls newer
				WHERE newer.profile_id = calls.profile_id
					AND newer.scope = calls.scope
					AND newer.workspace_id = calls.workspace_id
					AND newer.child_session_id = calls.child_session_id
					AND newer.call_id <> calls.call_id
					AND newer.created_at > COALESCE(calls.settled_at, calls.updated_at)
			)
			AND NOT EXISTS (
				SELECT 1 FROM call_messages resolution
				WHERE resolution.profile_id = calls.profile_id
					AND resolution.scope = calls.scope
					AND resolution.workspace_id = calls.workspace_id
					AND resolution.to_session_id = calls.child_session_id
					AND resolution.created_at > COALESCE(calls.settled_at, calls.updated_at)
			)`
	}
	filters := []struct {
		column string
		value  string
	}{
		{column: "caller_id", value: query.Caller},
		{column: "child_session_id", value: query.ChildSessionID},
		{column: "governed_root_id", value: query.RootSessionID},
		{column: "agent_name", value: query.Agent},
	}
	for _, filter := range filters {
		if value := strings.TrimSpace(filter.value); value != "" {
			statement += ` AND ` + filter.column + ` = ?`
			args = append(args, value)
		}
	}
	return statement, args
}

func normalizeCallStates(values []callspkg.State) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	states := make([]string, 0, len(values))
	for _, value := range values {
		state := callspkg.State(strings.TrimSpace(string(value)))
		if state == "" {
			continue
		}
		if state != callspkg.StateQueued && state != callspkg.StateRunning && !state.Terminal() {
			return nil, &callspkg.Error{Code: callspkg.CodeValidation, Message: fmt.Sprintf("unknown call state %q", state)}
		}
		if _, exists := seen[string(state)]; exists {
			continue
		}
		seen[string(state)] = struct{}{}
		states = append(states, string(state))
	}
	slices.Sort(states)
	return states, nil
}

func callQueryFingerprint(
	query callspkg.CallReadQuery,
	filters callReadFingerprint,
) (string, error) {
	filters.ProfileID = strings.TrimSpace(query.ReadScope.ProfileID)
	filters.AllProfiles = query.ReadScope.AllProfiles
	filters.Scope = string(query.Scope)
	filters.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	filters.States = append([]string(nil), filters.States...)
	filters.Caller = strings.TrimSpace(filters.Caller)
	filters.ChildSessionID = strings.TrimSpace(filters.ChildSessionID)
	filters.RootSessionID = strings.TrimSpace(filters.RootSessionID)
	filters.Agent = strings.TrimSpace(filters.Agent)
	filters.SessionID = strings.TrimSpace(filters.SessionID)
	fingerprint, err := listcursor.Fingerprint(filters)
	if err != nil {
		return "", fmt.Errorf("store: fingerprint calls query: %w", err)
	}
	return fingerprint, nil
}

func decodeCallReadCursor(raw string, kind string, fingerprint string) (callReadCursor, error) {
	if strings.TrimSpace(raw) == "" {
		return callReadCursor{}, nil
	}
	position, err := listcursor.Decode[callReadCursor](
		strings.TrimSpace(raw),
		callReadCursorVersion,
		kind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil || strings.TrimSpace(position.CreatedAt) == "" || strings.TrimSpace(position.ID) == "" {
		if err == nil {
			err = errors.New("cursor position is incomplete")
		}
		return callReadCursor{}, &callspkg.Error{
			Code: callspkg.CodeValidation, Message: "cursor does not match this calls query", Cause: err,
		}
	}
	return position, nil
}

func boundedCallReadLimit(limit int) int {
	if limit <= 0 {
		return callspkg.DefaultReadLimit
	}
	if limit > callspkg.MaxReadLimit {
		return callspkg.MaxReadLimit
	}
	return limit
}

func callReadPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}
