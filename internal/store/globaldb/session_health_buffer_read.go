package globaldb

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
)

func (b *sessionHealthBuffer) listLocked(
	ctx context.Context, query heartbeat.SessionHealthListQuery,
) ([]heartbeat.SessionHealth, error) {
	persistedQuery := query
	if persistedQuery.Limit > 0 {
		persistedQuery.Limit += len(b.pending)
	}
	repo := &HeartbeatRepo{repoBase: b.base}
	rows, err := repo.listSessionHealthRows(ctx, persistedQuery)
	if err != nil {
		return nil, err
	}
	rows, err = b.mergeLocked(ctx, rows, func(health heartbeat.SessionHealth) bool {
		return matchesSessionHealth(health, query)
	})
	if query.Limit > 0 && len(rows) > query.Limit {
		rows = rows[:query.Limit]
	}
	return rows, err
}

func (b *sessionHealthBuffer) mergeLocked(
	ctx context.Context, rows []heartbeat.SessionHealth, include func(heartbeat.SessionHealth) bool,
) ([]heartbeat.SessionHealth, error) {
	ids := make([]string, 0, len(b.pending))
	for id, entry := range b.pending {
		if include(entry.health) {
			ids = append(ids, id)
		}
	}
	owned := make(map[string]string, len(ids))
	for start := 0; start < len(ids); start += maxSessionHealthBatchSize {
		end := min(start+maxSessionHealthBatchSize, len(ids))
		owners, err := b.base.readQueries.ListSessionHealthOwners(ctx, ids[start:end])
		if err != nil {
			return nil, err
		}
		for _, owner := range owners {
			owned[owner.ID] = owner.WorkspaceID
		}
	}
	merged := make([]heartbeat.SessionHealth, 0, len(rows)+len(ids))
	for _, row := range rows {
		if _, overridden := b.pending[row.SessionID]; !overridden {
			merged = append(merged, row)
		}
	}
	for _, id := range ids {
		health := b.pending[id].health
		if workspace, exists := owned[id]; exists && workspace == health.WorkspaceID {
			merged = append(merged, health)
		}
	}
	slices.SortFunc(merged, func(a, b heartbeat.SessionHealth) int {
		return cmp.Or(b.UpdatedAt.Compare(a.UpdatedAt), strings.Compare(b.SessionID, a.SessionID))
	})
	return merged, nil
}

func matchesSessionHealth(health heartbeat.SessionHealth, query heartbeat.SessionHealthListQuery) bool {
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	query.AgentName = strings.TrimSpace(query.AgentName)
	query.SessionID = strings.TrimSpace(query.SessionID)
	return (query.WorkspaceID == "" || health.WorkspaceID == query.WorkspaceID) &&
		(query.AgentName == "" || health.AgentName == query.AgentName) &&
		(query.SessionID == "" || health.SessionID == query.SessionID) &&
		(query.State == "" || health.State == query.State) &&
		(query.Health == "" || health.Health == query.Health) &&
		(query.EligibleForWake == nil || health.EligibleForWake == *query.EligibleForWake)
}

func (b *sessionHealthBuffer) markStale(ctx context.Context, cutoff, updatedAt time.Time) (int64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.base.closed.Load() != 0 {
		return 0, store.ErrClosed
	}
	rows, err := b.listLocked(ctx, heartbeat.SessionHealthListQuery{State: heartbeat.SessionHealthStateIdle})
	if err != nil {
		return 0, err
	}
	var affected int64
	for _, health := range rows {
		if health.ActivePrompt || health.Health == heartbeat.SessionHealthStale ||
			health.Health == heartbeat.SessionHealthDead ||
			(!health.LastPresenceAt.IsZero() && !health.LastPresenceAt.Before(cutoff)) {
			continue
		}
		health.Health = heartbeat.SessionHealthStale
		health.EligibleForWake = false
		health.IneligibilityReason = string(heartbeat.SessionHealthReasonStale)
		health.UpdatedAt = updatedAt
		b.putLocked(health)
		affected++
	}
	return affected, nil
}
