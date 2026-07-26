package sessiondb

import (
	"database/sql"
	"errors"
	"fmt"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

func (s *SessionDB) scanHookRunRecords(
	rows *sql.Rows,
) (records []hookspkg.HookRunRecord, err error) {
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close hook run rows: %w", closeErr))
		}
	}()
	records = make([]hookspkg.HookRunRecord, 0)
	for rows.Next() {
		record, scanErr := s.scanHookRunRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("store: iterate hook runs: %w", rowsErr)
	}
	return records, nil
}

func (s *SessionDB) scanSessionEvents(
	rows *sql.Rows,
) (events []store.SessionEvent, err error) {
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close session event rows: %w", closeErr))
		}
	}()
	events = make([]store.SessionEvent, 0)
	for rows.Next() {
		event, scanErr := s.scanSessionEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("store: iterate session events: %w", rowsErr)
	}
	return events, nil
}
