package sessiondb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// CleanupComplete reports whether close has finished releasing all owned resources.
// A completed cleanup can still report an earlier drain timeout through Close.
func (s *SessionDB) CleanupComplete() bool {
	return s == nil || s.state.Load() == sessionStateClosed
}

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
	databaseCloseErr := s.closeDatabases(drainCtx)

	s.closeErr = errors.Join(writerErr, writerExitErr, checkpointExitErr, databaseCloseErr)
	s.state.Store(sessionStateClosed)
}

// closeDatabases retains the family guard through final backfill even after caller expiry.
func (s *SessionDB) closeDatabases(ctx context.Context) error {
	// Opening a pool connection takes the family lease itself. Acquire it before
	// guarding final backfill and last-connection close, which can also checkpoint.
	// Caller expiry bounds Close's wait; owned cleanup still joins physical opens.
	cleanupCtx := context.WithoutCancel(ctx)
	connection, connectionErr := s.db.Conn(cleanupCtx)
	lease, leaseErr := AcquireFamilyLease(cleanupCtx, s.path)
	defer lease.Release()

	var readerCloseErr error
	if s.readDB != nil {
		readerCloseErr = s.readDB.Close()
	}
	// A retained reader must not turn optional WAL truncation into a shutdown failure.
	checkpointErr := store.CheckpointPassiveConnection(cleanupCtx, connection)
	var maintenanceCloseErr error
	if s.checkpointDB != nil {
		maintenanceCloseErr = s.checkpointDB.Close()
	}
	var connectionCloseErr error
	if connection != nil {
		connectionCloseErr = connection.Close()
	}
	closeErr := s.db.Close()

	return errors.Join(
		connectionErr, leaseErr, readerCloseErr, checkpointErr, maintenanceCloseErr, connectionCloseErr, closeErr,
	)
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
