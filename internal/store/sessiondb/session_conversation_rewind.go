package sessiondb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

func (s *SessionDB) ConversationRewindTarget(
	ctx context.Context,
	messageID string,
) (store.ConversationRewindTarget, error) {
	if s == nil {
		return store.ConversationRewindTarget{}, errors.New("store: session database is required")
	}
	return conversationRewindTarget(ctx, s.db, messageID)
}

func (s *SessionDB) ConversationRewindState(
	ctx context.Context,
) (store.ConversationRewindState, bool, error) {
	if s == nil {
		return store.ConversationRewindState{}, false, errors.New("store: session database is required")
	}
	return conversationRewindState(ctx, s.db)
}

func (s *SessionDB) ConversationRewindReceipt(
	ctx context.Context,
	idempotencyKey string,
	requestHash string,
) (store.ConversationRewindResult, bool, error) {
	if s == nil {
		return store.ConversationRewindResult{}, false, errors.New("store: session database is required")
	}
	return conversationRewindReceipt(ctx, s.db, idempotencyKey, requestHash)
}

func (s *SessionDB) RewindConversation(
	ctx context.Context,
	request store.ConversationRewindRequest,
) (store.ConversationRewindResult, error) {
	if s == nil {
		return store.ConversationRewindResult{}, errors.New("store: session database is required")
	}
	if ctx == nil {
		return store.ConversationRewindResult{}, errors.New("store: conversation rewind context is required")
	}
	if err := request.Validate(); err != nil {
		return store.ConversationRewindResult{}, err
	}
	if !json.Valid([]byte(request.BaselineJSON)) {
		return store.ConversationRewindResult{}, errors.New("store: conversation rewind baseline is invalid JSON")
	}

	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return store.ConversationRewindResult{}, store.ErrClosed
	}
	req := sessionWriteRequest{
		ctx: ctx, kind: sessionWriteConversationRewind, rewind: &request,
		result: make(chan sessionWriteResult, 1),
	}
	select {
	case s.writeCh <- req:
	case <-ctx.Done():
		return store.ConversationRewindResult{}, fmt.Errorf("store: enqueue conversation rewind: %w", ctx.Err())
	}
	select {
	case result := <-req.result:
		return result.rewind, result.err
	case <-ctx.Done():
		return store.ConversationRewindResult{}, fmt.Errorf("store: wait for conversation rewind: %w", ctx.Err())
	}
}

func (s *SessionDB) writeConversationRewind(
	ctx context.Context,
	request *store.ConversationRewindRequest,
) (result store.ConversationRewindResult, retErr error) {
	if request == nil {
		return store.ConversationRewindResult{}, errors.New("store: conversation rewind request is required")
	}
	retErr = store.ExecuteWrite(ctx, s.db, func(ctx context.Context, tx *store.WriteTx) error {
		var err error
		result, err = s.applyConversationRewind(ctx, tx, *request)
		return err
	})
	return result, retErr
}

func (s *SessionDB) applyConversationRewind(
	ctx context.Context,
	tx *store.WriteTx,
	request store.ConversationRewindRequest,
) (store.ConversationRewindResult, error) {
	queries := sqlcgen.New(tx)
	if replayed, found, err := replayedConversationRewind(ctx, queries, request); err != nil || found {
		return replayed, err
	}
	state, err := queries.GetTranscriptProjectionState(ctx)
	if err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf(
			"store: query conversation rewind generation: %w",
			err,
		)
	}
	maxSequence, err := queries.MaxActiveEventSequence(ctx)
	if err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf(
			"store: query conversation rewind max sequence: %w",
			err,
		)
	}
	if state.Generation != request.ExpectedGeneration || maxSequence != request.ExpectedMaxSequence {
		return store.ConversationRewindResult{}, store.ErrConversationRewindFenceConflict
	}

	target, err := conversationRewindTarget(ctx, tx, request.MessageID)
	if err != nil {
		return store.ConversationRewindResult{}, err
	}
	archive, err := archiveConversationRewindSuffix(ctx, queries, target)
	if err != nil {
		return store.ConversationRewindResult{}, err
	}
	nowTime := s.now().UTC()
	now := nowTime.Format(time.RFC3339Nano)
	if err := queries.UpsertConversationRewindState(ctx, sqlcgen.UpsertConversationRewindStateParams{
		TargetMessageID:        target.MessageID,
		CoveredThroughSequence: target.StartSequence - 1,
		MessagesJson:           request.BaselineJSON,
		UpdatedAt:              now,
	}); err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf("store: persist conversation rewind baseline: %w", err)
	}
	result := store.ConversationRewindResult{
		IdempotencyKey: request.IdempotencyKey, TargetMessageID: target.MessageID,
		ArchivedFromSequence: target.StartSequence, ArchivedToSequence: archive.toSequence,
		ArchivedEventCount: archive.eventCount, Generation: archive.generation,
		MaxSequence: archive.maxSequence, TranscriptEpoch: request.TranscriptEpoch,
		DraftText: target.DraftText, CreatedAt: nowTime,
	}
	if err := queries.InsertConversationRewindReceipt(ctx, sqlcgen.InsertConversationRewindReceiptParams{
		IdempotencyKey: request.IdempotencyKey, RequestHash: request.RequestHash,
		TargetMessageID: target.MessageID, ArchivedFromSequence: target.StartSequence,
		ArchivedToSequence: archive.toSequence, ArchivedEventCount: archive.eventCount,
		Generation: archive.generation, MaxSequence: archive.maxSequence,
		TranscriptEpoch: request.TranscriptEpoch,
		DraftText:       target.DraftText, CreatedAt: now,
	}); err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf("store: persist conversation rewind receipt: %w", err)
	}
	return result, nil
}

