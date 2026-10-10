package sessiondb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

var (
	_ transcript.Reader = (*SessionDB)(nil)
	_ transcript.Reader = (*ReadOnlySessionDB)(nil)
)

// TranscriptPage returns one bounded materialized transcript window.
func (s *SessionDB) TranscriptPage(
	ctx context.Context,
	query transcript.PageQuery,
) (transcript.Page, error) {
	if s == nil || s.db == nil {
		return transcript.Page{}, errors.New("store: session database is required")
	}
	if ctx == nil {
		return transcript.Page{}, errors.New("store: transcript page context is required")
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return transcript.Page{}, store.ErrClosed
	}
	return queryTranscriptPage(ctx, s.db, query)
}

// TranscriptChanges returns entries changed by events after the supplied cursor.
func (s *SessionDB) TranscriptChanges(
	ctx context.Context,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	if s == nil || s.db == nil {
		return transcript.ChangePage{}, errors.New("store: session database is required")
	}
	if ctx == nil {
		return transcript.ChangePage{}, errors.New("store: transcript changes context is required")
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return transcript.ChangePage{}, store.ErrClosed
	}
	return queryTranscriptChanges(ctx, s.db, query)
}

// TranscriptPage returns one bounded materialized transcript window.
func (s *ReadOnlySessionDB) TranscriptPage(
	ctx context.Context,
	query transcript.PageQuery,
) (transcript.Page, error) {
	if s == nil || s.db == nil {
		return transcript.Page{}, errors.New("store: read-only session database is required")
	}
	if ctx == nil {
		return transcript.Page{}, errors.New("store: read-only transcript page context is required")
	}
	return queryTranscriptPage(ctx, s.db, query)
}

// TranscriptChanges returns entries changed by events after the supplied cursor.
func (s *ReadOnlySessionDB) TranscriptChanges(
	ctx context.Context,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	if s == nil || s.db == nil {
		return transcript.ChangePage{}, errors.New("store: read-only session database is required")
	}
	if ctx == nil {
		return transcript.ChangePage{}, errors.New("store: read-only transcript changes context is required")
	}
	return queryTranscriptChanges(ctx, s.db, query)
}

func queryTranscriptPage(
	ctx context.Context,
	db *sql.DB,
	query transcript.PageQuery,
) (page transcript.Page, err error) {
	query, err = query.Normalize()
	if err != nil {
		return transcript.Page{}, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return transcript.Page{}, fmt.Errorf("store: begin transcript page read: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			joinSessionCleanupError(&err, fmt.Errorf("store: close transcript page read: %w", rollbackErr))
		}
	}()
	state, err := loadProjectionState(ctx, tx)
	if err != nil {
		return transcript.Page{}, err
	}
	page.Generation = state.Generation
	page.MaxSequence, err = sqlcgen.New(tx).MaxActiveEventSequence(ctx)
	if err != nil {
		return transcript.Page{}, fmt.Errorf("store: query transcript max sequence: %w", err)
	}

	entries, err := queryMaterializedTranscriptPage(ctx, tx, query)
	if err != nil {
		return transcript.Page{}, fmt.Errorf("store: query transcript page: %w", err)
	}
	if len(entries) > query.Limit {
		page.HasOlder = true
		entries = entries[:query.Limit]
	}
	slices.Reverse(entries)
	page.Entries = entries
	if page.HasOlder && len(entries) > 0 {
		page.NextBeforeSequence = entries[0].StartSequence
	}
	if err := tx.Commit(); err != nil {
		return transcript.Page{}, fmt.Errorf("store: commit transcript page read: %w", err)
	}
	return page, nil
}

