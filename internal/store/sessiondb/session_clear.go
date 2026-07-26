package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb/sqlcgen"
)

func clearSessionSQLite(ctx context.Context, db *sql.DB) error {
	if ctx == nil {
		return errors.New("store: clear session sqlite context is required")
	}
	if db == nil {
		return errors.New("store: clear session sqlite database is required")
	}
	return store.ExecuteWrite(ctx, db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		clearSteps := []func(context.Context) error{
			queries.ClearHookRuns,
			queries.ClearTokenUsage,
			queries.ClearTranscriptToolRoutes,
			queries.ClearTranscriptEntries,
			queries.ClearEvents,
		}
		for _, clear := range clearSteps {
			if err := clear(ctx); err != nil {
				return fmt.Errorf("store: clear session table: %w", err)
			}
		}
		if err := queries.AdvanceTranscriptProjectionGeneration(ctx); err != nil {
			return fmt.Errorf("store: advance transcript projection generation: %w", err)
		}
		return nil
	})
}

// Clear removes all persisted per-session runtime records while preserving the
// migrated database schema and the open writer. It is used by session reset
// flows before the recorder is exposed to any live session goroutine.
func (s *SessionDB) Clear(ctx context.Context) error {
	if s == nil {
		return errors.New("store: session database is required")
	}
	if ctx == nil {
		return errors.New("store: clear session database context is required")
	}

	s.acceptMu.Lock()
	defer s.acceptMu.Unlock()
	if s.state.Load() != sessionStateOpen {
		return store.ErrClosed
	}

	req := sessionWriteRequest{
		ctx:    ctx,
		kind:   sessionWriteClear,
		result: make(chan sessionWriteResult, 1),
	}
	select {
	case s.writeCh <- req:
	case <-ctx.Done():
		return fmt.Errorf("store: enqueue session clear: %w", ctx.Err())
	}

	select {
	case result := <-req.result:
		return result.err
	case <-ctx.Done():
		return fmt.Errorf("store: wait for session clear completion: %w", ctx.Err())
	}
}
