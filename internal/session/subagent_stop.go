package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

func (s *subagentService) Cancel(
	ctx context.Context,
	actor SubagentActor,
	id, _ string,
) (SubagentCancelOutcome, error) {
	workspace := ""
	if actor.Kind == string(PromptCancelAgent) {
		if actor.Caller == nil {
			return SubagentCancelOutcome{}, ErrSubagentNotFound
		}
		// Canceling owned work needs ownership, not a live turn: an agent may
		// cancel over HTTP/UDS between turns, where no turn id exists.
		if err := s.callerSession(ctx, *actor.Caller); err != nil {
			return SubagentCancelOutcome{}, err
		}
		workspace = actor.Caller.WorkspaceID
	} else if actor.Kind != "operator" {
		return SubagentCancelOutcome{}, ErrSubagentNotFound
	}
	var row store.SessionSubagent
	var err error
	if workspace == "" {
		row, err = s.internalRow(ctx, id)
	} else {
		row, err = s.store.GetSubagent(ctx, workspace, id)
	}
	if errors.Is(err, store.ErrSubagentNotFound) {
		return SubagentCancelOutcome{}, ErrSubagentNotFound
	}
	if err != nil {
		return SubagentCancelOutcome{}, err
	}
	if actor.Kind == string(PromptCancelAgent) && row.ParentSessionID != actor.Caller.SessionID {
		return SubagentCancelOutcome{}, ErrSubagentNotFound
	}
	if row.Origin == store.SubagentOriginProviderNative {
		return SubagentCancelOutcome{}, ErrSubagentNotCancelable
	}
	if store.IsSubagentStatusTerminal(row.Status) {
		return SubagentCancelOutcome{ID: id, Status: row.Status}, nil
	}
	unlock := s.lock(row.ParentSessionID)
	// Delegation links the child and settlement finalizes the row under this
	// lock, so the pre-lock read can be stale: act on the current row.
	row, err = s.store.GetSubagentByID(ctx, id)
	if err != nil {
		unlock()
		return SubagentCancelOutcome{}, err
	}
	if store.IsSubagentStatusTerminal(row.Status) {
		unlock()
		return SubagentCancelOutcome{ID: id, Status: row.Status}, nil
	}
	err = s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: row.ParentSessionID, IDs: []string{id}})
	if err == nil {
		err = s.finalizeCanceled(ctx, row)
	}

	unlock()
	if err != nil {
		return SubagentCancelOutcome{}, err
	}
	if row.ChildSessionID != nil {
		// Freeze every live descendant before any stop signal can turn an
		// in-flight delegation admission into a provider failure.
		cascadeErr := s.cancelDescendants(ctx, *row.ChildSessionID)
		s.launch(func() {
			s.logError(s.ctx, "cancel_child", s.runtime.Stop(s.ctx, *row.ChildSessionID))
		})
		if cascadeErr != nil {
			return SubagentCancelOutcome{}, cascadeErr
		}
	}
	return SubagentCancelOutcome{ID: id, Status: "cancel_requested"}, nil
}
func (s *subagentService) OnParentStopped(ctx context.Context, parent string) error {
	unlock := s.lock(parent)
	if err := s.interruptNative(ctx, parent, ""); err != nil {
		unlock()
		return err
	}
	rows, err := s.parentRows(ctx, parent)
	if err != nil {
		unlock()
		return err
	}
	wakes, err := s.parentWakes(ctx, parent)
	if err != nil {
		unlock()
		return err
	}
	err = s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: parent})
	if err == nil {
		for _, row := range rows {
			if row.Origin == store.SubagentOriginDelegated && !store.IsSubagentStatusTerminal(row.Status) {
				err = errors.Join(err, s.finalizeCanceled(ctx, row))
			}
		}
	}
	for _, wake := range wakes {
		err = errors.Join(err, s.runtime.CancelWake(ctx, wake))
		_, settleErr := s.settleWakeRows(ctx, wake.WakeMessageID, true)
		err = errors.Join(err, settleErr)
	}
	unlock()
	for _, row := range rows {
		if row.Origin != store.SubagentOriginDelegated || row.ChildSessionID == nil {
			continue
		}
		// The reaper owns auto_stop_on_parent for terminal non-canceled children.
		if store.IsSubagentStatusTerminal(row.Status) && row.Status != store.SubagentStatusCanceled {
			continue
		}
		if snap, err := s.runtime.Snapshot(
			ctx,
			*row.ChildSessionID,
		); err == nil && snap.Info != nil &&
			snap.Info.State == StateStopped {
			continue
		}
		// Manager.Stop starts this child's stop before cascading to its descendants.
		// A separate pre-pass would serialize their escalation grace periods.
		stopErr := s.runtime.Stop(ctx, *row.ChildSessionID)
		s.logError(ctx, "stop_child", stopErr)
		err = errors.Join(err, stopErr)
	}
	return err
}
func (s *subagentService) OnParentTurnInterrupted(ctx context.Context, parent, turn string) error {
	unlock := s.lock(parent)
	defer unlock()
	if err := s.interruptNative(ctx, parent, turn); err != nil {
		return err
	}
	wakes, err := s.parentWakes(ctx, parent)
	if err != nil {
		return err
	}
	protected, err := s.interruptSteerBatches(ctx, turn, wakes)
	if err != nil {
		return err
	}
	rows, err := s.parentRows(ctx, parent)
	if err != nil {
		return err
	}
	var disposeIDs []string
	for _, row := range rows {
		if row.ParentTurnID == turn && !protected[row.ID] {
			disposeIDs = append(disposeIDs, row.ID)
		}
	}
	if len(disposeIDs) > 0 {
		if err := s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: parent, IDs: disposeIDs}); err != nil {
			return err
		}
	}
	for _, wake := range wakes {
		current, rows, err := s.store.GetWake(ctx, wake.WakeMessageID)
		if err != nil {
			return err
		}
		if current.State != store.SubagentWakeStateOpen {
			continue
		}
		if len(rows) == 0 {
			if err := s.runtime.CancelWake(ctx, current); err != nil {
				return err
			}
			if _, err := s.settleWakeRows(ctx, current.WakeMessageID, true); err != nil {
				return err
			}
		} else if err := s.rewriteWake(ctx, current, rows); err != nil {
			return err
		}
	}
	return nil
}

