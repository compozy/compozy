package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/agh/internal/store"
)

const sessionPassiveCheckpointEvery = 128

func (s *SessionDB) writerLoop() {
	writesSinceCheckpoint := 0
	for {
		if writesSinceCheckpoint >= sessionPassiveCheckpointEvery {
			if err := store.CheckpointPassive(s.writerCtx, s.db); err != nil {
				slog.Default().WarnContext(s.writerCtx, "store: passive session wal checkpoint failed", "error", err)
			}
			writesSinceCheckpoint = 0
		}
		select {
		case req := <-s.writeCh:
			result := s.executeWrite(req)
			if result.err == nil {
				writesSinceCheckpoint += sessionWriteCheckpointWeight(req, result)
			}
			req.result <- result
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
	case sessionWriteUsage, sessionWriteHookRun, sessionWriteArchive:
		return 1
	default:
		return 0
	}
}
