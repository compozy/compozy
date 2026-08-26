package terminal

import (
	"context"
	"errors"
	"slices"
	"time"
)

func (m *Service) reaper(ctx context.Context, period time.Duration) {
	defer close(m.reaperDone)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reap(ctx)
		}
	}
}

func (m *Service) reap(ctx context.Context) {
	now := m.now()
	type reapTarget struct {
		key    terminalKey
		item   *session
		reason string
	}
	m.mu.RLock()
	snapshot := make([]reapTarget, 0, len(m.terminals))
	for key, item := range m.terminals {
		snapshot = append(snapshot, reapTarget{key: key, item: item})
	}
	m.mu.RUnlock()
	targets := make([]reapTarget, 0)
	for _, candidate := range snapshot {
		item := candidate.item
		settings := item.settings(ctx)
		if exitAt, exited := item.exitAt(); exited {
			if !exitAt.IsZero() && now.Sub(exitAt) >= settings.ExitRetention {
				candidate.reason = "exit_retention"
				targets = append(targets, candidate)
			}
			continue
		}
		detached, activity := item.detachedSince()
		if detached && now.Sub(activity) >= settings.DetachedTTL {
			candidate.reason = "detached_ttl"
			targets = append(targets, candidate)
		}
	}
	slices.SortFunc(targets, func(left, right reapTarget) int {
		return compareTerminalKeys(left.key, right.key)
	})
	for _, target := range targets {
		if target.reason == "detached_ttl" {
			settings := target.item.settings(ctx)
			if !target.item.claimDetachedReap(now, settings.DetachedTTL) {
				continue
			}
			actor := Actor{Kind: ActorKindSystem, ID: "terminal-reaper", ProfileID: target.key.profileID}
			if _, err := target.item.close(ctx, SignalHUP, "expired", actor); err != nil && !errors.Is(err, ErrExited) {
				target.item.cancelDetachedReap()
				m.logger.Warn("terminal: reap detached terminal", "terminal_id", target.key.id, "error", err)
				continue
			}
		}
		m.removeWithTombstone(target.key, target.item, m.tombstoneExpiry(target.item))
	}
	m.sweepTombstones(now)
}

func (m *Service) removeWithTombstone(key terminalKey, expected *session, expiresAt time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.terminals[key] != expected {
		return
	}
	delete(m.terminals, key)
	m.tombstones[key] = tombstone{key: key, expiresAt: expiresAt}
	m.tombstoneOrder = append(m.tombstoneOrder, key)
	for len(m.tombstoneOrder) > maxTombstones {
		oldest := m.tombstoneOrder[0]
		m.tombstoneOrder = m.tombstoneOrder[1:]
		delete(m.tombstones, oldest)
	}
}

func (m *Service) sweepTombstones(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.tombstoneOrder[:0]
	for _, key := range m.tombstoneOrder {
		stone, ok := m.tombstones[key]
		if !ok {
			continue
		}
		if !now.Before(stone.expiresAt) {
			delete(m.tombstones, key)
			continue
		}
		kept = append(kept, key)
	}
	m.tombstoneOrder = kept
}
