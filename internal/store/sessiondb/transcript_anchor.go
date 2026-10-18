package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

var (
	_ store.TranscriptAnchorReader = (*SessionDB)(nil)
	_ store.TranscriptAnchorReader = (*ReadOnlySessionDB)(nil)
	_ store.TranscriptAnchorReader = (*readOnlyPoolLease)(nil)
)

// TranscriptUserAnchor resolves a durable user message to its turn.
func (s *SessionDB) TranscriptUserAnchor(ctx context.Context, messageID string) (store.TranscriptUserAnchor, error) {
	if s == nil {
		return store.TranscriptUserAnchor{}, errors.New("store: session database is required")
	}
	return transcriptUserAnchor(ctx, s.db, messageID)
}

// TranscriptUserAnchor resolves a durable user message to its turn.
func (s *ReadOnlySessionDB) TranscriptUserAnchor(
	ctx context.Context,
	messageID string,
) (store.TranscriptUserAnchor, error) {
	if s == nil {
		return store.TranscriptUserAnchor{}, errors.New("store: read-only session database is required")
	}
	return transcriptUserAnchor(ctx, s.db, messageID)
}

// TranscriptUserAnchor resolves a durable user message through the pooled recorder.
func (l *readOnlyPoolLease) TranscriptUserAnchor(
	ctx context.Context,
	messageID string,
) (store.TranscriptUserAnchor, error) {
	if l == nil || l.entry == nil || l.entry.recorder == nil {
		return store.TranscriptUserAnchor{}, errors.New("store: read-only pool lease recorder is required")
	}
	reader, ok := l.entry.recorder.(store.TranscriptAnchorReader)
	if !ok {
		return store.TranscriptUserAnchor{}, errors.New("store: pooled recorder has no transcript anchors")
	}
	return reader.TranscriptUserAnchor(ctx, messageID)
}

// transcriptUserAnchor is the anchor lookup below rewind's eligibility rules: a complete
// user entry whose start event is active. Unlike conversationRewindTarget it does not
// reject anchors with an archived prefix (compaction or an earlier rewind).
func transcriptUserAnchor(
	ctx context.Context,
	db projectionDB,
	messageID string,
) (store.TranscriptUserAnchor, error) {
	if ctx == nil {
		return store.TranscriptUserAnchor{}, errors.New("store: transcript anchor context is required")
	}
	target := strings.TrimSpace(messageID)
	if target == "" {
		return store.TranscriptUserAnchor{}, errors.New("store: transcript anchor message id is required")
	}
	row, err := sqlcgen.New(db).GetTranscriptUserAnchor(ctx, sql.NullString{String: target, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return store.TranscriptUserAnchor{}, store.ErrTranscriptAnchorNotFound
	}
	if err != nil {
		return store.TranscriptUserAnchor{}, fmt.Errorf("store: query transcript anchor: %w", err)
	}
	if row.Kind != string(transcript.EntryKindUser) || row.Complete == 0 {
		return store.TranscriptUserAnchor{}, store.ErrTranscriptAnchorInvalid
	}
	return store.TranscriptUserAnchor{
		MessageID:     row.MessageID.String,
		TurnID:        strings.TrimSpace(row.TurnID),
		StartSequence: row.StartSequence,
		Complete:      true,
	}, nil
}
