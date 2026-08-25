package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
)

var _ callspkg.PublicationStore = (*CallRepo)(nil)

// GetPublication resolves one published call/conversation idempotency row.
func (g *CallRepo) GetPublication(
	ctx context.Context,
	callID string,
	channel string,
	threadID string,
) (callspkg.Publication, error) {
	if err := g.checkReady(ctx, "get call publication"); err != nil {
		return callspkg.Publication{}, err
	}
	return scanCallPublication(g.db.QueryRowContext(ctx, `
		SELECT call_id, channel, thread_id, network_message_id, created_at
		FROM call_publications WHERE call_id = ? AND channel = ? AND thread_id = ?`,
		strings.TrimSpace(callID), strings.TrimSpace(channel), strings.TrimSpace(threadID),
	))
}

// RecordPublication inserts one publication, returning the conflict winner on replay.
func (g *CallRepo) RecordPublication(
	ctx context.Context,
	publication callspkg.Publication,
) (record callspkg.Publication, inserted bool, err error) {
	if err := g.checkReady(ctx, "record call publication"); err != nil {
		return callspkg.Publication{}, false, err
	}
	result, err := g.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO call_publications
		(call_id, channel, thread_id, network_message_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		strings.TrimSpace(publication.CallID), strings.TrimSpace(publication.Channel),
		strings.TrimSpace(publication.ThreadID), strings.TrimSpace(publication.NetworkMessageID),
		store.FormatTimestamp(publication.CreatedAt),
	)
	if err != nil {
		return callspkg.Publication{}, false, fmt.Errorf("store: record call publication: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return callspkg.Publication{}, false, fmt.Errorf("store: inspect call publication insert: %w", err)
	}
	record, err = g.GetPublication(ctx, publication.CallID, publication.Channel, publication.ThreadID)
	if err != nil {
		return callspkg.Publication{}, false, err
	}
	return record, rows == 1, nil
}

func scanCallPublication(row *sql.Row) (callspkg.Publication, error) {
	var publication callspkg.Publication
	var createdAt string
	err := row.Scan(
		&publication.CallID, &publication.Channel, &publication.ThreadID,
		&publication.NetworkMessageID, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return callspkg.Publication{}, callspkg.ErrPublicationNotFound
	}
	if err != nil {
		return callspkg.Publication{}, fmt.Errorf("store: scan call publication: %w", err)
	}
	publication.CreatedAt, err = store.ParseTimestamp(createdAt)
	if err != nil {
		return callspkg.Publication{}, fmt.Errorf("store: parse call publication created_at: %w", err)
	}
	return publication, nil
}
