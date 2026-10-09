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
		if _, err := s.caller(ctx, *actor.Caller); err != nil {
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
	err = s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: row.ParentSessionID, IDs: []string{id}})
	if err == nil && row.ChildSessionID == nil {
		settled, changed, settleErr := s.store.FinalizeSubagent(
			ctx,
			store.SubagentFinalize{
				ID:        id,
				Status:    store.SubagentStatusCanceled,
				WorkState: store.SubagentWorkStateResultAvailable,
				SettledAt: s.now().UTC(),
			},
		)
		err = settleErr
		if err == nil && changed {
			s.publishTerminal(ctx, settled)
		}
	}
	unlock()
	if err != nil {
		return SubagentCancelOutcome{}, err
	}
	if row.ChildSessionID != nil {
		s.logError(ctx, "cancel_descendants", s.OnParentStopped(ctx, *row.ChildSessionID))
		s.logError(ctx, "cancel_child", s.runtime.Stop(ctx, *row.ChildSessionID))
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
	for _, wake := range wakes {
		err = errors.Join(err, s.runtime.CancelWake(ctx, wake))
		_, settleErr := s.store.SettleWake(ctx, wake.WakeMessageID, true)
		err = errors.Join(err, settleErr)
	}
	unlock()
	for _, row := range rows {
		if row.Origin != store.SubagentOriginDelegated || store.IsSubagentStatusTerminal(row.Status) ||
			row.ChildSessionID == nil {
			continue
		}
		descendantErr := s.OnParentStopped(ctx, *row.ChildSessionID)
		s.logError(ctx, "stop_descendants", descendantErr)
		stopErr := s.runtime.Stop(ctx, *row.ChildSessionID)
		s.logError(ctx, "stop_child", stopErr)
		err = errors.Join(err, descendantErr, stopErr)
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
	if err := s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: parent, ParentTurnID: turn}); err != nil {
		return err
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
			if _, err := s.store.SettleWake(ctx, current.WakeMessageID, true); err != nil {
				return err
			}
		} else if err := s.rewriteWake(ctx, current, rows); err != nil {
			return err
		}
	}
	return nil
}
