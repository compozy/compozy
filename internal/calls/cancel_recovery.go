package calls

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Cancel terminalizes one call when the actor owns its control boundary.
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
	detail := sanitizeDiagnostic(reason, "canceled")
	failureDetail := sanitizeDiagnostic(actor.Kind+":"+actor.ID+": "+detail, detail)
	settled, settledNow, err := s.settleControlledCall(ctx, record, controlledSettlement{
		fenceReason: "call canceled",
		stop: func(current CallRecord) error {
			if current.State != StateRunning || strings.TrimSpace(current.ChildSessionID) == "" {
				return nil
			}
			if s.invoker == nil {
				return errors.New("calls: session invoker is required for cancellation")
			}
			if err := s.invoker.StopManaged(ctx, current.ChildSessionID, detail); err != nil {
				return fmt.Errorf("calls: stop child %q: %w", current.ChildSessionID, err)
			}
			return nil
		},
		mutation: func(current CallRecord) SettlementMutation {
			return SettlementMutation{
				CallID: current.CallID, ExpectedState: current.State, State: StateCanceled,
				FailureCode: "call_canceled", FailureDetail: failureDetail,
				SettledAt: s.now().UTC(),
			}
		},
	})
	if err == nil && settledNow {
		s.emitHook(ctx, HookCallCanceled, hookPayloadForCall(settled))
	}
	return settled, err
}

func requireCallControlActor(record CallRecord, actor Actor) error {
	kind, id := strings.TrimSpace(actor.Kind), strings.TrimSpace(actor.ID)
	if kind == "" || id == "" {
		return newError(CodeSettlementDenied, "cancel actor is required", nil)
	}
	isOperator := kind == "human" || kind == "daemon"
	isBoundActor := kind == record.Actor.Kind && id == record.Actor.ID
	isParentSession := kind == "agent_session" && (id == record.ParentSessionID || id == record.Caller.ID)
	if isOperator || isBoundActor || isParentSession {
		return nil
	}
	return newError(CodeSettlementDenied, "actor may not control this call", nil)
}

// SweepDeadlines terminalizes calls whose configured deadline has elapsed.
func (s *Service) SweepDeadlines(ctx context.Context, now time.Time) (SweepReport, error) {
	if now.IsZero() {
		now = s.now().UTC()
	} else {
		now = now.UTC()
	}
	due, err := s.store.ListDueCalls(ctx, now, callRecoveryBatchLimit)
	if err != nil {
		return SweepReport{}, err
	}
	report := SweepReport{TimedOut: make([]string, 0, len(due))}
	for _, record := range due {
		settled, settledNow, settleErr := s.settleControlledCall(ctx, record, controlledSettlement{
			fenceReason: "call deadline elapsed",
			stop: func(current CallRecord) error {
				if current.State != StateRunning || current.ChildSessionID == "" || s.invoker == nil {
					return nil
				}
				if err := s.invoker.StopManaged(ctx, current.ChildSessionID, "call deadline elapsed"); err != nil {
					return fmt.Errorf("calls: stop timed out child %q: %w", current.ChildSessionID, err)
				}
				return nil
			},
			mutation: func(current CallRecord) SettlementMutation {
				return SettlementMutation{
					CallID: current.CallID, ExpectedState: current.State, State: StateTimeout,
					FailureCode: "call_timeout", FailureDetail: "deadline elapsed", SettledAt: now,
				}
			},
		})
		if !settledNow && settleErr == nil {
			continue
		}
		if settleErr != nil {
			return report, settleErr
		}
		report.TimedOut = append(report.TimedOut, settled.CallID)
	}
	return report, nil
}

// DrainSubtree fences one governed tree and terminalizes every open call in it.
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
	detail := sanitizeDiagnostic(reason, "subtree drained")
	drainPayload := HookPayload{CallID: rootID, RootSessionID: rootID, Actor: actor}
	if len(openCalls) > 0 {
		drainPayload = hookPayloadForCall(openCalls[0])
		drainPayload.CallID = rootID
		drainPayload.RootSessionID = rootID
		drainPayload.Actor = actor
	}
	stopped := make(map[string]struct{}, len(openCalls))
	for _, record := range openCalls {
		settled, settledNow, settleErr := s.settleControlledCall(ctx, record, controlledSettlement{
			fenceReason: "subtree drain",
			stop: func(current CallRecord) error {
				childID := strings.TrimSpace(current.ChildSessionID)
				if childID == "" || s.invoker == nil {
					return nil
				}
				if _, seen := stopped[childID]; seen {
					return nil
				}
				if err := s.invoker.StopManaged(ctx, childID, detail); err != nil {
					return fmt.Errorf("calls: drain child %q: %w", childID, err)
				}
				stopped[childID] = struct{}{}
				report.Stopped = append(report.Stopped, childID)
				return nil
			},
			mutation: func(current CallRecord) SettlementMutation {
				return SettlementMutation{
					CallID: current.CallID, ExpectedState: current.State, State: StateCanceled,
					FailureCode: "call_subtree_drained", FailureDetail: detail,
					SettledAt: s.now().UTC(),
				}
			},
		})
		if !settledNow && settleErr == nil {
			continue
		}
		if settleErr != nil {
			return report, settleErr
		}
		report.CanceledCalls = append(report.CanceledCalls, settled.CallID)
		s.emitHook(ctx, HookCallCanceled, hookPayloadForCall(settled))
	}
	drainPayload.StoppedChildren = len(report.Stopped)
	drainPayload.ClosedCalls = len(report.CanceledCalls)
	drainPayload.PreservedResults = report.PreservedResults
	s.emitHook(ctx, HookCallSubtreeDrained, drainPayload)
	return report, nil
}

type controlledSettlement struct {
	fenceReason string
	stop        func(CallRecord) error
	mutation    func(CallRecord) SettlementMutation
}

func (s *Service) settleControlledCall(
	ctx context.Context,
	record CallRecord,
	options controlledSettlement,
) (CallRecord, bool, error) {
	if record.State.Terminal() {
		return record, false, nil
	}
	if _, err := s.fenceActivation(ctx, record, options.fenceReason); err != nil {
		return CallRecord{}, false, err
	}
	current, err := s.store.GetCallForSettlement(ctx, record.CallID)
	if err != nil {
		return CallRecord{}, false, err
	}
	if current.State.Terminal() {
		return current, false, nil
	}
	if options.stop != nil {
		if err := options.stop(current); err != nil {
			return CallRecord{}, false, err
		}
	}
	settled, err := s.store.SettleCall(ctx, options.mutation(current))
	if IsCode(err, CodeAlreadySettled) {
		latest, loadErr := s.store.GetCallForSettlement(ctx, current.CallID)
		if loadErr != nil {
			return CallRecord{}, false, errors.Join(err, loadErr)
		}
		if latest.State.Terminal() {
			return latest, false, nil
		}
	}
	if err != nil {
		return CallRecord{}, false, err
	}
	s.notifyWaiters(settled.CallID)
	s.emitTerminalTransition(ctx, current.State, settled)
	return settled, true, nil
}
