package sessiondb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
)

// ArchiveEvents marks one fixed event range as excluded from replay.
func (s *SessionDB) ArchiveEvents(
	ctx context.Context,
	request store.EventArchiveRequest,
) (store.EventArchiveResult, error) {
	if s == nil {
		return store.EventArchiveResult{}, errors.New("store: session database is required")
	}
	if ctx == nil {
		return store.EventArchiveResult{}, errors.New("store: archive events context is required")
	}
	if err := request.Validate(); err != nil {
		return store.EventArchiveResult{}, err
	}

	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return store.EventArchiveResult{}, store.ErrClosed
	}
	req := sessionWriteRequest{
		ctx:     ctx,
		kind:    sessionWriteArchive,
		archive: request,
		result:  make(chan sessionWriteResult, 1),
	}
	select {
	case s.writeCh <- req:
	case <-ctx.Done():
		return store.EventArchiveResult{}, fmt.Errorf("store: enqueue event archive: %w", ctx.Err())
	}
	select {
	case result := <-req.result:
		if result.err != nil {
			return store.EventArchiveResult{}, result.err
		}
		return result.archive, nil
	case <-ctx.Done():
		return store.EventArchiveResult{}, fmt.Errorf("store: wait for event archive: %w", ctx.Err())
	}
}

func (s *SessionDB) writeArchiveEvents(
	ctx context.Context,
	request store.EventArchiveRequest,
) (store.EventArchiveResult, error) {
	result := store.EventArchiveResult{
		FromSequence: request.FromSequence,
		ToSequence:   request.ToSequence,
	}
	err := store.ExecuteWrite(ctx, s.db, func(ctx context.Context, tx *store.WriteTx) error {
		count, err := archiveTranscriptRange(ctx, sqlcgen.New(tx), request)
		if err != nil {
			return err
		}

		result.ArchivedCount = count
		return nil
	})
	return result, err
}

// archiveTranscriptRange is the shared atomic cut for compaction and rewind.
// It never cuts a live/partially covered entry or changes the surviving active identity.
func archiveTranscriptRange(
	ctx context.Context,
	queries *sqlcgen.Queries,
	request store.EventArchiveRequest,
) (int64, error) {
	crossing, err := queries.CountTranscriptEntriesCrossingCut(ctx, sqlcgen.CountTranscriptEntriesCrossingCutParams{
		FromSequence: request.FromSequence, ToSequence: request.ToSequence,
	})
	if err != nil {
		return 0, fmt.Errorf("store: inspect transcript archive boundary: %w", err)
	}
	if crossing != 0 {
		return 0, errors.New("store: archive range cuts an incomplete transcript entry")
	}
	count, err := queries.ArchiveEventRange(
		ctx,
		sqlcgen.ArchiveEventRangeParams{FromSequence: request.FromSequence, ToSequence: request.ToSequence},
	)
	if err != nil {
		return 0, fmt.Errorf("store: archive transcript range: %w", err)
	}
	if count == 0 {
		return 0, nil
	}
	if err := queries.DeleteTranscriptToolRoutesInRange(
		ctx,
		sqlcgen.DeleteTranscriptToolRoutesInRangeParams{
			FromSequence: request.FromSequence,
			ToSequence:   request.ToSequence,
		},
	); err != nil {
		return 0, fmt.Errorf("store: cut transcript routes: %w", err)
	}
	if err := queries.DeleteTranscriptEntriesInRange(
		ctx,
		sqlcgen.DeleteTranscriptEntriesInRangeParams{
			FromSequence: request.FromSequence,
			ToSequence:   request.ToSequence,
		},
	); err != nil {
		return 0, fmt.Errorf("store: cut transcript entries: %w", err)
	}
	if err := queries.AdvanceTranscriptProjectionGeneration(ctx); err != nil {
		return 0, fmt.Errorf("store: advance archive generation: %w", err)
	}
	return count, nil
}
