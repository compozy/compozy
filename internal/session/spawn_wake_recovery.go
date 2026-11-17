package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func spawnWakeCanRetry(err error, shutdown bool) bool {
	return shutdown || errors.Is(err, ErrSpawnWakeRetryable)
}

func (m *Manager) spawnShutdownWakeState(info *Info, badge Badge) (shutdown, deferred bool) {
	shutdown = badge == BadgeStopped && info.StopReason == store.StopShutdown
	if !shutdown {
		return false, false
	}
	// The durable stop event owns this wake. Boot derives it after finalization;
	// querying history here would wait on the finalization invoking this callback.
	m.mu.RLock()
	_, deferred = m.finalizing[info.ID]
	m.mu.RUnlock()
	return shutdown, deferred
}

func (m *Manager) prepareSpawnWakeEvent(
	ctx context.Context, info *Info, badge Badge, reason SpawnWakeReason, shutdown bool,
) (SpawnWakeEvent, error) {
	event := SpawnWakeEvent{
		ChildSessionID: strings.TrimSpace(info.ID), ChildAgentName: strings.TrimSpace(info.AgentName),
		Reason: reason, Badge: badge, Detail: spawnWakeDetail(info, badge), WakeEventID: spawnWakeEventID(info, reason),
	}.Normalize()
	if shutdown {
		id, err := m.spawnShutdownWakeID(ctx, info.ID)
		if err != nil {
			return event, err
		}
		event.WakeEventID = id
		event.Detail = "Daemon shutdown stopped this child. Inspect its current status before intervening."
	}
	return event, event.Validate()
}

func (m *Manager) spawnShutdownWakeID(ctx context.Context, id string) (string, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{Type: EventTypeSessionStopped, Limit: 1,
		Archive: store.EventArchiveUnarchived})
	if err != nil || len(rows) == 0 {
		return "", err
	}
	event, err := transcript.UnmarshalAgentEvent(rows[0].Content)
	if err != nil || event.StopReason != string(store.StopShutdown) {
		return "", err
	}
	return fmt.Sprintf("session-wake:%s:shutdown:%s", id, rows[0].ID), nil
}

func (m *Manager) recoverSpawnShutdownWakes(ctx context.Context, infos []*Info) error {
	if m.spawnWakeNotifier == nil {
		return nil
	}
	for _, info := range infos {
		if info == nil || info.StopReason != store.StopShutdown || info.Lineage == nil ||
			!info.Lineage.NotifyCreator || info.Type == SessionTypeTriage {
			continue
		}
		wakeID, err := m.spawnShutdownWakeID(ctx, info.ID)
		if err != nil {
			return err
		}
		if wakeID == "" {
			continue
		}
		parentID := info.Lineage.ParentSessionID
		parent, err := m.Status(ctx, parentID)
		if err != nil || parent.State != StateActive {
			// A shutdown-stopped idle parent can reopen only through its captured binding.
			if err != nil || parent.StopReason != store.StopShutdown {
				continue
			}
			if err := m.ResumeWaitingSession(ctx, parentID); err != nil {
				m.logger.WarnContext(
					ctx,
					"session: shutdown wake parent unavailable",
					"session_id",
					parentID,
					"error",
					err,
				)
				continue
			}
		}
		stopped := *info
		stopped.State, stopped.StopReason = StateStopped, store.StopShutdown
		m.dispatchSpawnWake(ctx, &stopped, BadgeStopped)
	}
	return nil
}
