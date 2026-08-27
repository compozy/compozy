package calls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/contracts"
)

// RequireCallSettlementActor accepts only the child session bound to the call.
func RequireCallSettlementActor(record *CallRecord, actor SettlementActor) error {
	if strings.TrimSpace(actor.Kind) != actorKindAgentSession ||
		strings.TrimSpace(actor.ID) != strings.TrimSpace(record.ChildSessionID) {
		return newError(CodeSettlementDenied, "only the bound child session may settle this call", nil)
	}
	return nil
}

// Return validates and settles one result from its bound child session.
func (s *Service) Return(ctx context.Context, input ReturnInput) (Settlement, error) {
	callID := strings.TrimSpace(input.CallID)
	childID := strings.TrimSpace(input.ChildSessionID)
	if childID == "" {
		childID = strings.TrimSpace(input.Actor.ID)
	}
	var record CallRecord
	var err error
	if callID == "" {
		record, err = s.store.GetOpenCallForChild(ctx, childID)
	} else {
		record, err = s.store.GetCallForSettlement(ctx, callID)
	}
	if err != nil {
		return Settlement{}, err
	}
	if err := RequireCallSettlementActor(&record, input.Actor); err != nil {
		return Settlement{}, err
	}
	if record.State.Terminal() {
		if len(input.Result) > 0 {
			return s.recordSupersededResult(ctx, &record, input.Result)
		}
		return Settlement{}, newError(CodeAlreadySettled, fmt.Sprintf("call is %s", record.State), nil)
	}
	if len(input.Result) > 0 {
		return s.returnPayload(ctx, &record, input.Result, input.ChildLive)
	}
	return s.returnFinalText(ctx, &record, input.FinalText, input.ChildLive)
}

func (s *Service) returnPayload(
	ctx context.Context,
	record *CallRecord,
	raw json.RawMessage,
	childLive bool,
) (Settlement, error) {
	payload, verdict, issues, err := s.validateReturnedPayload(ctx, record, raw)
	if err != nil {
		return Settlement{}, err
	}
	if len(issues) > 0 {
		return s.handleInvalidResult(ctx, record, issues, childLive)
	}
	return s.settleWithPayload(ctx, record, payload, verdict)
}

func (s *Service) validateReturnedPayload(
	ctx context.Context,
	record *CallRecord,
	raw json.RawMessage,
) (json.RawMessage, Verdict, []contracts.ValidationIssue, error) {
	if !json.Valid(raw) {
		return nil, "", []contracts.ValidationIssue{{Path: "$", Message: "result must be valid JSON"}}, nil
	}
	if record.ExpectDigest == "" {
		clean, issues := sanitizeJSONResult(raw)
		return clean, VerdictReturned, issues, nil
	}
	contract, err := s.registry.Resolve(ctx, record.ExpectDigest)
	if err != nil {
		return nil, "", nil, err
	}
	redacted, redactions, redactErr := contracts.RedactPreservingContract(contract, raw)
	if redactErr == nil {
		verdict, validateErr := s.registry.Validate(ctx, record.ExpectDigest, redacted)
		if validateErr != nil {
			return nil, "", nil, validateErr
		}
		if verdict.Unwrapped {
			redacted = contracts.UnwrapSingleObject(redacted)
		}
		return redacted, VerdictReturned, nil, nil
	}
	if len(redactions) > 0 {
		return nil, "", []contracts.ValidationIssue{{
			Path:    "$",
			Message: "result contains secret material in a contract-constrained field",
		}}, nil
	}
	clean, issues := sanitizeJSONResult(raw)
	if len(issues) > 0 {
		return nil, "", issues, nil
	}
	verdict, validateErr := s.registry.Validate(ctx, record.ExpectDigest, clean)
	if validateErr != nil {
		return nil, "", nil, validateErr
	}
	if verdict.Valid {
		payload := clean
		if verdict.Unwrapped {
			payload = contracts.UnwrapSingleObject(payload)
		}
		return payload, VerdictReturned, nil, nil
	}
	return nil, "", verdict.Issues, nil
}