func (s *subagentService) interruptSteerBatches(
	ctx context.Context,
	turn string,
	wakes []store.SessionSubagentWake,
) (map[string]bool, error) {
	// Accepted but unconfirmed steer batches must survive interruption, even if
	// their results were created in this turn. Other turn-owned work is disposed.
	protected := make(map[string]bool)
	for _, wake := range wakes {
		s.mu.Lock()
		target, tracked := s.steerTurns[wake.WakeMessageID]
		s.mu.Unlock()
		if !tracked || target != turn || wake.State != store.SubagentWakeStateDispatched {
			continue
		}
		rows, err := s.settleWakeRows(ctx, wake.WakeMessageID, true)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			protected[row.ID] = true
			s.publish(ctx, row)
		}
	}
	return protected, nil
}

func (s *subagentService) finalizeCanceled(ctx context.Context, row store.SessionSubagent) error {
	settled, changed, err := s.store.FinalizeSubagent(ctx, store.SubagentFinalize{
		ID: row.ID, Status: store.SubagentStatusCanceled,
		WorkState: store.SubagentWorkStateResultAvailable, SettledAt: s.now().UTC(),
	})
	if err == nil && changed {
		s.publishTerminal(ctx, settled)
		s.logError(ctx, "cancel_parent_settle", s.settleParent(ctx, row.ParentSessionID))
	}
	return err
}

func (s *subagentService) cancelDescendants(ctx context.Context, parent string) error {
	unlock := s.lock(parent)
	rows, err := s.parentRows(ctx, parent)
	if err == nil {
		err = s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: parent})
	}
	var children []string
	if err == nil {
		for _, row := range rows {
			if row.Origin != store.SubagentOriginDelegated || store.IsSubagentStatusTerminal(row.Status) {
				continue
			}
			if err = s.finalizeCanceled(ctx, row); err != nil {
				break
			}
			if row.ChildSessionID != nil {
				children = append(children, *row.ChildSessionID)
			}
		}
	}
	unlock()
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := s.cancelDescendants(ctx, child); err != nil {
			return err
		}
	}
	return nil
}
