package session

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

const subagentPlanNone = "none"
const subagentWakeKind = "subagent_wake"

func planSubagentDelivery(row store.SessionSubagent, parent subagentSnapshot, dispatched bool) string {
	if store.IsSubagentDeliveryFinal(row.Delivery) {
		return subagentPlanNone
	}
	if parent.Info != nil && parent.Info.State == StateStopping {
		return store.SubagentDeliveryPending
	}
	if !subagentParentAcceptsWake(parent.Info) {
		return "dispose"
	}
	if row.WakePolicy == store.SubagentWakePolicySettledOnly && parent.Active && parent.TurnID == row.ParentTurnID {
		return subagentPlanNone
	}
	if dispatched {
		return store.SubagentDeliveryPending
	}
	return "claim"
}

// A daemon interruption suspends delivery until the user resumes the parent.
// Explicit stops still dispose results through the ordinary stop cascade.
func subagentParentAcceptsWake(info *Info) bool {
	return info != nil && (info.State == StateActive ||
		(info.State == StateStopped && (info.StopReason == store.StopShutdown || info.StopReason == store.StopAgentCrashed)))
}

func (s *subagentService) planDelivery(ctx context.Context, row store.SessionSubagent) error {
	snap, err := s.runtime.Snapshot(ctx, row.ParentSessionID)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	wakes, err := s.parentWakes(ctx, row.ParentSessionID)
	if err != nil {
		return err
	}
	dispatched := false
	for _, wake := range wakes {
		if wake.ParentSessionID == row.ParentSessionID && wake.State == store.SubagentWakeStateDispatched {
			dispatched = true
		}
	}
	switch planSubagentDelivery(row, snap, dispatched) {
	case subagentPlanNone:
		return nil
	case "dispose":
		return s.dispose(ctx, store.SubagentDisposeFilter{ParentSessionID: row.ParentSessionID, IDs: []string{row.ID}})
	case store.SubagentDeliveryPending:
		if err := s.store.SetPending(ctx, []string{row.ID}); err != nil {
			return err
		}
		return s.publishID(ctx, row.WorkspaceID, row.ID)
	default:
		return s.claim(ctx, row.ParentSessionID, []string{row.ID})
	}
}

func (s *subagentService) claim(ctx context.Context, parent string, ids []string) error {
	id, err := s.newID()
	if err != nil {
		return err
	}
	wake, err := s.store.OpenOrJoinWake(ctx, parent, ids, id)
	if err != nil {
		return err
	}
	wake, rows, err := s.store.GetWake(ctx, wake.WakeMessageID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		s.publish(ctx, row)
	}
	if wake.InputEntryID != "" {
		return s.rewriteWake(ctx, wake, rows)
	}
	if wake.Route == store.SubagentWakeRouteSteer {
		return nil
	}
	return s.deliver(ctx, wake, rows, false)
}

func subagentWakeText(rows []store.SessionSubagent) string {
	rows = append([]store.SessionSubagent(nil), rows...)

	slices.SortStableFunc(rows, func(a, b store.SessionSubagent) int {
		if a.SettledAt == nil || b.SettledAt == nil {
			return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
		}
		return cmp.Or(a.SettledAt.Compare(*b.SettledAt), cmp.Compare(a.ID, b.ID))
	})
	var lines []string
	for _, row := range rows {
		title := strings.Join(strings.Fields(strings.ReplaceAll(row.Title, "\"", "'")), " ")
		line := fmt.Sprintf("Subagent %q (%s) finished: %s.", title, row.ID, row.Status)
		if row.Isolation == SubagentIsolationWorktree && row.WorktreeState().Branch != "" {
			line += " Branch " + row.WorktreeState().Branch
			switch {
			case row.WorktreeState().Facts.PRURL != "":
				line += "; PR " + row.WorktreeState().Facts.PRURL + "."
			case row.WorktreeState().Facts.PRStatus == "none":
				line += "."
			default:
				line += "; PR status unknown."
			}
		}
		lines = append(lines, line)
	}
	suffix := "Call compozy__subagent_status to read each result."
	if len(rows) == 1 {
		suffix = "Call compozy__subagent_status to read its result."
	}
	return strings.Join(append(lines, suffix), "\n")
}
func subagentWakeMeta(rows []store.SessionSubagent) acp.PromptSyntheticMeta {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return acp.PromptSyntheticMeta{Kind: subagentWakeKind, SubagentIDs: ids, Reason: "subagent_settled"}
}