type conversationRewindArchive struct {
	toSequence  int64
	eventCount  int64
	generation  int64
	maxSequence int64
}

func archiveConversationRewindSuffix(
	ctx context.Context,
	queries *sqlcgen.Queries,
	target store.ConversationRewindTarget,
) (conversationRewindArchive, error) {
	toSequence, err := queries.MaxActiveEventSequence(ctx)
	if err != nil {
		return conversationRewindArchive{}, fmt.Errorf(
			"store: query conversation rewind archive boundary: %w",
			err,
		)
	}
	if toSequence < target.StartSequence {
		return conversationRewindArchive{}, store.ErrConversationRewindTargetInvalid
	}
	archived, err := queries.ArchiveEventRange(ctx, sqlcgen.ArchiveEventRangeParams{
		FromSequence: target.StartSequence,
		ToSequence:   toSequence,
	})
	if err != nil {
		return conversationRewindArchive{}, fmt.Errorf(
			"store: archive conversation rewind suffix: %w",
			err,
		)
	}
	if err := queries.DeleteTranscriptToolRoutesFromSequence(ctx, target.StartSequence); err != nil {
		return conversationRewindArchive{}, fmt.Errorf(
			"store: delete conversation rewind tool routes: %w",
			err,
		)
	}
	if err := queries.DeleteTranscriptEntriesFromSequence(ctx, target.StartSequence); err != nil {
		return conversationRewindArchive{}, fmt.Errorf(
			"store: delete conversation rewind transcript suffix: %w",
			err,
		)
	}
	if err := queries.AdvanceTranscriptProjectionGeneration(ctx); err != nil {
		return conversationRewindArchive{}, fmt.Errorf(
			"store: advance conversation rewind generation: %w",
			err,
		)
	}
	updatedState, err := queries.GetTranscriptProjectionState(ctx)
	if err != nil {
		return conversationRewindArchive{}, fmt.Errorf(
			"store: query updated conversation rewind generation: %w",
			err,
		)
	}
	activeMax, err := queries.MaxActiveEventSequence(ctx)
	if err != nil {
		return conversationRewindArchive{}, fmt.Errorf(
			"store: query updated conversation rewind max sequence: %w",
			err,
		)
	}
	return conversationRewindArchive{
		toSequence: toSequence, eventCount: archived,
		generation: updatedState.Generation, maxSequence: activeMax,
	}, nil
}

func replayedConversationRewind(
	ctx context.Context,
	queries *sqlcgen.Queries,
	request store.ConversationRewindRequest,
) (store.ConversationRewindResult, bool, error) {
	return readConversationRewindReceipt(ctx, queries, request.IdempotencyKey, request.RequestHash)
}

func readConversationRewindReceipt(
	ctx context.Context,
	queries *sqlcgen.Queries,
	idempotencyKey string,
	requestHash string,
) (store.ConversationRewindResult, bool, error) {
	receipt, err := queries.GetConversationRewindReceipt(ctx, idempotencyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ConversationRewindResult{}, false, nil
	}
	if err != nil {
		return store.ConversationRewindResult{}, false, fmt.Errorf(
			"store: query conversation rewind receipt: %w",
			err,
		)
	}
	if receipt.RequestHash != requestHash {
		return store.ConversationRewindResult{}, false, store.ErrConversationRewindIdempotencyConflict
	}
	result, err := conversationRewindResultFromReceipt(receipt)
	if err != nil {
		return store.ConversationRewindResult{}, false, err
	}
	result.Replayed = true
	return result, true, nil
}