func queryMaterializedTranscriptPage(
	ctx context.Context,
	tx *sql.Tx,
	query transcript.PageQuery,
) ([]transcript.Entry, error) {
	queries := sqlcgen.New(tx)
	if query.BeforeSequence == 0 {
		rows, err := queries.ListLatestTranscriptEntries(ctx, int64(query.Limit+1))
		if err != nil {
			return nil, err
		}
		entries := make([]transcript.Entry, 0, len(rows))
		for _, row := range rows {
			entry, mapErr := materializedTranscriptEntry(
				row.MessageJson,
				row.StartSequence,
				row.UpdatedSequence,
				row.EventType,
				row.MarkerJson,
			)
			if mapErr != nil {
				return nil, mapErr
			}
			entries = append(entries, entry)
		}
		return entries, nil
	}
	rows, err := queries.ListTranscriptEntriesBefore(ctx, sqlcgen.ListTranscriptEntriesBeforeParams{
		BeforeSequence: query.BeforeSequence,
		RowLimit:       int64(query.Limit + 1),
	})
	if err != nil {
		return nil, err
	}
	entries := make([]transcript.Entry, 0, len(rows))
	for _, row := range rows {
		entry, mapErr := materializedTranscriptEntry(
			row.MessageJson,
			row.StartSequence,
			row.UpdatedSequence,
			row.EventType,
			row.MarkerJson,
		)
		if mapErr != nil {
			return nil, mapErr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func queryTranscriptChanges(
	ctx context.Context,
	db *sql.DB,
	query transcript.ChangeQuery,
) (page transcript.ChangePage, err error) {
	query, err = query.Normalize()
	if err != nil {
		return transcript.ChangePage{}, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return transcript.ChangePage{}, fmt.Errorf("store: begin transcript changes read: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			joinSessionCleanupError(&err, fmt.Errorf("store: close transcript changes read: %w", rollbackErr))
		}
	}()
	state, err := loadProjectionState(ctx, tx)
	if err != nil {
		return transcript.ChangePage{}, err
	}
	page.Generation = state.Generation
	page.MaxSequence, err = sqlcgen.New(tx).MaxActiveEventSequence(ctx)
	if err != nil {
		return transcript.ChangePage{}, fmt.Errorf("store: query transcript changes max sequence: %w", err)
	}
	page.MinSequence, err = sqlcgen.New(tx).MinActiveEventSequence(ctx)
	if err != nil {
		return transcript.ChangePage{}, fmt.Errorf("store: query retained transcript boundary: %w", err)
	}
	rows, err := sqlcgen.New(tx).ListTranscriptChanges(ctx, sqlcgen.ListTranscriptChangesParams{
		AfterSequence: query.AfterSequence,
		SequenceLimit: int64(query.Limit + 1),
	})
	if err != nil {
		return transcript.ChangePage{}, fmt.Errorf("store: query transcript changes: %w", err)
	}
	entries, hasMore, err := materializedChangeEntries(rows, query.Limit)
	if err != nil {
		return transcript.ChangePage{}, err
	}
	page.HasMore = hasMore
	page.Entries = entries
	if len(entries) > 0 {
		page.NextAfter = entries[len(entries)-1].Sequence
	} else {
		page.NextAfter = query.AfterSequence
	}
	if err := tx.Commit(); err != nil {
		return transcript.ChangePage{}, fmt.Errorf("store: commit transcript changes read: %w", err)
	}
	return page, nil
}

func materializedChangeEntries(
	rows []sqlcgen.ListTranscriptChangesRow,
	limit int,
) ([]transcript.Entry, bool, error) {
	entries := make([]transcript.Entry, 0, len(rows))
	var previousSequence int64
	groups := 0
	hasMore := false
	for index, row := range rows {
		if index == 0 || row.UpdatedSequence != previousSequence {
			groups++
			previousSequence = row.UpdatedSequence
		}
		entry, err := materializedTranscriptEntry(
			row.MessageJson, row.StartSequence, row.UpdatedSequence, row.EventType, row.MarkerJson,
		)
		if err != nil {
			return nil, false, err
		}
		if groups > limit {
			hasMore = true
			continue
		}
		entries = append(entries, entry)
	}
	return entries, hasMore, nil
}

func decodeMaterializedEntry(entry *transcript.Entry, messageJSON string, markerJSON sql.NullString) error {
	if err := json.Unmarshal([]byte(messageJSON), &entry.Message); err != nil {
		return fmt.Errorf(
			"%w: decode message at sequence %d: %v",
			transcript.ErrProjectionCorrupt,
			entry.StartSequence,
			err,
		)
	}
	if markerJSON.Valid {
		var marker transcript.Marker
		if err := json.Unmarshal([]byte(markerJSON.String), &marker); err != nil {
			return fmt.Errorf(
				"%w: decode marker at sequence %d: %v",
				transcript.ErrProjectionCorrupt,
				entry.StartSequence,
				err,
			)
		}
		entry.Marker = &marker
	}
	return nil
}

func materializedTranscriptEntry(
	messageJSON sql.NullString,
	startSequence int64,
	updatedSequence int64,
	eventType string,
	markerJSON sql.NullString,
) (transcript.Entry, error) {
	entry := transcript.Entry{
		StartSequence: startSequence,
		Sequence:      updatedSequence,
		EventType:     eventType,
	}
	if !messageJSON.Valid {
		return transcript.Entry{}, fmt.Errorf(
			"%w: missing message at sequence %d",
			transcript.ErrProjectionCorrupt,
			startSequence,
		)
	}
	if err := decodeMaterializedEntry(&entry, messageJSON.String, markerJSON); err != nil {
		return transcript.Entry{}, err
	}
	return entry, nil
}
