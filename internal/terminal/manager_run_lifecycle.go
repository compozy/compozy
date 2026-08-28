package terminal

import (
	"context"
	"strings"
)

// RunEnded releases every terminal still controlled by the completed run and resolves its pending input.
func (m *Service) RunEnded(_ context.Context, workspaceID string, actor Actor) int {
	if strings.TrimSpace(workspaceID) == "" || !validRunActor(actor) {
		return 0
	}
	items := m.sessionsForRun(workspaceID, actor.ProfileID, actor.SessionID, actor.RunID)
	changed := 0
	for _, item := range items {
		if item.runEnded(actor) {
			changed++
		}
	}
	return changed
}

// SessionRunEnded releases agent-controlled terminals for one completed session generation.
func (m *Service) SessionRunEnded(
	_ context.Context,
	workspaceID, profileID, sessionID, runID string,
	generation int64,
) int {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(profileID) == "" ||
		strings.TrimSpace(sessionID) == "" || strings.TrimSpace(runID) == "" || generation <= 0 {
		return 0
	}
	items := m.sessionsForSession(workspaceID, profileID, sessionID)
	changed := 0
	for _, item := range items {
		info := item.Info()
		if info.Controller == nil || info.Controller.Kind != ActorKindAgent || info.Controller.RunID != runID ||
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
func (m *Service) RuntimeRecovered(_ context.Context, workspaceID string, previous, current Actor) int {
	if !validRunActor(previous) || !validRunActor(current) || !sameRun(previous, current) ||
		strings.TrimSpace(workspaceID) == "" || current.Generation <= previous.Generation {
		return 0
	}
	items := m.sessionsForRun(workspaceID, previous.ProfileID, previous.SessionID, previous.RunID)
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
	ctx context.Context,
	workspaceID string,
	id ID,
	actor Actor,
) error {
	if err := requestContextError(ctx, "claim"); err != nil {
		return err
	}
	if err := m.admit(ctx, workspaceID, actor); err != nil {
		return err
	}
	item, err := m.lookup(terminalKey{workspaceID: workspaceID, profileID: actor.ProfileID, id: id})
	if err != nil {
		return err
	}
	return item.claim(actor)
}

func (m *Service) sessionsForRun(workspaceID, profileID, sessionID, runID string) []*session {
	items := m.sessionsForSession(workspaceID, profileID, sessionID)
	filtered := make([]*session, 0, len(items))
	for _, item := range items {
		info := item.Info()
		if info.BoundRun != nil && info.BoundRun.RunID == runID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m *Service) sessionsForSession(workspaceID, profileID, sessionID string) []*session {
	m.mu.RLock()
	candidates := make([]*session, 0)
	for key, item := range m.terminals {
		if key.workspaceID == workspaceID && key.profileID == profileID {
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
		strings.TrimSpace(actor.SessionID) != "" && strings.TrimSpace(actor.RunID) != "" && actor.Generation > 0
}
