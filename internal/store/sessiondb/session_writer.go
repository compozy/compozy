package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/store"
)

const sessionPassiveCheckpointEvery = 128

func (s *SessionDB) writerLoop() {
	writesSinceCheckpoint := 0
	var pending *sessionWriteRequest
	for {
		if writesSinceCheckpoint >= sessionPassiveCheckpointEvery {
			if err := s.passiveCheckpoint(s.writerCtx); err != nil {
				slog.Default().WarnContext(s.writerCtx, "store: passive session wal checkpoint failed", "error", err)
			}
			writesSinceCheckpoint = 0
		}
		if pending != nil {
			request := *pending
			var weight int
			pending, weight = s.writePendingChunks(request)
			writesSinceCheckpoint += weight
			continue
		}
		select {
		case req := <-s.writeCh:
			var weight int
			pending, weight = s.writePendingChunks(req)
			writesSinceCheckpoint += weight
		case shutdown := <-s.shutdownCh:
			shutdown.result <- s.drainWrites(shutdown.ctx)
			return
		case <-s.writerCtx.Done():
			return
		}
	}
}

func (s *SessionDB) drainWrites(ctx context.Context) error {
	var drainErr error

	for {
		select {
		case <-ctx.Done():
			return errors.Join(drainErr, fmt.Errorf("%w: %w", store.ErrDrainTimeout, ctx.Err()))
		case req := <-s.writeCh:
			result := s.executeWrite(req)
			req.result <- result
			if result.err != nil {
				drainErr = errors.Join(drainErr, result.err)
			}
		default:
			return drainErr
		}
	}
}

func (s *SessionDB) executeWrite(req sessionWriteRequest) sessionWriteResult {
	if err := req.ctx.Err(); err != nil {
		return sessionWriteResult{err: fmt.Errorf("store: session write canceled before execution: %w", err)}
	}

	switch req.kind {
	case sessionWriteEvent:
		event, err := s.writeEvent(req.ctx, req.event)
		return sessionWriteResult{event: event, err: err}
	case sessionWriteEventIfAbsent:
		event, err := s.writeEventIfAbsent(req.ctx, req.event)
		return sessionWriteResult{event: event, err: err}
	case sessionWriteEventBatch:
		events, err := s.writeEventBatch(req.ctx, req.events)
		return sessionWriteResult{events: events, err: err}
	case sessionWriteUsage:
		return sessionWriteResult{err: s.writeTokenUsage(req.ctx, req.usage)}
	case sessionWriteHookRun:
		return sessionWriteResult{err: s.writeHookRun(req.ctx, req.hook)}
	case sessionWriteArchive:
		result, err := s.writeArchiveEvents(req.ctx, req.archive)
		return sessionWriteResult{archive: result, err: err}
	case sessionWriteConversationRewind:
		result, err := s.writeConversationRewind(req.ctx, req.rewind)
		return sessionWriteResult{rewind: result, err: err}
	case sessionWriteClear:
		err := clearSessionSQLite(req.ctx, s.db)
		if err != nil {
			err = fmt.Errorf("store: clear session database: %w", err)
		}
		return sessionWriteResult{err: err}
	default:
		return sessionWriteResult{err: fmt.Errorf("store: unsupported session write kind %d", req.kind)}
	}
}

func sessionWriteCheckpointWeight(req sessionWriteRequest, result sessionWriteResult) int {
	switch req.kind {
	case sessionWriteEvent, sessionWriteEventIfAbsent:
		return 1
	case sessionWriteEventBatch:
		return len(result.events)
	case sessionWriteUsage, sessionWriteHookRun, sessionWriteArchive, sessionWriteConversationRewind:
		return 1
	default:
		return 0
	}
}

func (s *SessionDB) passiveCheckpoint(ctx context.Context) (err error) {
	// Acquire the connection before the family lease: opening a pool connection
	// itself takes that lease to verify the immutable database identity.
	connection, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire session checkpoint connection: %w", err)
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: release session checkpoint connection: %w", closeErr))
		}
	}()
	lease, err := AcquireFamilyLease(ctx, s.path)
	if err != nil {
		return err
	}
	defer lease.Release()
	return store.CheckpointPassiveConnection(ctx, connection)
}
