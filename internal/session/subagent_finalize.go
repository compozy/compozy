package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

func deriveSubagentWorkState(active bool, queued, live int) string {
	if active || queued > 0 {
		return store.SubagentWorkStateWorking
	}
	if live > 0 {
		return store.SubagentWorkStateWaitingForChildren
	}
	return store.SubagentWorkStateResultAvailable
}

func (s *subagentService) OnChildSettled(ctx context.Context, child string) error {
	row, err := s.store.GetSubagentByChild(ctx, child)
	if errors.Is(err, store.ErrSubagentNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	unlock := s.lock(row.ParentSessionID)
	err = s.finalize(ctx, row)
	unlock()
	if err != nil {
		return err
	}
	return s.OnChildSettled(ctx, row.ParentSessionID)
}

func (s *subagentService) finalize(ctx context.Context, row store.SessionSubagent) error {
	current, err := s.store.GetSubagent(ctx, row.WorkspaceID, row.ID)
	if err != nil || store.IsSubagentStatusTerminal(current.Status) {
		return err
	}
	row = current
	child, err := s.runtime.Snapshot(ctx, *row.ChildSessionID)
	if err != nil {
		return err
	}
	if child.Info.State == StateStopping || child.Info.State == StateStarting ||
		(child.Info.State == StateStopped && child.Info.StopReason == store.StopShutdown) {
		return nil
	}
	if row.PendingTask != nil && child.Info.State != StateStopped {
		return nil
	}
	summary, err := s.store.Summaries(ctx, []string{*row.ChildSessionID})
	if err != nil {
		return err
	}
	work := deriveSubagentWorkState(child.Active, child.Queued, summary[*row.ChildSessionID].Live)
	status := store.SubagentStatusRunning
	badge := BadgeForInfo(child.Info)
	if badge == BadgeWaitingForAuth || badge == BadgeWaitingForInput || badge == BadgeNeedsAttention {
		status = store.SubagentStatusWaiting
	}
	if work != store.SubagentWorkStateResultAvailable && child.Info.State != StateStopped {
		updated, changed, err := s.store.UpdateSubagentState(ctx, row.ID, status, work, s.now().UTC())
		if err == nil && changed {
			s.publish(ctx, updated)
			if (row.Status == store.SubagentStatusWaiting) != (updated.Status == store.SubagentStatusWaiting) {
				s.runtime.PublishParent(ctx, row.ParentSessionID)
			}
		}
		return err
	}
	return s.finalizeAvailable(ctx, row, child, badge)
}

func (s *subagentService) finalizeAvailable(
	ctx context.Context,
	row store.SessionSubagent,
	child subagentSnapshot,
	badge Badge,
) error {
	status := store.SubagentStatusCompleted
	var failure *string
	if child.Info.Failure != nil {
		message := child.Info.Failure.Summary
		failure = &message
	}
	switch {
	case badge == BadgeFailed:
		status = store.SubagentStatusFailed
	case child.Info.State == StateStopped:
		if child.Info.StopReason == store.StopUserCanceled {
			status = store.SubagentStatusCanceled
		} else if child.Info.StopReason != store.StopCompleted {
			status = store.SubagentStatusInterrupted
		}
	}
	result, err := s.runtime.Result(ctx, *row.ChildSessionID)
	if err != nil {
		return err
	}
	runes := []rune(result)
	limit, err := s.resultLimit(ctx, row.WorkspaceID)
	if err != nil {
		return err
	}
	truncated := limit > 0 && len(runes) > limit
	if truncated {
		result = string(runes[:limit])
	}
	settled, changed, err := s.store.FinalizeSubagent(
		ctx,
		store.SubagentFinalize{
			ID:              row.ID,
			Status:          status,
			WorkState:       store.SubagentWorkStateResultAvailable,
			Result:          &result,
			Error:           failure,
			ResultTruncated: truncated,
			SettledAt:       s.now().UTC(),
		},
	)
	if err != nil || !changed {
		return err
	}
	// Terminal publication precedes delivery transitions and happens only for the winning finalizer.
	s.publishTerminal(ctx, settled)
	if err := s.planDelivery(ctx, settled); err != nil {
		return err
	}
	return s.settleParent(ctx, row.ParentSessionID)
}

func (s *subagentService) settleParent(ctx context.Context, parent string) error {
	summaries, err := s.store.Summaries(ctx, []string{parent})
	if err != nil || summaries[parent].Live > 0 {
		return err
	}
	snap, err := s.runtime.Snapshot(ctx, parent)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	if err != nil || snap.Active || snap.Info.State != StateActive {
		return err
	}
	wakes, err := s.parentWakes(ctx, parent)
	if err != nil {
		return err
	}
	for _, wake := range wakes {
		if wake.ParentSessionID == parent && wake.State == store.SubagentWakeStateDispatched {
			return nil
		}
	}
	return s.runtime.SettleParent(ctx, parent)
}
