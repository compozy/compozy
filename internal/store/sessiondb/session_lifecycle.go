package sessiondb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

// Close drains queued writes, checkpoints the WAL, and closes the database.
func (s *SessionDB) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close session database context is required")
	}
	if !s.state.CompareAndSwap(sessionStateOpen, sessionStateClosing) {
		if s.state.Load() == sessionStateClosed {
			return nil
		}
		return store.ErrClosed
	}

	drainCtx, cancel := context.WithTimeout(ctx, s.drainTimeout)
	defer cancel()
	if s.cancel != nil {
		defer s.cancel()
	}

	s.acceptMu.Lock()
	resultCh := make(chan error, 1)
	s.shutdownCh <- sessionShutdownRequest{
		ctx:    drainCtx,
		result: resultCh,
	}
	s.acceptMu.Unlock()

	writerErr := waitForShutdownResult(drainCtx, resultCh)
	writerExitErr := waitForWriterExit(drainCtx, s.writerDone)
	checkpointErr := store.Checkpoint(drainCtx, s.db)
	closeErr := s.db.Close()

	s.state.Store(sessionStateClosed)

	return errors.Join(writerErr, writerExitErr, checkpointErr, closeErr)
}

func (s *SessionDB) enqueueWrite(ctx context.Context, req sessionWriteRequest) error {
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()

	if s.state.Load() != sessionStateOpen {
		return store.ErrClosed
	}

	select {
	case s.writeCh <- req:
	case <-ctx.Done():
		return fmt.Errorf("store: enqueue session write: %w", ctx.Err())
	}

	select {
	case result := <-req.result:
		return result.err
	case <-ctx.Done():
		return fmt.Errorf("store: wait for session write completion: %w", ctx.Err())
	}
}

func waitForShutdownResult(ctx context.Context, resultCh <-chan error) error {
	select {
	case err := <-resultCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", store.ErrDrainTimeout, ctx.Err())
	}
}

func waitForWriterExit(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", store.ErrDrainTimeout, ctx.Err())
	}
}
