package calls

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/task"
)

func (s *Service) invokeClaimedActivation(
	ctx context.Context,
	record *CallRecord,
	admission Admission,
	claim *task.ClaimResult,
) (CallRecord, error) {
	if claim == nil || strings.TrimSpace(claim.ClaimToken) == "" {
		return CallRecord{}, newError(CodeValidation, "activation claim is incomplete", nil)
	}
	activation := admission.Activation
	if activation == nil {
		return CallRecord{}, newError(CodeValidation, "activation specification is missing", nil)
	}
	remainingDepth := max(s.config.MaxDepth-activation.Depth, 0)
	childID, createdChild, invokeErr := s.invokeActivation(ctx, record, admission, activation, remainingDepth)
	if invokeErr != nil {
		return s.failClaimedActivation(ctx, record, activation, claim, invokeErr)
	}
	bound, err := s.store.BindActivationChild(ctx, ActivationBinding{
		CallID: record.CallID, RunID: activation.RunID, ClaimToken: claim.ClaimToken,
		ChildID: childID, ActivatedAt: s.now().UTC(),
	})
	if err == nil {
		s.emitStateChanged(ctx, record.State, &bound)
		if activation.Kind == ActivationKindRevive {
			s.emitHook(ctx, HookCallRevived, hookPayloadForCall(&bound))
		}
		return bound, nil
	}
	var cleanupErr error
	if createdChild {
		cleanupCtx, cancel := s.detachedOperationContext(ctx)
		cleanupErr = s.invoker.StopManaged(cleanupCtx, childID, "call activation persistence failed")
		cancel()
	}
	if latest, handled, raceErr := s.resolveActivationSettlementRace(ctx, record.CallID, err, cleanupErr); handled {
		return latest, raceErr
	}
	releaseErr := s.releaseActivationClaim(ctx, claim, "activation persistence failed")
	return CallRecord{}, errors.Join(err, cleanupErr, releaseErr)
}

func (s *Service) invokeActivation(
	ctx context.Context,
	record *CallRecord,
	admission Admission,
	activation *ActivationSpec,
	remainingDepth int,
) (string, bool, error) {
	childID := strings.TrimSpace(activation.TargetSessionID)
	switch activation.Kind {
	case ActivationKindSpawn:
		child, err := s.invoker.SpawnChild(ctx, ChildSpec{
			CallID: record.CallID, ParentSessionID: activation.ParentSessionID,
			AgentName: activation.AgentName, Prompt: string(admission.Prompt),
			WorkspaceID: activation.WorkspaceID, IdleTTL: activation.IdleTTL,
			Runtime: activation.Runtime, Permissions: admission.Narrow,
			RemainingDepth: remainingDepth,
		})
		if err != nil {
			return childID, false, err
		}
		return strings.TrimSpace(child.ID), true, nil
	case ActivationKindRevive:
		return childID, false, s.invoker.Revive(
			ctx,
			childID,
			CallPromptWithRemainingDepth(string(admission.Prompt), remainingDepth),
			record.CallID,
		)
	default:
		return childID, false, fmt.Errorf("unsupported activation kind %q", activation.Kind)
	}
}

func (s *Service) failClaimedActivation(
	ctx context.Context,
	record *CallRecord,
	activation *ActivationSpec,
	claim *task.ClaimResult,
	invokeErr error,
) (CallRecord, error) {
	failureCode := "call_activation_failed"
	var callErr *Error
	if errors.As(invokeErr, &callErr) && callErr.Code != "" {
		failureCode = string(callErr.Code)
	}
	failed, settleErr := s.store.FailActivation(ctx, ActivationFailure{
		CallID:     record.CallID,
		RunID:      activation.RunID,
		ClaimToken: claim.ClaimToken,
		Code:       failureCode,
		Detail:     sanitizeDiagnostic(invokeErr.Error(), "activation failed"),
		FailedAt:   s.now().UTC(),
	})
	if settleErr != nil {
		if latest, handled, raceErr := s.resolveActivationSettlementRace(
			ctx,
			record.CallID,
			settleErr,
			nil,
		); handled {
			return latest, raceErr
		}
		releaseErr := s.releaseActivationClaim(ctx, claim, "activation settlement failed")
		return CallRecord{}, errors.Join(invokeErr, settleErr, releaseErr)
	}
	s.notifyWaiters(record.CallID)
	s.emitTerminalTransition(ctx, record.State, &failed)
	return failed, nil
}