func (s *Service) handleInvalidResult(
	ctx context.Context,
	record *CallRecord,
	issues []contracts.ValidationIssue,
	childLive bool,
) (Settlement, error) {
	prompt := contracts.BuildRepairPrompt(issues)
	if record.RepairAttempts == 0 && childLive {
		updated, err := s.store.RecordRepair(ctx, RepairMutation{
			CallID: record.CallID, IssueText: prompt, At: s.now().UTC(),
		})
		if err != nil {
			return Settlement{}, err
		}
		return Settlement{Call: updated, RepairPrompt: prompt, Issues: issues}, nil
	}
	settled, err := s.settleTerminal(ctx, record, SettlementMutation{
		CallID: record.CallID, ExpectedState: record.State, State: StateInvalidResult,
		FailureCode: string(CodeResultInvalid), FailureDetail: prompt, SecondIssueText: prompt,
		SettledAt: s.now().UTC(),
	})
	if err != nil {
		return Settlement{}, err
	}
	return Settlement{Call: settled, Issues: issues}, nil
}

func (s *Service) settleWithPayload(
	ctx context.Context,
	record *CallRecord,
	payload json.RawMessage,
	verdict Verdict,
) (Settlement, error) {
	verdict = effectiveVerdict(record, verdict)
	outcome, err := contracts.EnforceBudget(record.ResultBudget, payload)
	if err != nil {
		settled, settleErr := s.settleTerminal(ctx, record, SettlementMutation{
			CallID: record.CallID, ExpectedState: record.State, State: StateFailed,
			FailureCode: string(CodeResultOverBudget), FailureDetail: err.Error(), SettledAt: s.now().UTC(),
		})
		if settleErr != nil {
			return Settlement{}, errors.Join(err, settleErr)
		}
		return Settlement{Call: settled}, nil
	}
	ref := contracts.OutputRefForPayload(outcome.Payload)
	settled, err := s.settleTerminal(ctx, record, SettlementMutation{
		CallID: record.CallID, ExpectedState: record.State, State: StateCompleted, Verdict: verdict,
		Result: outcome.Payload, ResultRef: ref, ResultBytes: len(outcome.Payload), SettledAt: s.now().UTC(),
	})
	if IsCode(err, CodeAlreadySettled) {
		current, loadErr := s.store.GetCallForSettlement(ctx, record.CallID)
		if loadErr != nil {
			return Settlement{}, errors.Join(err, loadErr)
		}
		if current.State.Terminal() {
			return s.recordSupersededResult(ctx, &current, outcome.Payload)
		}
	}
	return Settlement{Call: settled}, err
}

func (s *Service) returnFinalText(
	ctx context.Context,
	record *CallRecord,
	finalText string,
	childLive bool,
) (Settlement, error) {
	clean, _, reject := contracts.SanitizeText(finalText)
	if reject {
		clean = ""
	}
	if !record.Strict {
		var newestIssues []contracts.ValidationIssue
		for _, candidate := range contracts.ExtractCandidates(clean) {
			payload, _, issues, err := s.validateReturnedPayload(ctx, record, candidate)
			if err != nil {
				return Settlement{}, err
			}
			if len(issues) == 0 {
				return s.settleWithPayload(ctx, record, payload, VerdictExtracted)
			}
			if newestIssues == nil {
				newestIssues = issues
			}
		}
		if newestIssues != nil {
			return s.handleInvalidResult(ctx, record, newestIssues, childLive)
		}
	}
	settled, err := s.settleTerminal(ctx, record, SettlementMutation{
		CallID: record.CallID, ExpectedState: record.State, State: StateCompletedWithoutResult,
		FinalProsePreview: clean, SettledAt: s.now().UTC(),
	})
	return Settlement{Call: settled}, err
}

