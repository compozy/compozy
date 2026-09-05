package terminal

import (
	"context"
	"strings"
)

// RunEnded resolves pending input requests created by a completed run.
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

// SessionRunEnded resolves pending input requests for one completed session generation.
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
		if info.BoundRun == nil || info.BoundRun.RunID != runID || info.BoundRun.Generation != generation {
			continue
		}
		actor := Actor{
			Kind: ActorKindAgent, ID: "runtime", ProfileID: profileID,
			SessionID: sessionID, RunID: runID, Generation: generation,
		}
		if item.runEnded(actor) {
			changed++
		}
	}
	return changed
}

// RuntimeRecovered advances provenance to the replacement runtime generation.
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
