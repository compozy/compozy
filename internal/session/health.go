package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
)

// HealthRecoveryResult summarizes one metadata-only restart recovery pass.
type HealthRecoveryResult struct {
	RefreshedActive int
	Recomputed      int
	MarkedStale     int64
}

type sessionHealthInput struct {
	activePrompt  bool
	attachable    bool
	touchPresence bool
	activityAt    time.Time
	lastError     string
}

// TouchSessionPresence records an idle metadata-only presence touch for an attachable session.
func (m *Manager) TouchSessionPresence(ctx context.Context, id string) (heartbeat.SessionHealth, error) {
	if m == nil {
		return heartbeat.SessionHealth{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return heartbeat.SessionHealth{}, errors.New("session: health context is required")
	}
	session, err := m.lookup(id)
	if err != nil {
		return heartbeat.SessionHealth{}, err
	}
	return m.persistSessionPresence(ctx, session, m.now())
}

// GetSessionHealth returns a route-ready metadata-only health read model.
func (m *Manager) GetSessionHealth(ctx context.Context, id string) (heartbeat.SessionHealth, error) {
	if m == nil {
		return heartbeat.SessionHealth{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return heartbeat.SessionHealth{}, errors.New("session: health context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return heartbeat.SessionHealth{}, errors.New("session: session id is required")
	}

	if session, ok := m.Get(target); ok {
		return m.persistSessionPresence(ctx, session, m.now())
	}

	existing, err := m.storedSessionHealth(ctx, target)
	if err != nil {
		return heartbeat.SessionHealth{}, err
	}
	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return heartbeat.SessionHealth{}, err
	}
	health := m.sessionHealthFromInfo(sessionInfoFromMeta(meta), existing, m.now(), sessionHealthInput{})
	return m.storeSessionHealth(ctx, health)
}

// ListSessionHealth refreshes active rows, marks stale rows, and returns persisted health.
func (m *Manager) ListSessionHealth(
	ctx context.Context,
	query heartbeat.SessionHealthListQuery,
) ([]heartbeat.SessionHealth, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: health context is required")
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if _, err := m.refreshActiveSessionHealth(ctx); err != nil {
		return nil, err
	}
	if err := m.markStaleSessionHealth(ctx, m.now()); err != nil {
		return nil, err
	}
	if m.sessionHealthStore == nil {
		return m.listInMemorySessionHealth(query), nil
	}
	rows, err := m.sessionHealthStore.ListSessionHealth(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("session: list session health: %w", err)
	}
	return rows, nil
}

// RecoverSessionHealth recomputes persisted rows after daemon restart before wake decisions run.
func (m *Manager) RecoverSessionHealth(ctx context.Context) (HealthRecoveryResult, error) {
	if m == nil {
		return HealthRecoveryResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return HealthRecoveryResult{}, errors.New("session: health recovery context is required")
	}

	refreshed, err := m.refreshActiveSessionHealth(ctx)
	if err != nil {
		return HealthRecoveryResult{}, err
	}
	result := HealthRecoveryResult{RefreshedActive: refreshed}
	if m.sessionHealthStore == nil {
		return result, nil
	}

	rows, err := m.sessionHealthStore.ListSessionHealthRecoveryInputs(ctx, 0)
	if err != nil {
		return HealthRecoveryResult{}, fmt.Errorf("session: list health recovery inputs: %w", err)
	}
	now := m.now()
	for _, row := range rows {
		if strings.TrimSpace(row.SessionID) == "" {
			continue
		}
		if _, ok := m.Get(row.SessionID); ok {
			continue
		}
		meta, readErr := m.readMetaWithContext(ctx, row.SessionID)
		if readErr != nil {
			if errors.Is(readErr, ErrSessionNotFound) {
				next := staleRecoveredSessionHealth(row, now)
				if !sessionHealthEqual(row, next) {
					if _, storeErr := m.storeSessionHealth(ctx, next); storeErr != nil {
						return HealthRecoveryResult{}, storeErr
					}
					result.Recomputed++
				}
				continue
			}
			return HealthRecoveryResult{}, fmt.Errorf(
				"session: read health recovery metadata for %q: %w",
				row.SessionID,
				readErr,
			)
		}
		next := m.sessionHealthFromInfo(sessionInfoFromMeta(meta), row, now, sessionHealthInput{})
		if sessionHealthEqual(row, next) {
			continue
		}
		if _, storeErr := m.storeSessionHealth(ctx, next); storeErr != nil {
			return HealthRecoveryResult{}, storeErr
		}
		result.Recomputed++
	}

	marked, err := m.markStaleSessionHealthCount(ctx, now)
	if err != nil {
		return HealthRecoveryResult{}, err
	}
	result.MarkedStale = marked
	return result, nil
}

func (m *Manager) persistSessionPresence(
	ctx context.Context,
	session *Session,
	at time.Time,
) (heartbeat.SessionHealth, error) {
	prompting := session != nil && session.IsPrompting()
	return m.persistSessionHealthForSession(ctx, session, at, sessionHealthInput{
		activePrompt:  prompting,
		attachable:    sessionAttachableAt(session, at),
		touchPresence: !prompting,
	})
}

func (m *Manager) persistSessionIdlePresence(
	ctx context.Context,
	session *Session,
	at time.Time,
) (heartbeat.SessionHealth, error) {
	return m.persistSessionHealthForSession(ctx, session, at, sessionHealthInput{
		activePrompt:  false,
		attachable:    sessionAttachableAt(session, at),
		touchPresence: true,
	})
}

func (m *Manager) persistSessionPromptActivity(
	ctx context.Context,
	session *Session,
	activityAt time.Time,
) (heartbeat.SessionHealth, error) {
	if activityAt.IsZero() {
		activityAt = m.now()
	}
	return m.persistSessionHealthForSession(ctx, session, activityAt, sessionHealthInput{
		activePrompt: true,
		attachable:   sessionAttachableAt(session, activityAt),
		activityAt:   activityAt,
	})
}

func (m *Manager) persistSessionStoppedHealth(
	ctx context.Context,
	session *Session,
	at time.Time,
) (heartbeat.SessionHealth, error) {
	if at.IsZero() {
		at = m.now()
	}
	return m.persistSessionHealthForSession(ctx, session, at, sessionHealthInput{
		activePrompt: false,
		attachable:   false,
	})
}

func (m *Manager) persistSessionHealthForSession(
	ctx context.Context,
	session *Session,
	at time.Time,
	input sessionHealthInput,
) (heartbeat.SessionHealth, error) {
	if session == nil {
		return heartbeat.SessionHealth{}, errors.New("session: session is required")
	}
	if ctx == nil {
		return heartbeat.SessionHealth{}, errors.New("session: health context is required")
	}
	existing, err := m.storedSessionHealth(ctx, session.ID)
	if err != nil {
		return heartbeat.SessionHealth{}, err
	}
	health := m.sessionHealthFromInfo(session.Info(), existing, at, input)
	return m.storeSessionHealth(ctx, health)
}
