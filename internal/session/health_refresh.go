package session

import (
	"context"

	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
)

func (m *Manager) refreshActiveSessionHealth(ctx context.Context) (int, error) {
	active := m.List()
	refreshed := 0
	for _, info := range active {
		if info == nil {
			continue
		}
		session, ok := m.Get(info.ID)
		if !ok {
			continue
		}
		if _, err := m.persistSessionPresence(ctx, session, m.now()); err != nil {
			return refreshed, err
		}
		refreshed++
	}
	return refreshed, nil
}

func (m *Manager) markStaleSessionHealth(ctx context.Context, now time.Time) error {
	_, err := m.markStaleSessionHealthCount(ctx, now)
	return err
}

func (m *Manager) markStaleSessionHealthCount(ctx context.Context, now time.Time) (int64, error) {
	if m.sessionHealthStore == nil {
		return 0, nil
	}
	if now.IsZero() {
		now = m.now()
	}
	cutoff := now.UTC().Add(-m.sessionHealthStaleAfter)
	marked, err := m.sessionHealthStore.MarkSessionHealthStale(ctx, cutoff, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("session: mark stale health rows: %w", err)
	}
	return marked, nil
}

func (m *Manager) listInMemorySessionHealth(
	query heartbeat.SessionHealthListQuery,
) []heartbeat.SessionHealth {
	active := m.List()
	rows := make([]heartbeat.SessionHealth, 0, len(active))
	now := m.now()
	for _, info := range active {
		if info == nil {
			continue
		}
		health := m.sessionHealthFromInfo(info, heartbeat.SessionHealth{}, now, sessionHealthInput{
			activePrompt:  info.Liveness != nil && info.Liveness.Activity != nil,
			attachable:    info.State == StateActive,
			touchPresence: true,
		})
		if !sessionHealthMatchesQuery(health, query) {
			continue
		}
		rows = append(rows, health)
	}
	return rows
}

func sessionHealthMatchesQuery(health heartbeat.SessionHealth, query heartbeat.SessionHealthListQuery) bool {
	switch {
	case strings.TrimSpace(query.WorkspaceID) != "" && health.WorkspaceID != strings.TrimSpace(query.WorkspaceID):
		return false
	case strings.TrimSpace(query.AgentName) != "" && health.AgentName != strings.TrimSpace(query.AgentName):
		return false
	case strings.TrimSpace(query.SessionID) != "" && health.SessionID != strings.TrimSpace(query.SessionID):
		return false
	case query.State != "" && health.State != query.State:
		return false
	case query.Health != "" && health.Health != query.Health:
		return false
	case query.EligibleForWake != nil && health.EligibleForWake != *query.EligibleForWake:
		return false
	default:
		return true
	}
}

func sessionAttachableAt(session *Session, now time.Time) bool {
	if session == nil {
		return false
	}
	info := session.Info()
	if !AttachableForInfo(info, now) {
		return false
	}
	proc := session.processHandle()
	return proc != nil && !isProcessDone(proc)
}

func infoLastActivityAt(info *Info) time.Time {
	if info == nil || info.Liveness == nil {
		return time.Time{}
	}
	if info.Liveness.Activity != nil &&
		info.Liveness.Activity.LastActivityAt != nil &&
		!info.Liveness.Activity.LastActivityAt.IsZero() {
		return info.Liveness.Activity.LastActivityAt.UTC()
	}
	return time.Time{}
}

func sessionHealthEqual(left heartbeat.SessionHealth, right heartbeat.SessionHealth) bool {
	left = left.Normalize()
	right = right.Normalize()
	return left.SessionID == right.SessionID &&
		left.WorkspaceID == right.WorkspaceID &&
		left.AgentName == right.AgentName &&
		left.State == right.State &&
		left.Health == right.Health &&
		left.ActivePrompt == right.ActivePrompt &&
		left.Attachable == right.Attachable &&
		left.EligibleForWake == right.EligibleForWake &&
		left.IneligibilityReason == right.IneligibilityReason &&
		left.LastError == right.LastError &&
		left.LastActivityAt.UTC().Equal(right.LastActivityAt.UTC()) &&
		left.LastPresenceAt.UTC().Equal(right.LastPresenceAt.UTC())
}