func (s *subagentService) rewriteWake(
	ctx context.Context,
	wake store.SessionSubagentWake,
	rows []store.SessionSubagent,
) error {
	if len(rows) == 0 {
		return s.runtime.CancelWake(ctx, wake)
	}
	metadata, err := json.Marshal(subagentWakeMeta(rows))
	if err != nil {
		return err
	}
	return s.store.RewriteSubagentWakeInput(ctx, wake.WakeMessageID, subagentWakeText(rows), metadata)
}
func canSteerSubagentWake(parent subagentSnapshot, rows []store.SessionSubagent) bool {
	if !parent.Active || !parent.CanSteer || parent.UserSteer {
		return false
	}
	for _, row := range rows {
		if row.WakePolicy != store.SubagentWakePolicyAlways {
			return false
		}
	}
	return true
}

func (s *subagentService) deliver(
	ctx context.Context,
	wake store.SessionSubagentWake,
	rows []store.SessionSubagent,
	forceQueue bool,
) error {
	snap, err := s.runtime.Snapshot(ctx, wake.ParentSessionID)
	if err != nil {
		return err
	}
	if !forceQueue && canSteerSubagentWake(snap, rows) {
		if err := s.store.SetWakeInput(ctx, wake.WakeMessageID, store.SubagentWakeRouteSteer, ""); err != nil {
			return err
		}
		if err := s.store.MarkWakeDispatched(ctx, wake.WakeMessageID); err != nil {
			return err
		}
		attempt, steerErr := s.runtime.Steer(
			ctx,
			wake.ParentSessionID,
			snap.TurnID,
			wake.WakeMessageID,
			subagentWakeText(rows),
		)
		if steerErr == nil && attempt.Attempt == acp.SteerAttemptPendingInjection {
			if attempt.Completion == nil {
				s.mu.Lock()
				if s.steerTurns == nil {
					s.steerTurns = make(map[string]string)
				}
				s.steerTurns[wake.WakeMessageID] = snap.TurnID
				s.mu.Unlock()
				return nil
			}
			s.launch(func() {
				select {
				case <-s.ctx.Done():
					return
				case err, ok := <-attempt.Completion:
					s.logError(
						s.ctx,
						"steer_outcome",
						s.OnSteerOutcome(s.ctx, wake.ParentSessionID, wake.WakeMessageID, ok && err == nil),
					)
				}
			})
			return nil
		}
		return s.steerOutcome(ctx, wake.WakeMessageID, steerErr == nil && attempt.Attempt == acp.SteerAttemptInjected)
	}
	if snap.UserSteer {
		s.logger.InfoContext(
			ctx,
			"subagent.wake_route_fallback",
			"reason",
			"steer_slot_taken",
			"parent_session_id",
			wake.ParentSessionID,
		)
	}
	entry, err := s.runtime.QueueWake(ctx, wake, rows)
	if errors.Is(err, store.ErrSessionInputQueueFull) {
		return s.deferFullWake(ctx, wake)
	}
	if err != nil {
		return err
	}
	return s.store.SetWakeInput(ctx, wake.WakeMessageID, store.SubagentWakeRouteQueue, entry)
}

// subagentQueueFullRetrySteps scales the wake retry delay while the parent's
// input queue has no room (5 × 200 ms).
const subagentQueueFullRetrySteps = 5

// deferFullWake returns a wake that found the parent's input queue full to
// pending and retries it later. A full queue is not a failed turn, so it does
// not consume the wake's attempts.
func (s *subagentService) deferFullWake(ctx context.Context, wake store.SessionSubagentWake) error {
	rows, err := s.settleWakeRows(ctx, wake.WakeMessageID, true)
	if err != nil {
		return err
	}
	for _, row := range rows {
		s.publish(ctx, row)
	}
	s.logger.InfoContext(
		ctx,
		"subagent.wake_deferred",
		"reason",
		"input_queue_full",
		"parent_session_id",
		wake.ParentSessionID,
		"wake_message_id",
		wake.WakeMessageID,
	)
	s.retryWake(wake.ParentSessionID, subagentQueueFullRetrySteps)
	return nil
}
func (s *subagentService) publishID(ctx context.Context, workspace, id string) error {
	row, err := s.store.GetSubagent(ctx, workspace, id)
	if err == nil {
		s.publish(ctx, row)
	}
	return err
}
func (s *subagentService) dispose(ctx context.Context, filter store.SubagentDisposeFilter) error {
	members, err := s.parentRows(ctx, filter.ParentSessionID)
	if err != nil {
		return err
	}
	rows, err := s.store.Dispose(ctx, filter)
	if err != nil {
		return err
	}
	disposed := make(map[string]bool, len(rows))
	for _, row := range rows {
		disposed[row.ID] = true
		s.publish(ctx, row)
	}
	remaining := make(map[string]bool)
	for _, row := range members {
		if row.WakeMessageID != nil && row.Delivery == store.SubagentDeliveryClaimed {
			id := *row.WakeMessageID
			remaining[id] = remaining[id] || !disposed[row.ID]
		}
	}
	for id, claimed := range remaining {
		if !claimed {
			s.forgetSteer(id)
		}
	}
	return nil
}
