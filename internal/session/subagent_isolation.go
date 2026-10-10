package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func (s *subagentService) provisionIsolation(
	ctx context.Context,
	req SubagentRequest,
	parent *Info,
	row *store.SessionSubagent,
) error {
	if s.worktrees == nil {
		return &ErrSubagentIsolationFailed{Cause: "unavailable"}
	}
	wt, err := s.worktrees.Provision(
		ctx,
		SubagentWorktreeRequest{
			WorkspaceID:      row.WorkspaceID,
			ProfileID:        parent.ProfileID,
			SubagentID:       row.ID,
			Title:            row.Title,
			BaseRef:          req.BaseRef,
			CallerPath:       parent.Workspace,
			CallerWorktreeID: parent.WorktreeID,
		},
	)
	if wt.ID != "" {
		associateSubagentWorktree(row, wt)
		persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
		defer cancel()
		if associateErr := s.store.AssociateSubagentWorktree(persist, *row); associateErr != nil {
			return errors.Join(err, associateErr)
		}
	}
	if err == nil {
		s.logger.InfoContext(
			ctx,
			"subagent.isolation.created",
			"subagent_id",
			row.ID,
			"worktree_id",
			wt.ID,
			"workspace_id",
			row.WorkspaceID,
		)
	}
	return err
}

// compensateIsolation retains any admitted work and fails closed on uncertain evidence.
// The run_id lookup closes the crash window before association; the shared child
// identity closes the window between spawning a process and linking its row.
func (s *subagentService) compensateIsolation(
	ctx context.Context,
	row *store.SessionSubagent,
	child string,
) (retained bool, err error) {
	if s.worktrees == nil {
		return false, errors.New("session: worktree service unavailable")
	}
	// Keep the reservation itself retryable even when ownership lookup is temporarily unreadable.
	if err := s.store.SetSubagentWorktreeCleanup(ctx, row.ID, "pending"); err != nil {
		return false, err
	}
	if err := s.findIsolationWorktree(ctx, row); err != nil {
		return false, err
	}
	admitted, err := s.stopUnadmittedIsolationChild(ctx, row, child)
	if err != nil {
		return false, err
	}
	if admitted {
		return true, s.store.SetSubagentWorktreeCleanup(ctx, row.ID, "done")
	}
	return s.rollbackIsolatedWorktree(ctx, *row)
}
func (s *subagentService) findIsolationWorktree(ctx context.Context, row *store.SessionSubagent) error {
	if row.WorktreeState().ID != "" {
		return nil
	}
	wt, err := s.worktrees.FindByRun(ctx, row.WorkspaceID, row.ID)
	if err != nil {
		return err
	}
	if wt == nil {
		return nil
	}
	associateSubagentWorktree(row, *wt)
	return s.store.AssociateSubagentWorktree(ctx, *row)
}
func (s *subagentService) stopUnadmittedIsolationChild(
	ctx context.Context,
	row *store.SessionSubagent,
	child string,
) (bool, error) {
	if row.ChildSessionID != nil {
		child = *row.ChildSessionID
	}
	if child == "" {
		candidate := subagentChildSessionID(row.ParentSessionID, row.ID)
		snap, err := s.runtime.Snapshot(ctx, candidate)
		if err != nil && !errors.Is(err, ErrSessionNotFound) {
			return false, err
		}
		if err == nil && snap.Info != nil {
			child = candidate
		}
	}
	if child != "" {
		if row.ChildSessionID == nil && row.Status == store.SubagentStatusQueued {
			linked, err := s.store.LinkChild(ctx, row.ID, child, s.now().UTC())
			if err != nil {
				return false, err
			}
			*row = linked
		}
		row.ChildSessionID = new(child)
		admitted, err := s.hasIsolatedAdmission(ctx, *row)
		if err != nil {
			return false, err
		}
		if admitted {
			return true, nil
		}
		if err := s.runtime.Stop(
			ctx,
			child,
		); err != nil && !errors.Is(err, ErrSessionNotActive) &&
			!errors.Is(err, ErrSessionNotFound) {
			return false, err
		}
		// Stop joins dispatch; evidence may have committed between the first read and stop.
		admitted, err = s.hasIsolatedAdmission(ctx, *row)
		if err != nil {
			return false, err
		}
		if admitted {
			return true, nil
		}
	}

	return false, nil
}

