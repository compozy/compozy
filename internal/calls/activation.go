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
	record CallRecord,
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
	childID := strings.TrimSpace(activation.TargetSessionID)
	createdChild := false
	var invokeErr error
	switch activation.Kind {
	case "spawn":
		child, err := s.invoker.SpawnChild(ctx, ChildSpec{
			CallID: record.CallID, ParentSessionID: activation.ParentSessionID,
			AgentName: activation.AgentName, Prompt: string(admission.Prompt),
			WorkspaceID: activation.WorkspaceID, IdleTTL: activation.IdleTTL,
			Runtime: activation.Runtime, Permissions: admission.Narrow,
		})
		if err != nil {
			invokeErr = err
		} else {
			childID = strings.TrimSpace(child.ID)
			createdChild = true
		}
	case "revive":
		invokeErr = s.invoker.Revive(ctx, childID, string(admission.Prompt), record.CallID)
	default:
		invokeErr = fmt.Errorf("unsupported activation kind %q", activation.Kind)
	}
	if invokeErr != nil {
		failureCode := "call_activation_failed"
		var callErr *Error
		if errors.As(invokeErr, &callErr) && callErr.Code != "" {
			failureCode = string(callErr.Code)
		}
		failed, settleErr := s.store.FailActivation(ctx, ActivationFailure{
			CallID: record.CallID, RunID: activation.RunID, ClaimToken: claim.ClaimToken,
			Code: failureCode, Detail: invokeErr.Error(), FailedAt: s.now().UTC(),
		})
		if settleErr != nil {
			releaseErr := s.releaseActivationClaim(ctx, claim, "activation settlement failed")
			return CallRecord{}, errors.Join(invokeErr, settleErr, releaseErr)
		}
		s.notifyWaiters(record.CallID)
		return failed, nil
	}
	bound, err := s.store.BindActivationChild(ctx, ActivationBinding{
		CallID: record.CallID, RunID: activation.RunID, ClaimToken: claim.ClaimToken,
		ChildID: childID, ActivatedAt: s.now().UTC(),
	})
	if err == nil {
		return bound, nil
	}
	var cleanupErr error
	if createdChild {
		cleanupErr = s.invoker.StopManaged(ctx, childID, "call activation persistence failed")
	}
	releaseErr := s.releaseActivationClaim(ctx, claim, "activation persistence failed")
	return CallRecord{}, errors.Join(err, cleanupErr, releaseErr)
}

// DispatchQueued claims call activations from the durable task queue and invokes them.
func (s *Service) DispatchQueued(ctx context.Context, limit int) (int, error) {
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
		_, err = s.invokeClaimedActivation(ctx, record, Admission{
			Record: record, Prompt: prompt, Narrow: permissions, Activation: &activation,
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
		if _, err := s.DrainSubtree(ctx, rootID, Actor{Kind: "daemon", ID: "calls.recovery"}, "resume interrupted drain"); err != nil {
			return err
		}
	}
	_, err = s.DispatchQueued(ctx, 100)
	return err
}

func (s *Service) releaseActivationClaim(
	ctx context.Context,
	claim *task.ClaimResult,
	reason string,
) error {
	if s.claimer == nil || claim == nil {
		return nil
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
