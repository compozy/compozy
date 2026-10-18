package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func (s *subagentService) Status(ctx context.Context, caller SubagentCaller, id string) (Subagent, error) {
	if _, err := s.caller(ctx, caller); err != nil {
		return Subagent{}, err
	}
	unlock := s.lock(caller.SessionID)
	defer unlock()
	result, err := s.Get(ctx, caller.WorkspaceID, id)
	if err != nil {
		return Subagent{}, err
	}
	if result.ParentSessionID != caller.SessionID {
		return Subagent{}, ErrSubagentNotFound
	}
	if !store.IsSubagentStatusTerminal(result.Status) || result.Delivery == store.SubagentDeliveryAcknowledged {
		return result, nil
	}
	row, wake, err := s.store.Acknowledge(ctx, id, caller.TurnID)
	if err != nil {
		return Subagent{}, err
	}
	s.publish(ctx, row)
	if wake != nil {
		current, rows, err := s.store.GetWake(ctx, wake.WakeMessageID)
		if err != nil {
			return Subagent{}, err
		}
		if len(rows) == 0 {
			if err := s.runtime.CancelWake(ctx, *wake); err != nil {
				return Subagent{}, err
			}
			if _, err := s.settleWakeRows(ctx, wake.WakeMessageID, true); err != nil {
				return Subagent{}, err
			}
		} else if current.State == store.SubagentWakeStateOpen {
			if err := s.rewriteWake(ctx, current, rows); err != nil {
				return Subagent{}, err
			}
		}
	}
	return presentSubagent(row), nil
}
func (s *subagentService) OnWakeDispatched(ctx context.Context, parent, id string) error {
	unlock := s.lockWakeDispatch(ctx, parent)
	defer unlock()
	wake, rows, err := s.store.GetWake(ctx, id)
	if err != nil {
		return err
	}
	if wake.ParentSessionID != parent {
		return ErrSubagentNotFound
	}
	if err := s.store.MarkWakeDispatched(ctx, id); err != nil {
		return err
	}
	for _, row := range rows {
		s.publish(ctx, row)
	}
	return nil
}
func (s *subagentService) OnWakeTurnSettled(ctx context.Context, parent, id string, canceled bool) error {
	unlock := s.lock(parent)
	err := s.settleWake(ctx, parent, id, canceled)
	unlock()
	if err != nil {
		return err
	}
	return s.OnChildSettled(ctx, parent)
}
func (s *subagentService) settleWake(ctx context.Context, parent, id string, canceled bool) error {
	wake, _, err := s.store.GetWake(ctx, id)
	if err != nil {
		return err
	}
	if wake.ParentSessionID != parent {
		return ErrSubagentNotFound
	}
	if wake.State == store.SubagentWakeStateSettled || wake.State == store.SubagentWakeStateCanceled {
		s.forgetSteer(id)
		return nil
	}
	rows, err := s.settleWakeRows(ctx, id, canceled)
	if err != nil {
		return err
	}
	for _, row := range rows {
		s.publish(ctx, row)
	}
	return s.successor(ctx, parent)
}
func (s *subagentService) OnWakeCanceled(ctx context.Context, parent, id string) error {
	unlock := s.lockWakeDispatch(ctx, parent)
	defer unlock()
	wake, _, err := s.store.GetWake(ctx, id)
	if err != nil {
		return err
	}
	if wake.ParentSessionID != parent {
		return ErrSubagentNotFound
	}
	rows, err := s.settleWakeRows(ctx, id, true)
	if err != nil {
		return err
	}
	for _, row := range rows {
		s.publish(ctx, row)
	}
	return nil
}
func (s *subagentService) OnSteerOutcome(ctx context.Context, parent, id string, injected bool) error {
	unlock := s.lock(parent)
	defer unlock()
	wake, _, err := s.store.GetWake(ctx, id)
	if err != nil {
		return err
	}
	if wake.ParentSessionID != parent {
		return ErrSubagentNotFound
	}
	return s.steerOutcome(ctx, id, injected)
}
func (s *subagentService) steerOutcome(ctx context.Context, id string, injected bool) error {
	wake, rows, err := s.store.GetWake(ctx, id)
	if err != nil {
		return err
	}
	if wake.State == store.SubagentWakeStateSettled || wake.State == store.SubagentWakeStateCanceled {
		s.forgetSteer(id)
		return nil
	}
	s.forgetSteer(id)
	if injected {
		return s.settleWake(ctx, wake.ParentSessionID, id, false)
	}
	if wake.SteerRequeued {
		return nil
	}
	if err := s.store.MarkWakeSteerRequeued(ctx, id); err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "subagent.wake_route_fallback", "reason", "steer_not_injected", "wake_message_id", id)
	return s.deliver(ctx, wake, rows, true)
}
func (s *subagentService) OnParentTurnSettled(ctx context.Context, parent, turn string) error {
	unlock := s.lock(parent)
	err := s.interruptNative(ctx, parent, turn)
	if err == nil {
		err = s.settleTurnSteers(ctx, parent, turn)
	}
	if err == nil {
		err = s.successor(ctx, parent)
	}
	unlock()
	if err != nil {
		return err
	}
	return s.OnChildSettled(ctx, parent)
}

