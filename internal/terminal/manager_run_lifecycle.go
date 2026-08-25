package terminal

import (
	"context"
	"strings"
)

// RunEnded releases every terminal still controlled by the completed run and resolves its pending input.
func (m *Service) RunEnded(_ context.Context, actor Actor) int {
	if !validRunActor(actor) {
		return 0
	}
	items := m.sessionsForRun(actor.ProfileID, actor.SessionID, actor.RunID)
	changed := 0
	for _, item := range items {
		if item.runEnded(actor) {
			changed++
		}
	}
	return changed
}

// SessionRunEnded releases agent-controlled terminals for one completed session generation.
func (m *Service) SessionRunEnded(_ context.Context, profileID, sessionID string, generation int64) int {
	if strings.TrimSpace(profileID) == "" || strings.TrimSpace(sessionID) == "" || generation <= 0 {
		return 0
	}
	items := m.sessionsForSession(profileID, sessionID)
	changed := 0
	for _, item := range items {
		info := item.Info()
		if info.Controller == nil || info.Controller.Kind != ActorKindAgent ||
			info.Controller.Generation != generation {
			continue
		}
		if item.runEnded(*info.Controller) {
			changed++
		}
	}
	return changed
}

// RuntimeRecovered fences the previous generation and makes the replacement generation reclaimable.
func (m *Service) RuntimeRecovered(_ context.Context, previous, current Actor) int {
	if !validRunActor(previous) || !validRunActor(current) || !sameRun(previous, current) ||
		current.Generation <= previous.Generation {
		return 0
	}
	items := m.sessionsForRun(previous.ProfileID, previous.SessionID, previous.RunID)
	changed := 0
	for _, item := range items {
		if item.runtimeRecovered(previous, current) {
			changed++
		}
	}
	return changed
}

// Claim grants an available or recovery-fenced terminal back to its bound agent generation.
func (m *Service) Claim(
	_ context.Context,
	workspaceID string,
	id ID,
	actor Actor,
) error {
	item, err := m.lookup(terminalKey{workspaceID: workspaceID, profileID: actor.ProfileID, id: id})
	if err != nil {
		return err
	}
	return item.claim(actor)
}

func (m *Service) sessionsForRun(profileID, sessionID, runID string) []*session {
	items := m.sessionsForSession(profileID, sessionID)
	filtered := make([]*session, 0, len(items))
	for _, item := range items {
		info := item.Info()
		if info.BoundRun != nil && info.BoundRun.RunID == runID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m *Service) sessionsForSession(profileID, sessionID string) []*session {
	m.mu.RLock()
	candidates := make([]*session, 0)
	for key, item := range m.terminals {
		if key.profileID == profileID {
			candidates = append(candidates, item)
		}
	}
	m.mu.RUnlock()
	items := make([]*session, 0, len(candidates))
	for _, item := range candidates {
		info := item.Info()
		if info.BoundRun != nil && info.BoundRun.SessionID == sessionID {
			items = append(items, item)
		}
	}
	return items
}

func validRunActor(actor Actor) bool {
	return actor.Kind == ActorKindAgent && strings.TrimSpace(actor.ProfileID) != "" &&
		strings.TrimSpace(actor.SessionID) != "" && actor.Generation > 0
}
