package session

import (
	"context"
	"errors"
	"fmt"
	"os"

	"strings"

	"github.com/compozy/agh/internal/store"
)

// InputQueueSummary is the session package projection of durable busy-input queue state.
type InputQueueSummary struct {
	QueueGeneration int64
	PendingInputs   int
}

// ListAll returns active and stopped sessions discovered from on-disk metadata.
func (m *Manager) ListAll(ctx context.Context) ([]*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: list context is required")
	}

	active := m.List()
	activeByID := make(map[string]*Info, len(active))
	for _, info := range active {
		if info == nil {
			continue
		}
		activeByID[info.ID] = info
	}

	entries, err := os.ReadDir(m.homePaths.SessionsDir)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return sortSessionInfos(active), nil
	default:
		return nil, fmt.Errorf("session: read sessions directory %q: %w", m.homePaths.SessionsDir, err)
	}

	infos := make([]*Info, 0, len(entries)+len(activeByID))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("session: list sessions canceled: %w", err)
		}
		if !entry.IsDir() {
			continue
		}

		id := strings.TrimSpace(entry.Name())
		if id == "" || strings.HasPrefix(id, sessionDeleteTombstonePrefix) {
			continue
		}

		meta, err := m.readMetaWithContext(ctx, id)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				continue
			}
			m.logger.Warn("session: skip unreadable session metadata", "session_id", id, "error", err)
			if info, ok := activeByID[id]; ok && info != nil {
				infos = append(infos, info)
				seen[id] = struct{}{}
			}
			continue
		}

		info := m.sessionInfoFromMeta(ctx, meta)
		if activeInfo, ok := activeByID[id]; ok && activeInfo != nil {
			info = activeInfo
		}
		infos = append(infos, info)
		seen[id] = struct{}{}
	}

	for id, info := range activeByID {
		if _, ok := seen[id]; ok || info == nil {
			continue
		}
		infos = append(infos, info)
	}

	return sortSessionInfos(infos), nil
}

// Status returns the current session status from memory or on-disk metadata.
func (m *Manager) Status(ctx context.Context, id string) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("session: status context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return nil, errors.New("session: session id is required")
	}

	if session, ok := m.Get(target); ok {
		return normalizeExpiredSessionAttach(m.sessionInfoForRead(session), m.now()), nil
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	return m.sessionInfoFromMeta(ctx, meta), nil
}

// InputQueueSummary returns the durable busy-input state for one session.
func (m *Manager) InputQueueSummary(ctx context.Context, id string) (InputQueueSummary, error) {
	if ctx == nil {
		return InputQueueSummary{}, errors.New("session: input queue summary context is required")
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return InputQueueSummary{}, errors.New("session: session id is required")
	}
	if m.inputQueueStore == nil {
		return InputQueueSummary{}, nil
	}
	summary, err := m.inputQueueStore.SessionInputQueueSummary(ctx, target)
	if err != nil {
		return InputQueueSummary{}, fmt.Errorf("session: read input queue summary for %q: %w", target, err)
	}
	return InputQueueSummary{
		QueueGeneration: summary.Generation,
		PendingInputs:   summary.PendingActive,
	}, nil
}

// Events returns persisted session events for active or stopped sessions.
func (m *Manager) Events(
	ctx context.Context,
	id string,
	query store.EventQuery,
) (events []store.SessionEvent, err error) {
	target := strings.TrimSpace(id)
	for attempt := range 2 {
		recorder, cleanup, openErr := m.openQueryRecorder(ctx, id)
		if openErr != nil {
			return nil, openErr
		}

		events, err = recorder.Query(ctx, query)
		cleanupErr := cleanup()
		if err == nil {
			if cleanupErr != nil {
				return nil, cleanupErr
			}
			return events, nil
		}
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		if errors.Is(err, store.ErrClosed) && attempt == 0 {
			if _, waitErr := m.waitForSessionFinalization(ctx, target); waitErr != nil {
				return nil, fmt.Errorf(
					"session: wait for finalization for %q after closed recorder: %w",
					target,
					waitErr,
				)
			}
			continue
		}
		return nil, fmt.Errorf("session: query events for %q: %w", target, err)
	}
	return nil, fmt.Errorf("session: query events for %q: %w", target, err)
}

// History returns turn-grouped persisted session events for active or stopped sessions.
func (m *Manager) History(
	ctx context.Context,
	id string,
	query store.EventQuery,
) (history []store.TurnHistory, err error) {
	target := strings.TrimSpace(id)
	for attempt := range 2 {
		recorder, cleanup, openErr := m.openQueryRecorder(ctx, id)
		if openErr != nil {
			return nil, openErr
		}

		history, err = recorder.History(ctx, query)
		cleanupErr := cleanup()
		if err == nil {
			if cleanupErr != nil {
				return nil, cleanupErr
			}
			return history, nil
		}
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
		if errors.Is(err, store.ErrClosed) && attempt == 0 {
			if _, waitErr := m.waitForSessionFinalization(ctx, target); waitErr != nil {
				return nil, fmt.Errorf(
					"session: wait for finalization for %q after closed recorder: %w",
					target,
					waitErr,
				)
			}
			continue
		}
		return nil, fmt.Errorf("session: query history for %q: %w", target, err)
	}
	return nil, fmt.Errorf("session: query history for %q: %w", target, err)
}
