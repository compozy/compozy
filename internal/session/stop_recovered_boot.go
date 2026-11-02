package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// RecoverPendingStops settles verified exits and terminates recovered orphans before boot admits work.
func (m *Manager) RecoverPendingStops(ctx context.Context) error {
	if err := m.ReconcileInactiveSessions(ctx); err != nil {
		return err
	}
	infos, err := m.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if !m.hasPendingStopSettlement(info.ID) && info.State != StateStopping {
			continue
		}
		if _, active := m.Get(info.ID); active {
			continue
		}
		cause := CauseProcessExited
		if info.StopReason == store.StopShutdown {
			cause = CauseShutdown
		}
		if info.StopReason == store.StopUserCanceled {
			cause = CauseUserRequested
		}
		if info.StopReason == store.StopError && info.StopDetail == resumeStopDetailStartIncomplete {
			cause = CauseFailed
		}
		if err := m.StopWithCause(ctx, info.ID, cause, info.StopDetail); err != nil {
			if m.hasPendingStopSettlement(info.ID) || ctx.Err() != nil || errors.Is(err, ErrRecoveryPersistence) {
				return fmt.Errorf("session: recover pending stop for %s: %w", info.ID, err)
			}
			outcome, stopErr := m.AwaitStopped(ctx, info.ID)
			if !outcome.Verified && !errors.Is(err, ErrStopVerificationFailed) {
				return errors.Join(err, stopErr)
			}
			m.logger.Warn("session: recovered stop incomplete", "session_id", info.ID, "error", err)
		}
	}
	return nil
}

// RecoverStaleInputDispatches settles busy-input leases a previous daemon
// process left in dispatching so they stop holding queue capacity. Boot calls
// it before any session can claim new input.
func (m *Manager) RecoverStaleInputDispatches(ctx context.Context) (int, error) {
	if m == nil || m.inputQueue == nil {
		return 0, nil
	}
	failed, err := m.inputQueue.FailStaleDispatches(ctx)
	if failed > 0 {
		m.logger.Warn("session: failed stale dispatching inputs after restart", "count", failed)
	}
	if err != nil {
		return failed, fmt.Errorf("session: recover stale input dispatches: %w", err)
	}
	return failed, m.publishIndeterminateReportNotes(ctx)
}