func (s *Service) resolveActivationSettlementRace(
	ctx context.Context,
	callID string,
	settleErr error,
	cleanupErr error,
) (CallRecord, bool, error) {
	if !IsCode(settleErr, CodeAlreadySettled) {
		return CallRecord{}, false, nil
	}
	latest, loadErr := s.store.GetCallForSettlement(ctx, callID)
	if loadErr != nil {
		return CallRecord{}, true, errors.Join(settleErr, cleanupErr, loadErr)
	}
	if !latest.State.Terminal() {
		return CallRecord{}, false, nil
	}
	if cleanupErr != nil {
		return latest, true, cleanupErr
	}
	return latest, true, nil
}

// DispatchQueued claims call activations from the durable task queue and invokes them.
func (s *Service) DispatchQueued(ctx context.Context, limit int) (int, error) {
	if s.claimer == nil {
		return 0, errors.New("calls: activation claimer is required")
	}
	if s.invoker == nil {
		return 0, errors.New("calls: session invoker is required")
	}
	runIDs, err := s.store.ListQueuedActivationRunIDs(ctx, limit)
	if err != nil {
		return 0, err
	}
	dispatched := 0
	for _, runID := range runIDs {
		record, activation, prompt, permissions, err := s.store.LoadActivation(ctx, runID)
		if err != nil {
			return dispatched, err
		}
		actor := activationDaemonActor(record.WorkspaceID)
		claim, err := s.claimer.ClaimNextRun(ctx, task.ClaimCriteria{
			RunID: runID, RunKind: task.RunKindCallActivation,
			Scope: task.Scope(record.Scope), WorkspaceID: record.WorkspaceID,
		}, actor)
		if errors.Is(err, task.ErrNoClaimableRun) {
			continue
		}
		if err != nil {
			return dispatched, fmt.Errorf("calls: claim queued activation %q: %w", runID, err)
		}
		_, err = s.invokeClaimedActivation(ctx, &record, Admission{
			Record: &record, Prompt: prompt, Narrow: permissions, Activation: &activation,
		}, claim)
		if err != nil {
			return dispatched, err
		}
		dispatched++
	}
	return dispatched, nil
}

// RecoverCallRuntime repairs activation crash windows and resumes fenced subtree drains.
func (s *Service) RecoverCallRuntime(ctx context.Context) error {
	drainRoots, err := s.store.ReconcileActivations(ctx, s.now().UTC())
	if err != nil {
		return err
	}
	for _, rootID := range drainRoots {
		if _, err := s.DrainSubtree(
			ctx,
			rootID,
			Actor{Kind: "daemon", ID: "calls.recovery"},
			"resume interrupted drain",
		); err != nil {
			return err
		}
	}
	_, err = s.DispatchQueued(ctx, callRecoveryBatchLimit)
	return err
}

func (s *Service) releaseActivationClaim(
	ctx context.Context,
	claim *task.ClaimResult,
	reason string,
) error {
	if claim == nil {
		return nil
	}
	if s.claimer == nil {
		return errors.New("calls: activation claimer is required to release a claim")
	}
	actor := activationDaemonActor(claim.Run.WorkspaceID)
	_, err := s.claimer.ReleaseRunLease(ctx, task.LeaseRelease{
		RunID: claim.Run.ID, ClaimToken: claim.ClaimToken, Reason: reason,
		Now: s.now().UTC(), Actor: actor,
	}, actor)
	if err != nil {
		return fmt.Errorf("calls: release activation claim %q: %w", claim.Run.ID, err)
	}
	return nil
}

func activationDaemonActor(workspaceID string) task.ActorContext {
	return task.ActorContext{
		Actor:     task.ActorIdentity{Kind: task.ActorKindDaemon, Ref: "calls.activation"},
		Origin:    task.Origin{Kind: task.OriginKindDaemon, Ref: "calls.activation"},
		Authority: task.Authority{Read: true, Write: true},
		Scope:     task.CallerScope{WorkspaceID: strings.TrimSpace(workspaceID)},
	}
}
