package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// publishAttentionCommit keeps observer failures from failing an already committed mutation.
func (m *Manager) publishAttentionCommit(ctx context.Context, sessionID string, commit store.SessionAttentionCommit) {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	unlock, err := m.lockAttentionCommit(readCtx, sessionID)
	var wake func()
	if err == nil {
		if m.attentionStore != nil {
			commit.After, err = m.attentionStore.GetSessionAttention(readCtx, sessionID)
		}
		if err == nil {
			wake, err = m.emitAttentionCommit(readCtx, sessionID, commit, false)
		}
		unlock()
	}
	if wake != nil {
		wake()
	}
	if err != nil {
		m.logger.WarnContext(
			ctx,
			"session: load attention event snapshot failed",
			"session_id",
			sessionID,
			"error",
			err,
		)
		if m.attentionStore != nil {
			m.PublishPendingInteractionCommits([]store.SessionAttentionCommit{{
				Before:      commit.Before,
				Interaction: &store.PendingInteraction{SessionID: sessionID},
			}}, nil)
		}
	}
}

// emitAttentionCommit returns snapshot errors before any session event is published.
func (m *Manager) emitAttentionCommit(
	ctx context.Context,
	sessionID string,
	commit store.SessionAttentionCommit,
	refreshCatalog bool,
) (func(), error) {
	m.attentionCommitMu.Lock()
	if m.attentionPublished == nil {
		m.attentionPublished = make(map[string]store.SessionAttention)
	}
	previous, published := m.attentionPublished[sessionID]
	if published {
		commit.Before = previous
	} else if sessionAttentionProjectionEqual(commit.Before, commit.After) && !refreshCatalog {
		m.attentionCommitMu.Unlock()
		return nil, nil
	} else {
		// Reserve before capturing Info so removal can invalidate this publication's baseline.
		m.attentionPublished[sessionID] = commit.Before
	}
	m.attentionCommitMu.Unlock()
	unchanged := sessionAttentionProjectionEqual(commit.Before, commit.After)
	if unchanged && !refreshCatalog {
		return nil, nil
	}
	info, err := m.sessionInfoForAttentionEvent(ctx, sessionID)
	if err != nil {
		if !published {
			m.attentionCommitMu.Lock()
			delete(m.attentionPublished, sessionID)
			m.attentionCommitMu.Unlock()
		}
		return nil, err
	}
	if unchanged {
		m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
		return nil, nil
	}
	before := *info
	after := *info
	applyAttentionToInfo(&before, commit.Before)
	applyAttentionToInfo(&after, commit.After)
	to, changed := m.publishAttentionEdge(ctx, &before, &after)
	if changed {
		return func() { m.dispatchSpawnWake(ctx, &after, to) }, nil
	}
	return nil, nil
}

// publishAttentionEdge publishes the catalog row and, when the badge changed, the
// canonical attention edge with its hooks. Creator wakes stay with the caller.
func (m *Manager) publishAttentionEdge(ctx context.Context, before *Info, after *Info) (Badge, bool) {
	m.rememberPublishedAttention(after)
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, after))
	from, to := BadgeForInfo(before), BadgeForInfo(after)
	if from == to {
		return to, false
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
	if !isInternalVerificationSession(after) {
		m.publishSessionCatalogEvent(sessionAttentionCatalogEvent(event))
	}
	m.dispatchSessionAttentionChanged(ctx, after, event)
	return to, true
}

// rememberPublishedAttention updates reserved baselines without reviving removed sessions.
func (m *Manager) rememberPublishedAttention(after *Info) {
	m.attentionCommitMu.Lock()
	if _, registered := m.attentionPublished[after.ID]; !registered {
		m.attentionCommitMu.Unlock()
		return
	}
	m.attentionPublished[after.ID] = store.SessionAttention{
		PendingPermissionCount: after.PendingPermissionCount,
		PendingClarifyCount:    after.PendingClarifyCount,
		AttentionRevision:      after.AttentionRevision,
		LastSettledRevision:    after.LastSettledRevision,
		LastSeenRevision:       after.LastSeenRevision,
		LastSeenAt:             cloneTimePointer(after.LastSeenAt),
		AttentionChangedAt:     cloneTimePointer(after.AttentionChangedAt),
		EndedEarlyTurnID:       after.EndedEarlyTurnID,
		FailureSeenAt:          cloneTimePointer(after.FailureSeenAt),
	}
	m.attentionCommitMu.Unlock()
}

func sessionAttentionProjectionEqual(left, right store.SessionAttention) bool {
	return left.PendingPermissionCount == right.PendingPermissionCount &&
		left.PendingClarifyCount == right.PendingClarifyCount &&
		left.AttentionRevision == right.AttentionRevision &&
		left.LastSettledRevision == right.LastSettledRevision &&
		left.LastSeenRevision == right.LastSeenRevision &&
		timePointersEqual(left.LastSeenAt, right.LastSeenAt) &&
		timePointersEqual(left.AttentionChangedAt, right.AttentionChangedAt) &&
		left.EndedEarlyTurnID == right.EndedEarlyTurnID &&
		timePointersEqual(left.FailureSeenAt, right.FailureSeenAt)
}

func timePointersEqual(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

// sessionInfoForAttentionEvent bounds snapshot reads within the caller's publication lifetime.
func (m *Manager) sessionInfoForAttentionEvent(ctx context.Context, sessionID string) (*Info, error) {
	if active, ok := m.Get(sessionID); ok {
		return active.Info(), nil
	}
	if m.sessionCatalog == nil {
		return nil, fmt.Errorf("session: attention event catalog unavailable for %q", sessionID)
	}
	queryCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
	defer cancel()
	rows, err := m.sessionCatalog.ListSessions(queryCtx, store.SessionListQuery{
		ReadScope: store.ReadScope{AllProfiles: true},
		ID:        strings.TrimSpace(sessionID),
		Archive:   store.SessionArchiveInclude,
		Limit:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("session: read durable attention event snapshot: %w", err)
	}
	for index := range rows {
		if strings.TrimSpace(rows[index].ID) == strings.TrimSpace(sessionID) {
			return sessionInfoFromCatalog(&rows[index]), nil
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
	info.EndedEarlyTurnID = projection.EndedEarlyTurnID
	info.FailureSeenAt = cloneTimePointer(projection.FailureSeenAt)
	info.PendingPermission = projection.PendingPermissionCount > 0
}

func errorsForAttentionSnapshot(sessionID string) error {
	return fmt.Errorf(
		"%w: attention event snapshot not found for %q",
		store.ErrSessionNotFound,
		strings.TrimSpace(sessionID),
	)
}

func isInternalVerificationSession(info *Info) bool {
	return info != nil && normalizeSessionType(info.Type) == SessionTypeSystem && info.VerdictOnly
}