func (s *Service) settleTerminal(
	ctx context.Context,
	record *CallRecord,
	mutation SettlementMutation,
) (CallRecord, error) {
	if err := s.fenceActivation(ctx, record, "call terminal settlement"); err != nil {
		return CallRecord{}, err
	}
	settled, err := s.store.SettleCall(ctx, mutation)
	if err == nil {
		s.notifyWaiters(record.CallID)
		s.emitTerminalTransition(ctx, record.State, &settled)
		if parkErr := s.parkSettledChild(ctx, &settled); parkErr != nil {
			s.logger.WarnContext(
				ctx,
				"calls: settled child parking failed after durable settlement",
				"call_id", settled.CallID,
				"child_session_id", settled.ChildSessionID,
				"error", sanitizeDiagnostic(parkErr.Error(), "child parking failed"),
			)
		}
	}
	return settled, err
}

func (s *Service) parkSettledChild(ctx context.Context, record *CallRecord) error {
	childID := strings.TrimSpace(record.ChildSessionID)
	if childID == "" {
		return nil
	}
	if s.invoker == nil {
		return errors.New("calls: session invoker is required for child parking")
	}
	mailbox, err := s.mailboxStore()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	eligible, err := mailbox.ParkCallChild(ctx, childID, now, now.Add(record.IdleTTL))
	if err != nil {
		return fmt.Errorf("calls: inspect child %q park eligibility: %w", childID, err)
	}
	if !eligible {
		return nil
	}
	stopCtx, cancel := s.detachedOperationContext(ctx)
	defer cancel()
	if err := s.invoker.StopManaged(stopCtx, childID, "call child parked"); err != nil {
		clearCtx, clearCancel := s.detachedOperationContext(ctx)
		defer clearCancel()
		clearErr := mailbox.ClearCallChildIdleClock(clearCtx, childID, now)
		return errors.Join(fmt.Errorf("calls: park child runtime %q: %w", childID, err), clearErr)
	}
	return nil
}

func (s *Service) fenceActivation(
	ctx context.Context,
	record *CallRecord,
	reason string,
) error {
	if strings.TrimSpace(record.ActivationRunID) == "" {
		return nil
	}
	if s.canceler == nil {
		return errors.New("calls: activation run canceler is required")
	}
	_, err := s.canceler.CancelActivationRun(ctx, record.ActivationRunID, reason)
	if err != nil {
		return fmt.Errorf("calls: fence activation run %q: %w", record.ActivationRunID, err)
	}
	return nil
}

func (s *Service) recordSupersededResult(
	ctx context.Context,
	record *CallRecord,
	payload json.RawMessage,
) (Settlement, error) {
	clean, issues := sanitizeJSONResult(payload)
	if len(issues) > 0 {
		return Settlement{}, newError(CodeResultInvalid, issues[0].Message, nil)
	}
	ref := contracts.OutputRefForPayload(clean)
	updated, err := s.store.SettleCall(ctx, SettlementMutation{
		CallID: record.CallID, Superseded: clean, SupersededRef: ref, SettledAt: s.now().UTC(),
	})
	if err != nil {
		return Settlement{}, err
	}
	return Settlement{Call: updated}, newError(CodeAlreadySettled, fmt.Sprintf("call is %s", record.State), nil)
}

func effectiveVerdict(record *CallRecord, verdict Verdict) Verdict {
	if record.RepairAttempts > 0 {
		return VerdictRepaired
	}
	return verdict
}

func sanitizeJSONResult(raw json.RawMessage) (json.RawMessage, []contracts.ValidationIssue) {
	if !json.Valid(raw) {
		return nil, []contracts.ValidationIssue{{Path: "$", Message: "result must be valid JSON"}}
	}
	clean, _, reject := contracts.SanitizeText(string(raw))
	if reject || !json.Valid([]byte(clean)) {
		return nil, []contracts.ValidationIssue{{Path: "$", Message: "result contains unsafe secret material"}}
	}
	return json.RawMessage(clean), nil
}
