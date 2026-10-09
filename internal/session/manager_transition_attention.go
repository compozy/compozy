package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) durableSessionInfoForLifecycleTransition(
	ctx context.Context,
	sessionID string,
) (*Info, error) {
	rows, err := m.sessionCatalog.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{AllProfiles: true},
		ID:        strings.TrimSpace(sessionID),
		Archive:   store.SessionArchiveInclude,
		Limit:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("session: read lifecycle attention snapshot: %w", err)
	}
	for index := range rows {
		if strings.TrimSpace(rows[index].ID) == strings.TrimSpace(sessionID) {
			return sessionInfoFromCatalog(&rows[index]), nil
		}
	}
	return nil, fmt.Errorf("session: lifecycle attention snapshot not found for %q", sessionID)
}

func (m *Manager) publishLifecycleAttentionTransition(
	ctx context.Context,
	before *Info,
	after *Info,
) {
	m.publishLifecycleAttentionTransitionWake(ctx, before, after, true)
}

// publishLifecycleAttentionTransitionWake publishes the canonical attention edge
// and optionally dispatches the completed spawn wake. A canceled child turn still
// emits its catalog event, wait-badge edge, and attention hooks, but must not wake
// its creator with a false "completed" reason.
func (m *Manager) publishLifecycleAttentionTransitionWake(
	ctx context.Context,
	before *Info,
	after *Info,
	deliverCompletedWake bool,
) {
	if after == nil {
		return
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, after))
	from, to := BadgeForInfo(before), BadgeForInfo(after)
	if from == to {
		return
	}
	m.publishSubagentLifecycleEdge(ctx, before, after)
	at := m.now().UTC()
	if after.AttentionChangedAt != nil {
		at = after.AttentionChangedAt.UTC()
	}
	event := AttentionEvent{
		SessionID:   strings.TrimSpace(after.ID),
		ProfileID:   strings.TrimSpace(after.ProfileID),
		WorkspaceID: strings.TrimSpace(after.WorkspaceID),
		From:        from,
		To:          to,
		Class:       ClassForBadge(to),
		At:          at,
	}
	m.publishWaitBadgeEdge(after, to)
	m.publishSessionCatalogEvent(sessionAttentionCatalogEvent(event))
	m.dispatchSessionAttentionChanged(ctx, after, event)
	if !deliverCompletedWake && to == BadgeDone {
		return
	}
	m.dispatchSpawnWake(ctx, after, to)
}
