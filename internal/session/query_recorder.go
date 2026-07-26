package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
)

type recorderOpenTarget struct {
	sessionID string
	dbPath    string
	active    EventRecorder
}

func (m *Manager) openQueryRecorder(ctx context.Context, id string) (EventReadCloser, func() error, error) {
	if m.openQueryStore == nil {
		return nil, nil, errors.New("session: query recorder opener is required")
	}
	target, err := m.resolveRecorderOpenTarget(ctx, id, true)
	if err != nil {
		return nil, nil, err
	}
	if target.active != nil {
		return target.active, func() error { return nil }, nil
	}
	reader, err := m.openQueryStore(ctx, target.sessionID, target.dbPath)
	if err != nil {
		return nil, nil, normalizeRecorderOpenError(target.sessionID, err)
	}
	return reader, eventStoreCleanup(reader), nil
}

func (m *Manager) openMutationRecorder(ctx context.Context, id string) (EventRecorder, func() error, error) {
	if m.openStore == nil {
		return nil, nil, errors.New("session: mutable recorder opener is required")
	}
	target, err := m.resolveRecorderOpenTarget(ctx, id, false)
	if err != nil {
		return nil, nil, err
	}
	if target.active != nil {
		return target.active, func() error { return nil }, nil
	}
	recorder, err := m.openStore(ctx, target.sessionID, target.dbPath)
	if err != nil {
		return nil, nil, normalizeRecorderOpenError(target.sessionID, err)
	}
	return recorder, eventStoreCleanup(recorder), nil
}

func (m *Manager) resolveRecorderOpenTarget(
	ctx context.Context,
	id string,
	allowReadOnlyFallback bool,
) (recorderOpenTarget, error) {
	if ctx == nil {
		return recorderOpenTarget{}, errors.New("session: query context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return recorderOpenTarget{}, errors.New("session: session id is required")
	}

	waited, err := m.waitForSessionFinalization(ctx, target)
	if err != nil {
		return recorderOpenTarget{}, fmt.Errorf("session: wait for finalization for %q: %w", target, err)
	}

	if session, ok := m.Get(target); ok {
		recorder := session.recorderHandle()
		if recorder == nil && session.Info().State == StateStarting {
			if err := waitForSessionStartRecorder(ctx, m.sessionStartRun(target)); err != nil {
				return recorderOpenTarget{}, fmt.Errorf(
					"session: wait for recorder for %q: %w",
					target,
					err,
				)
			}
			recorder = session.recorderHandle()
		}
		if recorder != nil {
			return recorderOpenTarget{sessionID: target, active: recorder}, nil
		}
		if !waited {
			recorder := session.recorderHandle()
			if recorder != nil {
				return recorderOpenTarget{sessionID: target, active: recorder}, nil
			}
			if allowReadOnlyFallback {
				m.logger.DebugContext(
					ctx,
					"session: active recorder unavailable; falling back to stored events",
					"session_id",
					target,
				)
			} else {
				return recorderOpenTarget{}, fmt.Errorf("session: recorder is not available for %q", target)
			}
		}
	}

	if _, err := m.readMetaWithContext(ctx, target); err != nil {
		return recorderOpenTarget{}, err
	}

	dbPath := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, target))
	if err := m.recoverSessionDBClear(ctx, target, dbPath); err != nil {
		return recorderOpenTarget{}, fmt.Errorf("session: recover clear state for %q: %w", target, err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return recorderOpenTarget{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
		}
		return recorderOpenTarget{}, fmt.Errorf("session: stat events database for %q: %w", target, err)
	}
	return recorderOpenTarget{sessionID: target, dbPath: dbPath}, nil
}

func normalizeRecorderOpenError(sessionID string, err error) error {
	if errors.Is(err, sessiondb.ErrReadOnlyPoolQuiescing) {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	return fmt.Errorf("session: open events database for %q: %w", sessionID, err)
}

func eventStoreCleanup(store store.EventCloser) func() error {
	return func() error {
		closeCtx, cancel := context.WithTimeout(context.Background(), defaultLifecycleTimeout)
		defer cancel()
		return store.Close(closeCtx)
	}
}

func (m *Manager) waitForSessionFinalization(ctx context.Context, id string) (bool, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return false, nil
	}

	m.mu.RLock()
	finalization, ok := m.finalizing[target]
	m.mu.RUnlock()
	if !ok || finalization == nil {
		return false, nil
	}

	select {
	case <-finalization.done:
		return true, nil
	case <-ctx.Done():
		return true, ctx.Err()
	}
}
