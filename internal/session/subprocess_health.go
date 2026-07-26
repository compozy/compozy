package session

import (
	"context"
	"sort"

	"github.com/compozy/agh/internal/subprocess"
)

// SubprocessHealthSnapshot is one immutable health observation for an active session process.
type SubprocessHealthSnapshot struct {
	SessionID   string
	WorkspaceID string
	AgentName   string
	Health      subprocess.HealthState
}

// SubprocessHealthNotifier is an optional notifier extension for observed failed health verdicts.
type SubprocessHealthNotifier interface {
	OnSubprocessHealth(context.Context, SubprocessHealthSnapshot)
}

// SubprocessHealthSnapshots returns immutable snapshots for active processes that expose health state.
func (m *Manager) SubprocessHealthSnapshots() []SubprocessHealthSnapshot {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	m.mu.RUnlock()

	snapshots := make([]SubprocessHealthSnapshot, 0, len(sessions))
	for _, sess := range sessions {
		proc := sess.processHandle()
		if proc == nil {
			continue
		}
		health, ok := proc.HealthState()
		if !ok {
			continue
		}
		snapshots = append(snapshots, subprocessHealthSnapshot(sess, health))
	}
	sort.Slice(snapshots, func(i int, j int) bool {
		return snapshots[i].SessionID < snapshots[j].SessionID
	})
	return snapshots
}

func (m *Manager) notifySubprocessHealth(
	ctx context.Context,
	sess *Session,
	health subprocess.HealthState,
) {
	if m == nil || m.notifier == nil || sess == nil {
		return
	}
	notifier, ok := m.notifier.(SubprocessHealthNotifier)
	if !ok {
		return
	}
	notifier.OnSubprocessHealth(ctx, subprocessHealthSnapshot(sess, health))
}

func subprocessHealthSnapshot(sess *Session, health subprocess.HealthState) SubprocessHealthSnapshot {
	snapshot := SubprocessHealthSnapshot{Health: subprocess.CloneHealthState(health)}
	if sess == nil {
		return snapshot
	}
	info := sess.Info()
	if info == nil {
		return snapshot
	}
	snapshot.SessionID = info.ID
	snapshot.WorkspaceID = info.WorkspaceID
	snapshot.AgentName = info.AgentName
	return snapshot
}
