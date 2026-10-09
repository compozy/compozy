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
		if _, err := s.failDelegation(ctx, row, child, errors.New("delegation interrupted")); err != nil {
			return err
		}
		s.recovered(ctx, row.ID, "stale_reservation")
	}
	if err := s.recoverRunning(ctx); err != nil {
		return err
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
		if err := s.runtime.Stop(ctx, child); err != nil {
			return err
		}
		s.recovered(ctx, child, "orphan_stopped")
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
		admitted, err := s.runtime.HasAdmission(ctx, row)
		if err != nil {
			return err
		}
		if !admitted {
			if row.PendingTask == nil {
				if _, err := s.failDelegation(
					ctx,
					row,
					*row.ChildSessionID,
					errors.New("delegation interrupted"),
				); err != nil {
					return err
				}
				s.recovered(ctx, row.ID, "missing_pending_task")
				continue
			}
			if err := s.runtime.Admit(ctx, row, subagentPrompt(row.Role, *row.PendingTask)); err != nil {
				if _, settleErr := s.failDelegation(ctx, row, *row.ChildSessionID, err); settleErr != nil {
					return settleErr
				}
				continue
			}
			s.recovered(ctx, row.ID, "first_prompt_readmitted")
		}
		if row.PendingTask != nil {
			if err := s.store.MarkFirstPromptAdmitted(ctx, row.ID); err != nil {
				return err
			}
		}
		if err := s.OnChildSettled(ctx, *row.ChildSessionID); err != nil {
			return err
		}
		s.recovered(ctx, row.ID, "child_reconciled")
	}
	return nil
}
func (s *subagentService) recoverWakes(ctx context.Context) error {
	wakes, err := s.store.ListOpenWakes(ctx)
	if err != nil {
		return err
	}
	for _, wake := range wakes {
		snap, err := s.runtime.Snapshot(ctx, wake.ParentSessionID)
		if err != nil && !errors.Is(err, ErrSessionNotFound) {
			return err
		}
		if snap.Info != nil && snap.Info.State == StateStopping {
			continue
		}
		if !subagentParentAcceptsWake(snap.Info) {
			if err := s.OnParentStopped(ctx, wake.ParentSessionID); err != nil {
				return err
			}
			s.recovered(ctx, wake.WakeMessageID, "stopped_parent_disposed")
			continue
		}
		unlock := s.lock(wake.ParentSessionID)
		current, rows, readErr := s.store.GetWake(ctx, wake.WakeMessageID)
		if readErr == nil {
			readErr = s.deliver(ctx, current, rows, true)
		}
		unlock()
		if readErr != nil {
			return readErr
		}
		s.recovered(ctx, wake.WakeMessageID, "wake_reoffered")
	}
	return nil
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
			return err
		}
		if snap.Active {
			continue
		}
		if err := s.OnParentTurnSettled(ctx, row.ParentSessionID, ""); err != nil {
			return err
		}
		s.recovered(ctx, row.ID, "pending_claimed")
	}
	return nil
}

func (s *subagentService) recovered(ctx context.Context, id, reason string) {
	s.logger.InfoContext(ctx, "subagent.recovered", "id", id, "reason", reason)
}
