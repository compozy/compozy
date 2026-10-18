package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

var _ store.EventMetadataReader = (*SessionDB)(nil)

func (s *SessionDB) QueryEventMetadata(ctx context.Context, query store.EventQuery) ([]store.EventMetadata, error) {
	if s == nil {
		return nil, errors.New("store: session database is required")
	}
	if ctx == nil {
		return nil, errors.New("store: query session event metadata context is required")
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return nil, store.ErrClosed
	}
	return queryEventMetadata(ctx, s.db, s.owner.SessionID, query)
}

func (s *SessionDB) MaxEventSequence(ctx context.Context) (int64, error) {
	if s == nil {
		return 0, errors.New("store: session database is required")
	}
	if ctx == nil {
		return 0, errors.New("store: query max event sequence context is required")
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return 0, store.ErrClosed
	}
	return currentMaxSequence(ctx, s.db)
}

func queryEventMetadata(
	ctx context.Context,
	db *sql.DB,
	sessionID string,
	query store.EventQuery,
) (events []store.EventMetadata, err error) {
	sqlQuery, args, err := buildEventQuerySQL(sessionEventMetadataColumns, query)
	if err != nil {
		return nil, err
	}
	// dynamic-sql: buildEventQuerySQL composes the existing metadata projection, filters, and cursor bounds.
	rows, err := db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query session event metadata: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			joinSessionCleanupError(&err, fmt.Errorf("store: close session event metadata rows: %w", closeErr))
		}
	}()
	events = make([]store.EventMetadata, 0)
	for rows.Next() {
		event, scanErr := scanEventMetadata(rows, sessionID)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate session event metadata: %w", err)
	}
	return events, nil
}

func scanEventMetadata(row rowScanner, sessionID string) (store.EventMetadata, error) {
	var event store.EventMetadata
	var timestampRaw string
	if err := row.Scan(
		&event.ID, &event.Sequence, &event.TurnID, &event.Type, &event.AgentName, &timestampRaw,
	); err != nil {
		return store.EventMetadata{}, fmt.Errorf("store: scan session event metadata: %w", err)
	}
	timestamp, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		return store.EventMetadata{}, err
	}
	event.SessionID = sessionID
	event.Timestamp = timestamp
	return event, nil
}
