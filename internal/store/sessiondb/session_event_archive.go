package sessiondb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
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
		count, err := sqlcgen.New(tx).ArchiveEventRange(ctx, sqlcgen.ArchiveEventRangeParams{
			FromSequence: request.FromSequence,
			ToSequence:   request.ToSequence,
		})
		if err != nil {
			return fmt.Errorf("store: archive session event range: %w", err)
		}
		result.ArchivedCount = count
		return nil
	})
	return result, err
}