func (s *subagentService) settleTurnSteers(ctx context.Context, parent, turn string) error {
	wakes, err := s.parentWakes(ctx, parent)
	if err != nil {
		return err
	}
	for _, wake := range wakes {
		s.mu.Lock()
		target, accepted := s.steerTurns[wake.WakeMessageID]
		s.mu.Unlock()
		if accepted && target == turn {
			if err := s.settleWake(ctx, parent, wake.WakeMessageID, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *subagentService) OnWakeFailed(ctx context.Context, parent, id string) error {
	unlock := s.lockWakeDispatch(ctx, parent)
	defer unlock()
	wake, _, err := s.store.GetWake(ctx, id)
	if err != nil {
		return err
	}
	if wake.ParentSessionID != parent {
		return ErrSubagentNotFound
	}
	return s.failWake(ctx, parent, id)
}

func (s *subagentService) failWake(ctx context.Context, parent, id string) error {
	wake, rows, err := s.store.FailWake(ctx, id)
	if err != nil {
		return err
	}
	s.forgetSteer(id)
	for _, row := range rows {
		s.publish(ctx, row)
	}
	if len(rows) > 0 && wake.Attempts >= 3 {
		s.logger.WarnContext(
			ctx,
			"subagent.wake_abandoned",
			"parent_session_id",
			parent,
			"wake_message_id",
			id,
			"attempts",
			wake.Attempts,
		)
	}
	if len(rows) > 0 && wake.Attempts < 3 {
		s.retryWake(parent, wake.Attempts)
	}
	return nil
}

const subagentWakeRetryDelay = 200 * time.Millisecond

func (s *subagentService) retryWake(parent string, attempts int) {
	s.mu.Lock()
	if s.wakeRetries == nil {
		s.wakeRetries = make(map[string]bool)
	}
	if s.wakeRetries[parent] {
		s.mu.Unlock()
		return
	}
	s.wakeRetries[parent] = true
	s.mu.Unlock()
	s.launch(func() {
		timer := time.NewTimer(subagentWakeRetryDelay * time.Duration(attempts))
		defer timer.Stop()
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
		}
		unlock := s.lock(parent)
		defer unlock()
		s.mu.Lock()
		delete(s.wakeRetries, parent)
		s.mu.Unlock()
		s.logError(s.ctx, "wake_retry", s.successor(s.ctx, parent))
	})
}

func (s *subagentService) forgetSteer(id string) {
	s.mu.Lock()
	delete(s.steerTurns, id)
	s.mu.Unlock()
}

func (s *subagentService) settleWakeRows(
	ctx context.Context,
	id string,
	canceled bool,
) ([]store.SessionSubagent, error) {
	rows, err := s.store.SettleWake(ctx, id, canceled)
	if err == nil {
		s.forgetSteer(id)
	}
	return rows, err
}

func (s *subagentService) successor(ctx context.Context, parent string) error {
	s.mu.Lock()
	retrying := s.wakeRetries[parent]
	s.mu.Unlock()
	if retrying {
		return nil
	}
	pending, err := s.store.ListPending(ctx, parent)
	if err != nil {
		return err
	}
	var ids []string
	for _, row := range pending {
		if row.ParentSessionID == parent {
			ids = append(ids, row.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	snap, err := s.runtime.Snapshot(ctx, parent)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	if snap.Info != nil && snap.Info.State == StateStopping {
		return nil
	}
	if !subagentParentAcceptsWake(snap.Info) {
		return s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: parent, IDs: ids})
	}
	wakes, err := s.parentWakes(ctx, parent)
	if err != nil {
		return err
	}
	for _, wake := range wakes {
		if wake.State == store.SubagentWakeStateDispatched {
			return nil
		}
	}
	return s.claim(ctx, parent, ids)
}
func (s *subagentService) UpgradeWakePolicy(ctx context.Context, id string) error {
	row, err := s.internalRow(ctx, id)
	if err != nil {
		return err
	}
	unlock := s.lock(row.ParentSessionID)
	defer unlock()
	row, err = s.store.UpgradeWakePolicy(ctx, id)
	if err != nil {
		return err
	}
	s.publish(ctx, row)
	if store.IsSubagentStatusTerminal(row.Status) {
		return s.planDelivery(ctx, row)
	}
	return nil
}
func (s *subagentService) parentRows(ctx context.Context, parent string) ([]store.SessionSubagent, error) {
	q := store.SubagentListQuery{ParentSessionID: parent, Limit: 100}
	var rows []store.SessionSubagent
	for {
		page, err := s.store.ListSubagents(ctx, q)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page.Items...)
		if page.NextCursor == "" {
			return rows, nil
		}
		q.Cursor = page.NextCursor
	}
}
func (s *subagentService) parentWakes(ctx context.Context, parent string) ([]store.SessionSubagentWake, error) {
	return s.store.ListWakesByParent(
		ctx,
		parent,
		[]string{store.SubagentWakeStateOpen, store.SubagentWakeStateDispatched},
	)
}

func (s *subagentService) internalRow(ctx context.Context, id string) (store.SessionSubagent, error) {
	return s.store.GetSubagentByID(ctx, id)
}