func (s *subagentService) rollbackIsolatedWorktree(
	ctx context.Context,
	row store.SessionSubagent,
) (retained bool, err error) {
	if row.WorktreeState().ID == "" {
		return false, s.store.SetSubagentWorktreeCleanup(ctx, row.ID, "done")
	}
	retained, err = s.worktrees.SafeRollback(
		ctx,
		row.WorkspaceID,
		row.WorktreeState().ID,
		row.ID,
		row.WorktreeState().BaseSHA,
	)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"subagent.isolation.rollback_failed",
			"subagent_id",
			row.ID,
			"worktree_id",
			row.WorktreeState().ID,
			"error",
			err,
		)
		return false, err
	}
	if retained {
		s.logger.InfoContext(
			ctx,
			"subagent.isolation.rollback_skipped",
			"subagent_id",
			row.ID,
			"worktree_id",
			row.WorktreeState().ID,
			"reason",
			"worktree_retained",
		)
	}
	if err := s.store.SetSubagentWorktreeCleanup(ctx, row.ID, "done"); err != nil {
		return retained, err
	}
	return retained, nil
}
func (s *subagentService) hasIsolatedAdmission(ctx context.Context, row store.SessionSubagent) (bool, error) {
	if row.ChildSessionID == nil {
		return false, nil
	}
	committed, err := s.store.HasSubagentCommittedAdmission(ctx, row.WorkspaceID, *row.ChildSessionID, row.ID)
	if err != nil || committed {
		return committed, err
	}
	return s.runtime.HasAdmission(ctx, row)
}

func (s *subagentService) failIsolatedDelegation(
	ctx context.Context,
	row store.SessionSubagent,
	child string,
	cause error,
) (Subagent, error) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
	defer cancel()
	retained, cleanupErr := s.compensateIsolation(cleanup, &row, child)

	if row.WorktreeState().ID == "" && row.ChildSessionID == nil && cleanupErr == nil {
		cleanupErr = s.store.DeleteReserved(cleanup, row.ID)
	} else {
		summary := cause.Error()
		if retained {
			summary = "worktree_retained: " + summary
		}
		unlock := s.lock(row.ParentSessionID)
		settled, changed, err := s.store.FinalizeSubagent(
			cleanup,
			store.SubagentFinalize{
				ID:        row.ID,
				Status:    store.SubagentStatusFailed,
				WorkState: store.SubagentWorkStateResultAvailable,
				Error:     new(summary),
				SettledAt: s.now().UTC(),
			},
		)
		if err == nil && changed {
			s.publishTerminal(cleanup, settled)
		}
		unlock()
		cleanupErr = errors.Join(cleanupErr, err)
	}
	// Failed calls stop their child even when admitted work requires retaining its checkout.
	if retained && row.ChildSessionID != nil {
		stopErr := s.runtime.Stop(cleanup, *row.ChildSessionID)
		if !errors.Is(stopErr, ErrSessionNotActive) && !errors.Is(stopErr, ErrSessionNotFound) {
			cleanupErr = errors.Join(cleanupErr, stopErr)
		}
	}
	public := cause
	if isolation, ok := errors.AsType[*ErrSubagentIsolationFailed](cause); ok {
		public = &SubagentError{Code: "isolation_failed", Message: isolation.Error(), Err: cause}
	}
	s.mu.Lock()
	if flight := s.flights[row.ID]; flight != nil {
		flight.err = public
	}
	s.mu.Unlock()
	s.logger.ErrorContext(
		cleanup,
		"subagent.isolation.failed",
		"subagent_id",
		row.ID,
		"workspace_id",
		row.WorkspaceID,
		"error",
		public,
	)
	return Subagent{}, errors.Join(public, cleanupErr)
}
func (s *subagentService) recoverIsolation(ctx context.Context, row store.SessionSubagent) error {
	retained, err := s.compensateIsolation(ctx, &row, "")
	if err != nil {
		return err
	}
	if retained && row.ChildSessionID != nil {
		admitted, err := s.hasIsolatedAdmission(ctx, row)
		if err != nil {
			return err
		}
		if admitted {
			if _, err := s.store.LinkChild(ctx, row.ID, *row.ChildSessionID, s.now().UTC()); err != nil {
				return err
			}
			return s.recoverChild(ctx, row)
		}
	}
	summary := "delegation interrupted"
	if retained {
		summary = "worktree_retained"
	}
	unlock := s.lock(row.ParentSessionID)
	defer unlock()
	failed, changed, err := s.store.FinalizeSubagent(
		ctx,
		store.SubagentFinalize{
			ID:        row.ID,
			Status:    store.SubagentStatusFailed,
			Error:     new(summary),
			SettledAt: s.now().UTC(),
		},
	)
	if err != nil {
		return err
	}
	if changed {
		s.publishTerminal(ctx, failed)
	}
	if failed.WakePolicy != store.SubagentWakePolicyAlways {
		failed, err = s.store.UpgradeWakePolicy(ctx, row.ID)
		if err != nil {
			return err
		}
	}
	return s.planDelivery(ctx, failed)
}
func (s *subagentService) recoverIsolationCleanup(ctx context.Context) error {
	rows, err := s.store.ListSubagentWorktreeCleanupPending(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if !store.IsSubagentStatusTerminal(row.Status) || s.inFlight(row.ID) {
			continue
		}
		if _, err := s.compensateIsolation(ctx, &row, ""); err != nil {
			s.recoveryResult(ctx, row.ID, "worktree_cleanup", fmt.Errorf("retry rollback: %w", err))
		}
	}
	return nil
}
