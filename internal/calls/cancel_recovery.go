package calls

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) Cancel(
	ctx context.Context,
	callID string,
	reason string,
	actor Actor,
) (CallRecord, error) {
	record, err := s.store.GetCallForSettlement(ctx, strings.TrimSpace(callID))
	if err != nil {
		return CallRecord{}, err
	}
	if err := requireCallControlActor(record, actor); err != nil {
		return CallRecord{}, err
	}
	if record.State.Terminal() {
		return record, nil
	}
	outcome, err := s.fenceActivation(ctx, record, "call canceled")
	if err != nil {
		return CallRecord{}, err
	}
	if outcome.Claimed {
		record, err = s.store.GetCallForSettlement(ctx, record.CallID)
		if err != nil {
			return CallRecord{}, err
		}
		if record.State.Terminal() {
			return record, nil
		}
	}
	if record.State == StateRunning && strings.TrimSpace(record.ChildSessionID) != "" {
		if s.invoker == nil {
			return CallRecord{}, errors.New("calls: session invoker is required for cancellation")
		}
		if err := s.invoker.StopManaged(ctx, record.ChildSessionID, strings.TrimSpace(reason)); err != nil {
			return CallRecord{}, fmt.Errorf("calls: stop child %q: %w", record.ChildSessionID, err)
		}
	}
	detail := strings.TrimSpace(reason)
	if detail == "" {
		detail = "canceled"
	}
	settled, err := s.store.SettleCall(ctx, SettlementMutation{
		CallID: record.CallID, ExpectedState: record.State, State: StateCanceled,
		FailureCode: "call_canceled", FailureDetail: actor.Kind + ":" + actor.ID + ": " + detail,
		SettledAt: s.now().UTC(),
	})
	if IsCode(err, CodeAlreadySettled) {
		current, loadErr := s.store.GetCallForSettlement(ctx, record.CallID)
		if loadErr != nil {
			return CallRecord{}, errors.Join(err, loadErr)
		}
		if current.State.Terminal() {
			return current, nil
		}
	}
	if err == nil {
		s.notifyWaiters(record.CallID)
		s.emitHook(ctx, HookCallCanceled, hookPayloadForCall(settled))
	}
	return settled, err
}

func requireCallControlActor(record CallRecord, actor Actor) error {
	kind, id := strings.TrimSpace(actor.Kind), strings.TrimSpace(actor.ID)
	if kind == "" || id == "" {
		return newError(CodeSettlementDenied, "cancel actor is required", nil)
	}
	if kind == "human" || kind == "daemon" ||
		(kind == record.Actor.Kind && id == record.Actor.ID) ||
		(kind == "agent_session" && (id == record.ParentSessionID || id == record.Caller.ID)) {
		return nil
	}
	return newError(CodeSettlementDenied, "actor may not control this call", nil)
}

func (s *Service) SweepDeadlines(ctx context.Context, now time.Time) (SweepReport, error) {
	if now.IsZero() {
		now = s.now().UTC()
	} else {
		now = now.UTC()
	}
	due, err := s.store.ListDueCalls(ctx, now, 100)
	if err != nil {
		return SweepReport{}, err
	}
	report := SweepReport{TimedOut: make([]string, 0, len(due))}
	for _, record := range due {
		if _, err := s.fenceActivation(ctx, record, "call deadline elapsed"); err != nil {
			return report, err
		}
		record, err = s.store.GetCallForSettlement(ctx, record.CallID)
		if err != nil {
			return report, err
		}
		if record.State.Terminal() {
			continue
		}
		if record.State == StateRunning && record.ChildSessionID != "" && s.invoker != nil {
			if err := s.invoker.StopManaged(ctx, record.ChildSessionID, "call deadline elapsed"); err != nil {
				return report, fmt.Errorf("calls: stop timed out child %q: %w", record.ChildSessionID, err)
			}
		}
		settled, settleErr := s.store.SettleCall(ctx, SettlementMutation{
			CallID: record.CallID, ExpectedState: record.State, State: StateTimeout,
			FailureCode: "call_timeout", FailureDetail: "deadline elapsed", SettledAt: now,
		})
		if IsCode(settleErr, CodeAlreadySettled) {
			continue
		}
		if settleErr != nil {
			return report, settleErr
		}
		report.TimedOut = append(report.TimedOut, record.CallID)
		s.notifyWaiters(record.CallID)
		s.emitHook(ctx, HookCallSettled, hookPayloadForCall(settled))
	}
	return report, nil
}

func (s *Service) DrainSubtree(
	ctx context.Context,
	rootSessionID string,
	actor Actor,
	reason string,
) (DrainReport, error) {
	rootID := strings.TrimSpace(rootSessionID)
	if rootID == "" {
		return DrainReport{}, newError(CodeValidation, "root_session_id is required", nil)
	}
	if strings.TrimSpace(actor.Kind) == "" || strings.TrimSpace(actor.ID) == "" {
		return DrainReport{}, newError(CodeSettlementDenied, "drain actor is required", nil)
	}
	if err := s.store.FenceSessionDrain(ctx, rootID, s.now().UTC()); err != nil {
		return DrainReport{}, err
	}
	openCalls, err := s.store.ListOpenSubtreeCalls(ctx, rootID)
	if err != nil {
		return DrainReport{}, err
	}
	preservedResults, err := s.store.CountPreservedSubtreeResults(ctx, rootID)
	if err != nil {
		return DrainReport{}, err
	}
	report := DrainReport{RootSessionID: rootID, PreservedResults: preservedResults}
	drainPayload := HookPayload{CallID: rootID, RootSessionID: rootID, Actor: actor}
	if len(openCalls) > 0 {
		drainPayload = hookPayloadForCall(openCalls[0])
		drainPayload.CallID = rootID
		drainPayload.RootSessionID = rootID
		drainPayload.Actor = actor
	}
	stopped := make(map[string]struct{})
	for _, record := range openCalls {
		if _, err := s.fenceActivation(ctx, record, "subtree drain"); err != nil {
			return report, err
		}
		record, err = s.store.GetCallForSettlement(ctx, record.CallID)
		if err != nil {
			return report, err
		}
		if record.State.Terminal() {
			continue
		}
		childID := strings.TrimSpace(record.ChildSessionID)
		if childID != "" {
			if _, seen := stopped[childID]; !seen && s.invoker != nil {
				if err := s.invoker.StopManaged(ctx, childID, reason); err != nil {
					return report, fmt.Errorf("calls: drain child %q: %w", childID, err)
				}
				stopped[childID] = struct{}{}
				report.Stopped = append(report.Stopped, childID)
			}
		}
		settled, settleErr := s.store.SettleCall(ctx, SettlementMutation{
			CallID: record.CallID, ExpectedState: record.State, State: StateCanceled,
			FailureCode: "call_subtree_drained", FailureDetail: strings.TrimSpace(reason),
			SettledAt: s.now().UTC(),
		})
		if IsCode(settleErr, CodeAlreadySettled) {
			continue
		}
		if settleErr != nil {
			return report, settleErr
		}
		report.CanceledCalls = append(report.CanceledCalls, record.CallID)
		s.notifyWaiters(record.CallID)
		s.emitHook(ctx, HookCallCanceled, hookPayloadForCall(settled))
	}
	drainPayload.StoppedChildren = len(report.Stopped)
	drainPayload.ClosedCalls = len(report.CanceledCalls)
	drainPayload.PreservedResults = report.PreservedResults
	s.emitHook(ctx, HookCallSubtreeDrained, drainPayload)
	return report, nil
}
