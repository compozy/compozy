package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
)

const sessionEventsOrderASCClause = " ORDER BY sequence ASC"

// Query returns events filtered by the supplied options.
func (s *SessionDB) Query(ctx context.Context, query store.EventQuery) ([]store.SessionEvent, error) {
	if s == nil {
		return nil, errors.New("store: session database is required")
	}
	if ctx == nil {
		return nil, errors.New("store: query events context is required")
	}
	sqlQuery, args, err := buildEventQuerySQL(sessionEventColumns, query)
	if err != nil {
		return nil, err
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return nil, store.ErrClosed
	}

	// dynamic-sql: buildEventQuerySQL composes optional filters, cursor bounds, and recent-window limiting.
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query session events: %w", err)
	}
	return s.scanSessionEvents(rows)
}

// History returns ordered session events grouped by turn id.
func (s *SessionDB) History(ctx context.Context, query store.EventQuery) ([]store.TurnHistory, error) {
	return queryTurnHistory(ctx, query, s.Query)
}

func (s *SessionDB) scanSessionEvent(scanner rowScanner) (store.SessionEvent, error) {
	var (
		event     store.SessionEvent
		timestamp string
	)
	if err := scanner.Scan(
		&event.ID,
		&event.Sequence,
		&event.TurnID,
		&event.Type,
		&event.AgentName,
		&event.Content,
		&event.Archived,
		&timestamp,
	); err != nil {
		return store.SessionEvent{}, fmt.Errorf("store: scan session event: %w", err)
	}

	parsed, err := store.ParseTimestamp(timestamp)
	if err != nil {
		return store.SessionEvent{}, err
	}
	event.Timestamp = parsed
	event.SessionID = s.sessionID
	return event, nil
}

func currentMaxSequence(ctx context.Context, db *sql.DB) (int64, error) {
	return sqlcgen.New(db).MaxEventSequence(ctx)
}

func nextEventSequence(ctx context.Context, tx *store.WriteTx) (int64, error) {
	current, err := sqlcgen.New(tx).MaxEventSequence(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: query next session event sequence: %w", err)
	}
	return current + 1, nil
}
