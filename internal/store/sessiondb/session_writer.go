package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const (
	sessionPassiveCheckpointEvery = 128
	// sessionWALTruncateBytes bounds a live session WAL. PASSIVE checkpoints never shrink the
	// file, and frequent readers keep SQLite from restarting it, so it otherwise grows for the
	// whole session.
	sessionWALTruncateBytes = 16 << 20
	// sessionWALTruncateWait caps how long readers may delay the writer during a truncation.
	sessionWALTruncateWait = 200 * time.Millisecond
)

func (s *SessionDB) writerLoop() {
	writesSinceCheckpoint := 0
	var pending *sessionWriteRequest
	for {
		if writesSinceCheckpoint >= sessionPassiveCheckpointEvery {
			s.checkpointWAL()
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
		s.transcriptFolds.reset()
		result, err := s.writeArchiveEvents(req.ctx, req.archive)
		return sessionWriteResult{archive: result, err: err}
	case sessionWriteConversationRewindBaselineRefresh:
		return sessionWriteResult{err: s.writeConversationRewindBaselineRefresh(req.ctx, req.baseline)}
	case sessionWriteConversationRewind:
		s.transcriptFolds.reset()
		result, err := s.writeConversationRewind(req.ctx, req.rewind)
		return sessionWriteResult{rewind: result, err: err}
	case sessionWriteClear:
		s.transcriptFolds.reset()
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
	case sessionWriteUsage,
		sessionWriteHookRun,
		sessionWriteArchive,
		sessionWriteConversationRewind,
		sessionWriteConversationRewindBaselineRefresh:
		return 1
	default:
		return 0
	}
}

func (s *SessionDB) passiveCheckpoint(ctx context.Context) error {
	return s.checkpointWithLease(ctx, store.CheckpointPassiveConnection)
}

// checkpointWAL backfills the WAL and truncates it once it outgrows the live-session bound.
// Readers that outlast the bounded wait defer truncation to the next checkpoint cycle.
func (s *SessionDB) checkpointWAL() {
	if err := s.passiveCheckpoint(s.writerCtx); err != nil {
		slog.Default().WarnContext(s.writerCtx, "store: passive session wal checkpoint failed", "error", err)
		return
	}
	info, err := os.Stat(s.path + "-wal")
	if err != nil || info.Size() < s.walTruncateLimit() {
		return
	}
	ctx, cancel := context.WithTimeout(s.writerCtx, sessionWALTruncateWait)
	defer cancel()
	err = s.checkpointWithLease(ctx, store.CheckpointTruncateConnection)
	switch {
	case err == nil:
	case errors.Is(err, context.DeadlineExceeded):
		slog.Default().DebugContext(s.writerCtx, "store: session wal truncation deferred by readers",
			"wal_bytes", info.Size())
	default:
		slog.Default().WarnContext(s.writerCtx, "store: truncate session wal failed", "error", err)
	}
}

func (s *SessionDB) walTruncateLimit() int64 {
	if limit := s.walTruncateBytes.Load(); limit > 0 {
		return limit
	}
	return sessionWALTruncateBytes
}

func (s *SessionDB) checkpointWithLease(
	ctx context.Context,
	checkpoint func(context.Context, *sql.Conn) error,
) (err error) {
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
	return checkpoint(ctx, connection)
}
