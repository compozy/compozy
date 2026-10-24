package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// PrepareDaemonShutdown records intent before brokers or subprocesses can settle an interrupted turn.
func (m *Manager) PrepareDaemonShutdown(ctx context.Context, restart bool) error {
	var errs []error
	infos, err := m.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, info := range infos {
		active, ok := m.Get(info.ID)
		if !ok || !active.IsPrompting() {
			if !restart && info.StopReason == store.StopShutdown {
				errs = append(errs, m.cancelPendingRestart(ctx, info.ID))
			}
			continue
		}
		meta := active.Meta()
		reason := ""
		if !restart {
			reason = "daemon_stopped_by_user"
		}
		if err := m.captureRestartInterruption(ctx, &meta, reason); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) cancelPendingRestart(ctx context.Context, id string) error {
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return err
	}
	checkpoint, pending, err := m.pendingRestartCheckpoint(ctx, &meta)
	if err != nil || !pending {
		return err
	}
	checkpoint.Reason = "daemon_stopped_by_user"
	return m.appendRestartEvent(ctx, &meta, restartSkippedEvent, checkpoint, "Stopped by user.")
}

// CaptureRestartInterruptions runs before pending-interaction expiration and history repair.
func (m *Manager) CaptureRestartInterruptions(ctx context.Context, infos []*Info) error {
	for _, info := range infos {
		if info.StopReason != store.StopShutdown {
			continue
		}
		meta, err := m.readMetaWithContext(ctx, info.ID)
		if err != nil {
			return err
		}
		if err := m.captureRestartInterruption(ctx, &meta, ""); err != nil {
			return fmt.Errorf("session: capture restart interruption for %s: %w", info.ID, err)
		}
	}
	return nil
}

func (m *Manager) captureRestartInterruption(ctx context.Context, meta *store.SessionMeta, reason string) error {
	checkpoint, found, err := m.latestRestartBoundary(ctx, meta)
	if err != nil || !found {
		return err
	}
	interrupted, err := m.restartEventExists(ctx, meta.ID, checkpoint.TurnID, restartInterruptedEvent)
	if err != nil {
		return err
	}
	if interrupted {
		if reason != "" {
			checkpoint.Reason = reason
			return m.appendRestartEvent(ctx, meta, restartSkippedEvent, checkpoint, "Stopped by user.")
		}
		return nil
	}
	terminal, err := m.restartTurnTerminal(ctx, meta.ID, checkpoint.TurnID)
	if err != nil || terminal {
		return err
	}
	checkpoint.Reason = reason
	if reason == "" && checkpoint.Delivery != PromptDeliveryNormal {
		checkpoint.Reason = "maintenance_command"
	}
	if m.attentionStore != nil && checkpoint.Reason == "" {
		interactions, err := m.PendingInteractions(ctx, meta.ID, []string{
			store.PendingInteractionStatusPending, store.PendingInteractionStatusOrphaned,
		})
		if err != nil {
			return err
		}
		if len(interactions) > 0 {
			checkpoint.Reason = "pending_interaction"
		}
	}
	return m.appendRestartEvent(ctx, meta, restartInterruptedEvent, checkpoint, "Interrupted by CompozyOS restart.")
}
