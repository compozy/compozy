package sessiondb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// Close drains queued writes, checkpoints the WAL, and closes the database.
// If its deadline expires, cleanup retains ownership until active I/O stops;
// another Close can join that cleanup with its own waiting deadline.
func (s *SessionDB) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close session database context is required")
	}
	owner := s.state.CompareAndSwap(sessionStateOpen, sessionStateClosing)
	if !owner && s.state.Load() == sessionStateClosed {
		return nil
	}

	drainCtx, cancel := context.WithTimeout(ctx, s.drainTimeout)
	defer cancel()
	if owner {
		go s.close(drainCtx)
	}
	select {
	case <-s.closeDone:
		return s.closeErr
	case <-drainCtx.Done():
		if owner && s.cancel != nil {
			s.cancel()
		}
		return fmt.Errorf("%w: %w", store.ErrDrainTimeout, drainCtx.Err())
	}
}

func (s *SessionDB) close(drainCtx context.Context) {
	defer close(s.closeDone)
	s.acceptMu.Lock()
	resultCh := make(chan error, 1)
	s.shutdownCh <- sessionShutdownRequest{
		ctx:    drainCtx,
		result: resultCh,
	}
	s.acceptMu.Unlock()

	writerErr := waitForShutdownResult(drainCtx, resultCh)
	writerExitErr := waitForWriterExit(drainCtx, s.writerDone)
	if s.cancel != nil {
		s.cancel()
	}
	checkpointExitErr := waitForWriterExit(drainCtx, s.checkpointDone)
	// Cancellation cannot interrupt a filesystem sync already in progress. This
	// owned cleanup joins both workers before closing their connections, even if
	// the caller has already received ErrDrainTimeout.
	if s.writerDone != nil {
		<-s.writerDone
	}
	if s.checkpointDone != nil {
		<-s.checkpointDone
	}
	var readerCloseErr error
	if s.readDB != nil {
		readerCloseErr = s.readDB.Close()
	}
	// A retained reader must not turn optional WAL truncation into a shutdown failure.
	checkpointErr := store.CheckpointPassive(drainCtx, s.db)
	var maintenanceCloseErr error
	if s.checkpointDB != nil {
		maintenanceCloseErr = s.checkpointDB.Close()
	}
	closeErr := s.db.Close()

	s.closeErr = errors.Join(
		writerErr, writerExitErr, checkpointExitErr, readerCloseErr, checkpointErr, maintenanceCloseErr, closeErr,
	)
	s.state.Store(sessionStateClosed)
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
	case <-s.writerDone:
		return store.ErrClosed
	}

	select {
	case result := <-req.result:
		return result.err
	case <-ctx.Done():
		return fmt.Errorf("store: wait for session write completion: %w", ctx.Err())
	case <-s.writerDone:
		// Preserve an already committed outcome when shutdown and acknowledgement
		// become ready together. Unexecuted requests cannot wait on an exited worker.
		select {
		case result := <-req.result:
			return result.err
		default:
			return store.ErrClosed
		}
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
