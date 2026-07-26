package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/heartbeat"
)

// SessionHealthForPage derives current health for one bounded catalog page.
// Durable health is loaded in one query; no session metadata files are opened.
func (m *Manager) SessionHealthForPage(
	ctx context.Context,
	infos []*Info,
) (map[string]heartbeat.SessionHealth, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: health context is required")
	}
	if len(infos) > MaxListLimit {
		return nil, fmt.Errorf(
			"%w: session health page exceeds maximum %d",
			heartbeat.ErrInvalidSessionHealth,
			MaxListLimit,
		)
	}

	pageInfos := make(map[string]*Info, len(infos))
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		id := strings.TrimSpace(info.ID)
		if id == "" {
			return nil, fmt.Errorf("%w: session id is required", heartbeat.ErrInvalidSessionHealth)
		}
		if _, exists := pageInfos[id]; exists {
			continue
		}
		pageInfos[id] = info
		ids = append(ids, id)
	}

	storedByID := make(map[string]heartbeat.SessionHealth, len(ids))
	if m.sessionHealthStore != nil && len(ids) > 0 {
		rows, err := m.sessionHealthStore.ListSessionHealthByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("session: list page health: %w", err)
		}
		for _, health := range rows {
			normalized := health.Normalize()
			if err := normalized.Validate(); err != nil {
				return nil, fmt.Errorf("session: validate page health: %w", err)
			}
			if _, requested := pageInfos[normalized.SessionID]; requested {
				storedByID[normalized.SessionID] = normalized
			}
		}
	}

	now := m.now()
	healthByID := make(map[string]heartbeat.SessionHealth, len(ids))
	for _, id := range ids {
		info := pageInfos[id]
		if active, ok := m.Get(id); ok {
			healthByID[id] = m.sessionHealthFromInfo(active.Info(), storedByID[id], now, sessionHealthInput{
				activePrompt:  active.IsPrompting(),
				attachable:    sessionAttachableAt(active, now),
				touchPresence: true,
			})
			continue
		}
		if stored, exists := storedByID[id]; exists {
			healthByID[id] = stored
			continue
		}
		health := m.sessionHealthFromInfo(info, heartbeat.SessionHealth{}, now, sessionHealthInput{})
		if info.State == StateActive {
			health = staleRecoveredSessionHealth(health, now)
		}
		healthByID[id] = health
	}
	return healthByID, nil
}
