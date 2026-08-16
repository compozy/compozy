package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) publishAttentionCommit(
	ctx context.Context,
	sessionID string,
	commit store.SessionAttentionCommit,
) {
	info, err := m.sessionInfoForAttentionEvent(ctx, sessionID)
	if err != nil {
		m.logger.WarnContext(
			ctx,
			"session: load attention event snapshot failed",
			"session_id", sessionID,
			"error", err,
		)
		return
	}
	before := *info
	after := *info
	applyAttentionToInfo(&before, commit.Before)
	applyAttentionToInfo(&after, commit.After)

	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, &after))
	from, to := BadgeForInfo(&before), BadgeForInfo(&after)
	if from == to {
		return
	}
	at := m.now().UTC()
	if commit.After.AttentionChangedAt != nil {
		at = commit.After.AttentionChangedAt.UTC()
	}
	event := AttentionEvent{
		SessionID:   strings.TrimSpace(after.ID),
		WorkspaceID: strings.TrimSpace(after.WorkspaceID),
		From:        from,
		To:          to,
		Class:       ClassForBadge(to),
		At:          at,
	}
	m.publishSessionCatalogEvent(sessionAttentionCatalogEvent(event))
	m.dispatchSessionAttentionChanged(ctx, &after, event)
}

func (m *Manager) sessionInfoForAttentionEvent(ctx context.Context, sessionID string) (*Info, error) {
	if active, ok := m.Get(sessionID); ok {
		return active.Info(), nil
	}
	if m.sessionCatalog == nil {
		return nil, errorsForAttentionSnapshot(sessionID)
	}
	queryCtx := ctx
	if queryCtx == nil {
		queryCtx = context.Background()
	} else {
		queryCtx = context.WithoutCancel(queryCtx)
	}
	rows, err := m.sessionCatalog.ListSessions(queryCtx, store.SessionListQuery{
		ID:      strings.TrimSpace(sessionID),
		Archive: store.SessionArchiveInclude,
		Limit:   1,
	})
	if err != nil {
		return nil, fmt.Errorf("session: read durable attention event snapshot: %w", err)
	}
	for index := range rows {
		if strings.TrimSpace(rows[index].ID) == strings.TrimSpace(sessionID) {
			return sessionInfoFromCatalog(rows[index]), nil
		}
	}
	return nil, errorsForAttentionSnapshot(sessionID)
}

func applyAttentionToInfo(info *Info, projection store.SessionAttention) {
	if info == nil {
		return
	}
	info.PendingPermissionCount = projection.PendingPermissionCount
	info.PendingClarifyCount = projection.PendingClarifyCount
	info.AttentionRevision = projection.AttentionRevision
	info.LastSettledRevision = projection.LastSettledRevision
	info.LastSeenRevision = projection.LastSeenRevision
	info.LastSeenAt = cloneTimePointer(projection.LastSeenAt)
	info.AttentionChangedAt = cloneTimePointer(projection.AttentionChangedAt)
	info.PendingPermission = projection.PendingPermissionCount > 0
}

func errorsForAttentionSnapshot(sessionID string) error {
	return fmt.Errorf("session: attention event snapshot not found for %q", strings.TrimSpace(sessionID))
}
