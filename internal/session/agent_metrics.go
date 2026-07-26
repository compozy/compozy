package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

// AgentSessionMetrics is the exact visible-session aggregate for one agent.
type AgentSessionMetrics struct {
	Total          int
	Active         int
	Failed         int
	RuntimeSeconds int64
	LastActivityAt time.Time
}

// AggregateSessionsByAgent returns workspace-scoped durable metrics overlaid with
// the manager's current live-session snapshots.
func (m *Manager) AggregateSessionsByAgent(
	ctx context.Context,
	workspaceID string,
) (map[string]AgentSessionMetrics, error) {
	if ctx == nil {
		return nil, errors.New("session: aggregate sessions by agent context is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("session: aggregate sessions by agent workspace id is required")
	}
	reader, ok := m.sessionCatalog.(store.SessionAgentMetricsReader)
	if !ok || reader == nil {
		return nil, errors.New("session: grouped agent session metrics reader is required")
	}

	_, activeIDs, activeMatches := m.activeSessionCatalogRows(ListQuery{
		WorkspaceID: workspaceID,
	})
	durable, err := reader.AggregateSessionsByAgent(ctx, store.SessionAgentMetricsQuery{
		WorkspaceID:         workspaceID,
		ExcludeIDs:          activeIDs,
		ExcludeSessionTypes: []string{string(SessionTypeDream)},
		ExcludeSpawnRoles:   []string{SpawnRoleMemoryExtractor, SpawnRoleAutoTitle},
	})
	if err != nil {
		return nil, fmt.Errorf("session: aggregate durable sessions by agent: %w", err)
	}

	metrics := make(map[string]AgentSessionMetrics, len(durable)+len(activeMatches))
	for _, durableMetric := range durable {
		name := strings.TrimSpace(durableMetric.AgentName)
		if name == "" {
			continue
		}
		metrics[name] = AgentSessionMetrics{
			Total:          durableMetric.Total,
			Active:         durableMetric.Active,
			Failed:         durableMetric.Failed,
			RuntimeSeconds: durableMetric.RuntimeSeconds,
			LastActivityAt: durableMetric.LastActivityAt,
		}
	}
	now := m.now().UTC()
	for _, active := range activeMatches {
		name := strings.TrimSpace(active.AgentName)
		if name == "" {
			continue
		}
		metric := metrics[name]
		metric.Total++
		if strings.TrimSpace(active.State) == string(StateActive) {
			metric.Active++
		}
		if storeSessionInfoFailed(active) {
			metric.Failed++
		}
		metric.RuntimeSeconds += sessionRuntimeSeconds(
			active.CreatedAt,
			active.UpdatedAt,
			State(strings.TrimSpace(active.State)),
			now,
		)
		activityAt := storeSessionInfoLastActivityAt(active)
		if activityAt.After(metric.LastActivityAt) {
			metric.LastActivityAt = activityAt
		}
		metrics[name] = metric
	}
	return metrics, nil
}

func storeSessionInfoFailed(info store.SessionInfo) bool {
	if strings.TrimSpace(info.State) != string(StateStopped) {
		return false
	}
	return info.Failure != nil || info.StopReason == store.StopAgentCrashed ||
		info.StopReason == store.StopError
}

func sessionRuntimeSeconds(createdAt time.Time, updatedAt time.Time, state State, now time.Time) int64 {
	if createdAt.IsZero() {
		return 0
	}
	end := updatedAt
	if state == StateActive {
		end = now
	}
	if end.Before(createdAt) {
		return 0
	}
	return int64(end.Sub(createdAt).Seconds())
}

func storeSessionInfoLastActivityAt(info store.SessionInfo) time.Time {
	if info.Liveness != nil && info.Liveness.LastUpdateAt != nil &&
		!info.Liveness.LastUpdateAt.IsZero() {
		return info.Liveness.LastUpdateAt.UTC()
	}
	return info.UpdatedAt.UTC()
}
