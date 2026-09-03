package session

import (
	"context"
	"errors"
	"strings"
)

// ErrPromptNotActive reports that no authoritative prompt run is published for the session.
var ErrPromptNotActive = errors.New("session: prompt is not active")

// ActivePromptRun returns the complete identity of the session's active prompt.
func (m *Manager) ActivePromptRun(ctx context.Context, sessionID string) (PromptRunIdentity, error) {
	if ctx == nil {
		return PromptRunIdentity{}, errors.New("session: active prompt context is required")
	}
	if err := ctx.Err(); err != nil {
		return PromptRunIdentity{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return PromptRunIdentity{}, errors.New("session: session id is required")
	}
	m.mu.RLock()
	identity, ok := m.activePromptRuns[target]
	m.mu.RUnlock()
	if !ok {
		return PromptRunIdentity{}, ErrPromptNotActive
	}
	return identity, nil
}

func (m *Manager) publishOrReplaceActivePromptRun(session *Session, state *promptTurnDispatchState) error {
	identity, err := promptRunIdentity(session, state)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activePromptRuns == nil {
		m.activePromptRuns = make(map[string]PromptRunIdentity)
	}
	current, exists := m.activePromptRuns[identity.SessionID]
	if exists && current.RunID != identity.RunID {
		return ErrPromptInProgress
	}
	m.activePromptRuns[identity.SessionID] = identity
	return nil
}

func (m *Manager) clearActivePromptRunForState(session *Session, state *promptTurnDispatchState) {
	identity, err := promptRunIdentity(session, state)
	if err == nil {
		m.clearActivePromptRun(identity)
	}
}

func (m *Manager) clearActivePromptRun(identity PromptRunIdentity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.activePromptRuns[identity.SessionID]
	if ok && current.RunID == identity.RunID && current.Generation == identity.Generation {
		delete(m.activePromptRuns, identity.SessionID)
	}
}

func (m *Manager) activePromptRunSnapshot(sessionID string) (PromptRunIdentity, bool) {
	m.mu.RLock()
	identity, ok := m.activePromptRuns[strings.TrimSpace(sessionID)]
	m.mu.RUnlock()
	return identity, ok
}

func promptRunIdentity(session *Session, state *promptTurnDispatchState) (PromptRunIdentity, error) {
	if session == nil || state == nil {
		return PromptRunIdentity{}, errors.New("session: prompt run identity is unavailable")
	}
	info := session.Info()
	identity := PromptRunIdentity{
		WorkspaceID: strings.TrimSpace(info.WorkspaceID),
		ProfileID:   strings.TrimSpace(info.ProfileID),
		SessionID:   strings.TrimSpace(info.ID),
		RunID:       strings.TrimSpace(state.runID),
		Generation:  state.generation,
	}
	if identity.WorkspaceID == "" || identity.ProfileID == "" || identity.SessionID == "" ||
		identity.RunID == "" || identity.Generation <= 0 {
		return PromptRunIdentity{}, errors.New("session: prompt run identity is incomplete")
	}
	return identity, nil
}