func conversationRewindTarget(
	ctx context.Context,
	db projectionDB,
	messageID string,
) (store.ConversationRewindTarget, error) {
	if ctx == nil {
		return store.ConversationRewindTarget{}, errors.New("store: conversation rewind target context is required")
	}
	target := strings.TrimSpace(messageID)
	if target == "" {
		return store.ConversationRewindTarget{}, errors.New("store: conversation rewind message id is required")
	}
	queries := sqlcgen.New(db)
	row, err := queries.GetConversationRewindTarget(ctx, sql.NullString{
		String: target,
		Valid:  true,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.ConversationRewindTarget{}, store.ErrConversationRewindTargetNotFound
	}
	if err != nil {
		return store.ConversationRewindTarget{}, fmt.Errorf("store: query conversation rewind target: %w", err)
	}
	if row.Kind != string(transcript.EntryKindUser) || row.Complete == 0 || !row.MessageJson.Valid {
		return store.ConversationRewindTarget{}, store.ErrConversationRewindTargetInvalid
	}
	archivedPrefixEvents, err := queries.CountArchivedTranscriptEventsBeforeSequence(ctx, row.StartSequence)
	if err != nil {
		return store.ConversationRewindTarget{}, fmt.Errorf(
			"store: query archived conversation rewind prefix: %w",
			err,
		)
	}
	if archivedPrefixEvents > 0 {
		return store.ConversationRewindTarget{}, store.ErrConversationRewindTargetInvalid
	}
	var message transcript.UIMessage
	if err := json.Unmarshal([]byte(row.MessageJson.String), &message); err != nil {
		return store.ConversationRewindTarget{}, fmt.Errorf("store: decode conversation rewind target: %w", err)
	}
	parts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			parts = append(parts, part.Text)
		}
	}
	state, err := queries.GetTranscriptProjectionState(ctx)
	if err != nil {
		return store.ConversationRewindTarget{}, fmt.Errorf("store: query conversation rewind generation: %w", err)
	}
	maxSequence, err := queries.MaxActiveEventSequence(ctx)
	if err != nil {
		return store.ConversationRewindTarget{}, fmt.Errorf("store: query conversation rewind max sequence: %w", err)
	}
	return store.ConversationRewindTarget{
		MessageID: row.MessageID.String, StartSequence: row.StartSequence,
		DraftText: strings.Join(parts, "\n"), Generation: state.Generation,
		MaxSequence: maxSequence,
	}, nil
}

func conversationRewindState(
	ctx context.Context,
	db projectionDB,
) (store.ConversationRewindState, bool, error) {
	if ctx == nil {
		return store.ConversationRewindState{}, false, errors.New(
			"store: conversation rewind state context is required",
		)
	}
	row, err := sqlcgen.New(db).GetConversationRewindState(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ConversationRewindState{}, false, nil
	}
	if err != nil {
		return store.ConversationRewindState{}, false, fmt.Errorf("store: query conversation rewind state: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	if err != nil {
		return store.ConversationRewindState{}, false, fmt.Errorf("store: parse conversation rewind timestamp: %w", err)
	}
	return store.ConversationRewindState{
		TargetMessageID: row.TargetMessageID, CoveredThroughSequence: row.CoveredThroughSequence,
		MessagesJSON: row.MessagesJson, UpdatedAt: updatedAt,
	}, true, nil
}

func conversationRewindReceipt(
	ctx context.Context,
	db projectionDB,
	idempotencyKey string,
	requestHash string,
) (store.ConversationRewindResult, bool, error) {
	if ctx == nil {
		return store.ConversationRewindResult{}, false, errors.New(
			"store: conversation rewind receipt context is required",
		)
	}
	key := strings.TrimSpace(idempotencyKey)
	hash := strings.TrimSpace(requestHash)
	if key == "" || hash == "" {
		return store.ConversationRewindResult{}, false, errors.New(
			"store: conversation rewind receipt identity is required",
		)
	}
	return readConversationRewindReceipt(ctx, sqlcgen.New(db), key, hash)
}

func conversationRewindResultFromReceipt(
	row sqlcgen.ConversationRewindReceipt,
) (store.ConversationRewindResult, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf(
			"store: parse conversation rewind receipt timestamp: %w",
			err,
		)
	}
	return store.ConversationRewindResult{
		IdempotencyKey: row.IdempotencyKey, TargetMessageID: row.TargetMessageID,
		ArchivedFromSequence: row.ArchivedFromSequence, ArchivedToSequence: row.ArchivedToSequence,
		ArchivedEventCount: row.ArchivedEventCount, Generation: row.Generation,
		MaxSequence: row.MaxSequence, TranscriptEpoch: row.TranscriptEpoch,
		DraftText: row.DraftText, CreatedAt: createdAt,
	}, nil
}
