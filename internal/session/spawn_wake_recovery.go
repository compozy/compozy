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
	// Shutdown recovery checks SyntheticWakeAdmitted durably before redispatching any failed delivery.
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
			!info.Lineage.NotifyCreator || info.Type == SessionTypeTriage || info.ArchivedAt != nil ||
			(info.Lineage.TTLExpiresAt != nil && !info.Lineage.TTLExpiresAt.After(m.now())) {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := m.recoverSpawnShutdownWake(ctx, info); err != nil && ctx.Err() == nil {
			m.logger.WarnContext(ctx, "session: shutdown wake recovery unavailable", "session_id", info.ID,
				"parent_session_id", info.Lineage.ParentSessionID, "error", err)
		}
	}
	return ctx.Err()
}

func (m *Manager) recoverSpawnShutdownWake(ctx context.Context, info *Info) error {
	wakeID, err := m.spawnShutdownWakeID(ctx, info.ID)
	if err != nil || wakeID == "" {
		return err
	}
	parentID := info.Lineage.ParentSessionID
	admitted, err := m.SyntheticWakeAdmitted(ctx, parentID, wakeID)
	if err != nil {
		return fmt.Errorf("read shutdown wake admission: %w", err)
	}
	if admitted {
		return nil
	}
	parent, err := m.Status(ctx, parentID)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read shutdown wake parent: %w", err)
	}
	if parent.State != StateActive {
		if parent.State != StateStopped || parent.StopReason != store.StopShutdown {
			return nil
		}
		if err := m.resumeRestartWaitingSession(ctx, parentID); err != nil {
			return fmt.Errorf("resume shutdown wake parent: %w", err)
		}
	}
	stopped := *info
	stopped.State, stopped.StopReason = StateStopped, store.StopShutdown
	m.dispatchSpawnWake(ctx, &stopped, BadgeStopped)
	return nil
}
