package task

import (
	"context"
	"fmt"
	"strings"
)

const wakeEventCacheMaxEntries = 4096

func (m *Service) reserveWakeEvent(ctx context.Context, taskID string, wakeEventID string) (bool, error) {
	key := wakeEventKey(taskID, wakeEventID)
	if key == "" {
		return false, fmt.Errorf("%w: wake event key is required", ErrValidation)
	}
	m.wakeMu.Lock()
	if _, ok := m.wakeEventIDs[key]; ok {
		m.wakeMu.Unlock()
		return false, nil
	}
	m.wakeMu.Unlock()

	recorded, err := m.hasWakeAuditEvent(ctx, taskID, wakeEventID)
	if err != nil {
		return false, err
	}

	m.wakeMu.Lock()
	defer m.wakeMu.Unlock()
	if _, ok := m.wakeEventIDs[key]; ok {
		return false, nil
	}
	if recorded {
		m.rememberWakeEventKeyLocked(key)
		return false, nil
	}
	m.rememberWakeEventKeyLocked(key)
	return true, nil
}

func (m *Service) rememberWakeEventKeyLocked(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	if _, ok := m.wakeEventIDs[key]; ok {
		return
	}
	for len(m.wakeEventIDs) >= wakeEventCacheMaxEntries && len(m.wakeEventOrder) > 0 {
		evicted := m.wakeEventOrder[0]
		delete(m.wakeEventIDs, evicted)
		copy(m.wakeEventOrder, m.wakeEventOrder[1:])
		m.wakeEventOrder[len(m.wakeEventOrder)-1] = ""
		m.wakeEventOrder = m.wakeEventOrder[:len(m.wakeEventOrder)-1]
	}
	if len(m.wakeEventIDs) >= wakeEventCacheMaxEntries {
		clear(m.wakeEventIDs)
		m.wakeEventOrder = m.wakeEventOrder[:0]
	}
	m.wakeEventIDs[key] = struct{}{}
	m.wakeEventOrder = append(m.wakeEventOrder, key)
}

func (m *Service) hasWakeAuditEvent(ctx context.Context, taskID string, wakeEventID string) (bool, error) {
	return m.store.TaskWakeEventExists(ctx, taskID, wakeEventID)
}
