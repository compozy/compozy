package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const SessionPresenceLeaseTTL = 15 * time.Second

var (
	ErrSessionPresenceLeaseNotFound = errors.New("session: presence lease not found")
	ErrSessionPresenceLeaseMismatch = errors.New("session: presence lease belongs to another session")
)

type sessionPresenceKey struct {
	sessionID string
	leaseID   string
}

type sessionPresenceLease struct {
	expiresAt time.Time
}

// SessionPresence acquires, renews, or releases one opaque per-client lease.
func (m *Manager) SessionPresence(
	ctx context.Context,
	sessionID string,
	leaseID string,
	visible bool,
) (string, error) {
	if m == nil {
		return "", errors.New("session: manager is required")
	}
	if ctx == nil {
		return "", errors.New("session: presence context is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return "", errors.New("session: presence session id is required")
	}
	if m.attentionStore == nil {
		return "", errors.New("session: attention store is required")
	}
	if _, err := m.attentionStore.GetSessionAttention(ctx, target); err != nil {
		return "", err
	}
	now := m.now().UTC()
	m.presenceMu.Lock()
	m.pruneExpiredPresenceLeasesLocked(now)
	resultID, commit, err := m.updatePresenceLeaseLocked(ctx, target, strings.TrimSpace(leaseID), visible, now)
	m.presenceMu.Unlock()
	if err != nil {
		return "", err
	}
	if commit != nil {
		m.publishAttentionCommit(ctx, target, *commit)
	}
	return resultID, nil
}

func (m *Manager) updatePresenceLeaseLocked(
	ctx context.Context,
	sessionID string,
	leaseID string,
	visible bool,
	now time.Time,
) (string, *store.SessionAttentionCommit, error) {
	if !visible {
		if leaseID == "" {
			return "", nil, errors.New("session: presence lease id is required for release")
		}
		key := sessionPresenceKey{sessionID: sessionID, leaseID: leaseID}
		if _, exists := m.presenceLeases[key]; !exists {
			return "", nil, m.classifyPresenceLeaseMissLocked(sessionID, leaseID)
		}
		delete(m.presenceLeases, key)
		return leaseID, nil, nil
	}
	if leaseID == "" {
		generated, err := m.newPresenceLeaseID()
		if err != nil {
			return "", nil, err
		}
		leaseID = strings.TrimSpace(generated)
		if leaseID == "" {
			return "", nil, errors.New("session: generated presence lease id is empty")
		}
	} else {
		key := sessionPresenceKey{sessionID: sessionID, leaseID: leaseID}
		if _, exists := m.presenceLeases[key]; !exists {
			return "", nil, m.classifyPresenceLeaseMissLocked(sessionID, leaseID)
		}
	}
	key := sessionPresenceKey{sessionID: sessionID, leaseID: leaseID}
	previous, existed := m.presenceLeases[key]
	m.presenceLeases[key] = sessionPresenceLease{expiresAt: now.Add(SessionPresenceLeaseTTL)}
	commit, err := m.attentionStore.MarkSessionSeen(ctx, sessionID, now)
	if err != nil {
		if existed {
			m.presenceLeases[key] = previous
		} else {
			delete(m.presenceLeases, key)
		}
		return "", nil, fmt.Errorf("session: mark visible session seen: %w", err)
	}
	m.applyAttentionProjection(sessionID, commit.After)
	return leaseID, &commit, nil
}

func (m *Manager) classifyPresenceLeaseMissLocked(sessionID string, leaseID string) error {
	for key := range m.presenceLeases {
		if key.leaseID == leaseID && key.sessionID != sessionID {
			return fmt.Errorf("%w: %s", ErrSessionPresenceLeaseMismatch, leaseID)
		}
	}
	return fmt.Errorf("%w: %s", ErrSessionPresenceLeaseNotFound, leaseID)
}

func (m *Manager) pruneExpiredPresenceLeasesLocked(now time.Time) {
	for key, lease := range m.presenceLeases {
		if !lease.expiresAt.After(now) {
			delete(m.presenceLeases, key)
		}
	}
}

func (m *Manager) hasLivePresenceLocked(sessionID string, now time.Time) bool {
	m.pruneExpiredPresenceLeasesLocked(now)
	for key := range m.presenceLeases {
		if key.sessionID == sessionID {
			return true
		}
	}
	return false
}

func (m *Manager) settleSessionAttention(
	ctx context.Context,
	sessionID string,
	at time.Time,
) error {
	if m.attentionStore == nil {
		return nil
	}
	if at.IsZero() {
		at = m.now().UTC()
	}
	m.presenceMu.Lock()
	seen := m.hasLivePresenceLocked(sessionID, at)
	commit, err := m.attentionStore.MarkSessionSettled(ctx, sessionID, seen, at)
	if err == nil {
		m.applyAttentionProjection(sessionID, commit.After)
	}
	m.presenceMu.Unlock()
	if err != nil {
		return fmt.Errorf("session: settle attention for %q: %w", sessionID, err)
	}
	m.publishAttentionCommit(ctx, sessionID, commit)
	return nil
}
