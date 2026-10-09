package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func (s *subagentService) Recover(ctx context.Context) error {
	stale, err := s.store.ListStaleReserved(ctx, s.now().Add(-2*time.Minute))
	if err != nil {
		return err
	}
	for _, row := range stale {
		child := ""
		if row.ChildSessionID != nil {
			child = *row.ChildSessionID
		}
		_, err := s.failDelegation(ctx, row, child, errors.New("delegation interrupted"))
		s.recoveryResult(ctx, row.ID, "stale_reservation", err)
	}
	if err := s.recoverRunning(ctx); err != nil {
		return err
	}
	native, err := s.store.ListUnfinalizedNative(ctx)
	if err != nil {
		return err
	}
	for _, row := range native {
		s.recoveryResult(ctx, row.ID, "native_interrupted", s.recoverNative(ctx, row))
	}
	if err := s.recoverWakes(ctx); err != nil {
		return err
	}
	if err := s.recoverPending(ctx); err != nil {
		return err
	}
	orphans, err := s.store.ListOrphanSubagentSessions(ctx)
	if err != nil {
		return err
	}
	for _, child := range orphans {
		s.recoveryResult(ctx, child, "orphan_stopped", s.runtime.Stop(ctx, child))
	}
	return nil
}

func (s *subagentService) recoverRunning(ctx context.Context) error {
	rows, err := s.store.ListUnfinalizedDelegated(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ChildSessionID == nil || row.Status == store.SubagentStatusQueued {
			continue
		}
		s.recoveryResult(ctx, row.ID, "child_reconciled", s.recoverChild(ctx, row))
	}
	return nil
}

func (s *subagentService) recoverChild(ctx context.Context, row store.SessionSubagent) error {
	snap, err := s.runtime.Snapshot(ctx, *row.ChildSessionID)
	if err != nil {
		return err
	}
	if snap.Info.State == StateStopped && snap.Info.StopReason == store.StopShutdown && row.PendingTask == nil {
		if err := s.runtime.ResumeChild(ctx, row); err != nil {
			s.recoveryResult(ctx, row.ID, "child_resume", err)
			unlock := s.lock(row.ParentSessionID)
			defer unlock()
			return s.finalizeAvailable(ctx, row, snap, BadgeIdle)
		}
		return nil
	}
	admitted, err := s.runtime.HasAdmission(ctx, row)
	if err != nil {
		return err
	}
	if !admitted {
		if row.PendingTask == nil {
			_, err := s.failDelegation(ctx, row, *row.ChildSessionID, errors.New("delegation interrupted"))
			return err
		}
		if err := s.runtime.Admit(ctx, row, subagentPrompt(row.Role, *row.PendingTask)); err != nil {
			_, settleErr := s.failDelegation(ctx, row, *row.ChildSessionID, err)
			return settleErr
		}
	}
	if row.PendingTask != nil {
		if err := s.store.MarkFirstPromptAdmitted(ctx, row.ID); err != nil {
			return err
		}
	}
	return s.OnChildSettled(ctx, *row.ChildSessionID)
}

func (s *subagentService) recoverNative(ctx context.Context, row store.SessionSubagent) error {
	unlock := s.lock(row.ParentSessionID)
	snap, err := s.runtime.Snapshot(ctx, row.ParentSessionID)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		unlock()
		return err
	}
	if snap.Active && snap.TurnID == row.ParentTurnID {
		unlock()
		return nil
	}
	err = s.finalizeInterruptedNative(ctx, row)
	if err == nil {
		err = s.settleParent(ctx, row.ParentSessionID)
	}
	unlock()
	if err != nil {
		return err
	}
	return s.OnChildSettled(ctx, row.ParentSessionID)
}

func (s *subagentService) recoverWakes(ctx context.Context) error {
	wakes, err := s.store.ListOpenWakes(ctx)
	if err != nil {
		return err
	}
	for _, wake := range wakes {
		s.recoveryResult(ctx, wake.WakeMessageID, "wake_reconciled", s.recoverWake(ctx, wake))
	}
	return nil
}

func (s *subagentService) recoverWake(ctx context.Context, wake store.SessionSubagentWake) error {
	snap, err := s.runtime.Snapshot(ctx, wake.ParentSessionID)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	if snap.Info != nil && snap.Info.State == StateStopping {
		return nil
	}
	if !subagentParentAcceptsWake(snap.Info) {
		return s.OnParentStopped(ctx, wake.ParentSessionID)
	}
	unlock := s.lock(wake.ParentSessionID)
	defer unlock()
	current, rows, err := s.store.GetWake(ctx, wake.WakeMessageID)
	if err != nil {
		return err
	}
	status, err := s.runtime.WakeInputStatus(ctx, current)
	if err != nil {
		return err
	}
	switch status {
	case store.SessionInputQueueStatusFailed:
		return s.failWake(ctx, wake.ParentSessionID, wake.WakeMessageID)
	case store.SessionInputQueueStatusCanceled:
		return s.settleWake(ctx, wake.ParentSessionID, wake.WakeMessageID, true)
	case store.SessionInputQueueStatusQueued, store.SessionInputQueueStatusDispatching:
		return nil
	case store.SessionInputQueueStatusSent:
		if snap.Active {
			return s.store.MarkWakeDispatched(ctx, wake.WakeMessageID)
		}
		return s.settleWake(ctx, wake.ParentSessionID, wake.WakeMessageID, false)
	}
	if current.State == store.SubagentWakeStateDispatched && current.Route == store.SubagentWakeRouteQueue {
		return s.failWake(ctx, wake.ParentSessionID, wake.WakeMessageID)
	}
	if current.State == store.SubagentWakeStateDispatched && current.Route == store.SubagentWakeRouteSteer {
		if err := s.store.MarkWakeSteerRequeued(ctx, current.WakeMessageID); err != nil {
			return err
		}
	}
	return s.deliver(ctx, current, rows, true)
}

func (s *subagentService) recoverPending(ctx context.Context) error {
	pending, err := s.store.ListPending(ctx)
	if err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, row := range pending {
		if seen[row.ParentSessionID] {
			continue
		}
		seen[row.ParentSessionID] = true
		snap, err := s.runtime.Snapshot(ctx, row.ParentSessionID)
		if err != nil && !errors.Is(err, ErrSessionNotFound) {
			s.recoveryResult(ctx, row.ID, "pending_snapshot", err)
			continue
		}
		if snap.Active {
			continue
		}
		s.recoveryResult(ctx, row.ID, "pending_claimed", s.OnParentTurnSettled(ctx, row.ParentSessionID, ""))
	}
	return nil
}

func (s *subagentService) recoveryResult(ctx context.Context, id, reason string, err error) {
	if err != nil {
		s.logger.ErrorContext(ctx, "subagent.recovery_failed", "id", id, "reason", reason, "error", err)
		return
	}
	s.logger.InfoContext(ctx, "subagent.recovered", "id", id, "reason", reason)
}
