package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func (b *loopActionSessionBinder) prepareBindingAttempt(
	ctx context.Context,
	key goalpkg.BindingKey,
	epoch int64,
	attemptID string,
	sessionID string,
	identity store.SessionCreationIdentity,
) (goalpkg.SessionBinding, error) {
	return b.bindings.PrepareSessionBindingAttempt(ctx, goalpkg.PrepareBindingAttemptRequest{
		Key:                key,
		BindingEpoch:       epoch,
		BindingAttemptID:   attemptID,
		SessionID:          sessionID,
		CreationProfileRef: identity.CreationProfileRef,
		PolicySpecDigest:   identity.PolicySpecDigest,
		CreationDigest:     identity.CreationDigest,
	})
}

func (b *loopActionSessionBinder) prepareRunOwnedBinding(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
	profile store.SessionCreationProfile,
	opts session.CreateOpts,
) (goalpkg.SessionBinding, store.SessionCreationIdentity, error) {
	if strings.TrimSpace(req.ExpectedCheckpointPhase) != "" {
		if b.bindingAllocator == nil {
			return goalpkg.SessionBinding{}, store.SessionCreationIdentity{}, errors.New(
				"daemon: Goal binding attempt allocator is unavailable",
			)
		}
		prepared, err := b.bindingAllocator.AllocateSessionBindingAttempt(
			ctx,
			&goalpkg.AllocateBindingAttemptRequest{
				Key:                            key,
				CheckpointKey:                  bindingCheckpointKey(req),
				TargetBindingEpoch:             req.TargetBindingEpoch,
				ExpectedControlEpoch:           req.ExpectedControlEpoch,
				ExpectedCheckpointPhase:        req.ExpectedCheckpointPhase,
				ExpectedTaskRunID:              req.ExpectedTaskRunID,
				ExpectedQueueEntryID:           req.ExpectedQueueEntryID,
				ExpectedPromptID:               req.ExpectedPromptID,
				ExpectedCheckpointBindingEpoch: req.ExpectedCheckpointBindingEpoch,
				ExpectedCheckpointSessionID:    req.ExpectedCheckpointSessionID,
				ExpectedCheckpointHandle:       req.ExpectedCheckpointHandle,
				IdentityHandle:                 strings.TrimSpace(req.Handle),
				CreationProfile:                profile,
				CreationOptions:                bindingCreationOptions(opts, ""),
			},
		)
		if err != nil {
			return goalpkg.SessionBinding{}, store.SessionCreationIdentity{}, err
		}
		return prepared, bindingIdentityFromGoal(prepared), nil
	}

	epoch := req.TargetBindingEpoch
	attemptID, sessionID := bindingAttemptIdentity(req, epoch)
	identity, err := bindingCreationIdentity(profile, opts, sessionID)
	if err != nil {
		return goalpkg.SessionBinding{}, store.SessionCreationIdentity{}, err
	}
	prepared, err := b.prepareBindingAttempt(ctx, key, epoch, attemptID, sessionID, identity)
	return prepared, identity, err
}

func bindingCheckpointKey(req looppkg.ActionSessionBindRequest) goalpkg.TurnKey {
	return goalpkg.TurnKey{
		WorkspaceID: req.WorkspaceID,
		LoopRunID:   req.LoopRunID,
		Generation:  req.Generation,
		NodeID:      req.NodeID,
		ItemIndex:   req.ItemIndex,
	}
}

func bindingIdentityFromGoal(binding goalpkg.SessionBinding) store.SessionCreationIdentity {
	return store.SessionCreationIdentity{
		CreationProfileRef: binding.CreationProfileRef,
		PolicySpecDigest:   binding.PolicySpecDigest,
		CreationDigest:     binding.CreationDigest,
	}
}

func activationRequest(
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
	prepared goalpkg.SessionBinding,
) goalpkg.ActivateBindingRequest {
	var checkpointKey *goalpkg.TurnKey
	var cellFence *goalpkg.BindingCellFence
	if strings.TrimSpace(req.ExpectedCheckpointPhase) != "" {
		checkpointKey = &goalpkg.TurnKey{
			WorkspaceID: req.WorkspaceID,
			LoopRunID:   req.LoopRunID,
			Generation:  req.Generation,
			NodeID:      req.NodeID,
			ItemIndex:   req.ItemIndex,
		}
	} else if req.CellFence != nil {
		cellFence = &goalpkg.BindingCellFence{
			Key: goalpkg.TurnKey{
				WorkspaceID: req.WorkspaceID,
				LoopRunID:   req.LoopRunID,
				Generation:  req.Generation,
				NodeID:      req.NodeID,
				ItemIndex:   req.ItemIndex,
			},
			Epoch:     req.CellFence.Epoch,
			TaskRunID: strings.TrimSpace(req.CellFence.TaskRunID),
		}
	}
	return goalpkg.ActivateBindingRequest{
		Key:                  key,
		CheckpointKey:        checkpointKey,
		CellFence:            cellFence,
		ExpectedBindingEpoch: prepared.BindingEpoch,
		ExpectedControlEpoch: req.ExpectedControlEpoch,
		GrantID:              req.ReseedGrantID,
	}
}

func (b *loopActionSessionBinder) loadActiveBinding(
	ctx context.Context,
	key goalpkg.BindingKey,
) (goalpkg.SessionBinding, bool, error) {
	binding, err := b.bindings.GetActiveSessionBinding(ctx, key)
	if errors.Is(err, goalpkg.ErrBindingNotFound) {
		return goalpkg.SessionBinding{}, false, nil
	}
	return binding, err == nil, err
}

func (b *loopActionSessionBinder) validateActiveBindingPolicy(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	binding goalpkg.SessionBinding,
) (looppkg.RuntimeSpec, error) {
	identity := store.SessionCreationIdentity{
		CreationProfileRef: binding.CreationProfileRef,
		PolicySpecDigest:   binding.PolicySpecDigest,
		CreationDigest:     binding.CreationDigest,
	}
	return b.revalidatePersistedProfile(ctx, req, identity, false)
}

func (b *loopActionSessionBinder) revalidatePersistedProfile(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	identity store.SessionCreationIdentity,
	allowRuntimeOverride bool,
) (looppkg.RuntimeSpec, error) {
	profile, opts, materialized, err := b.resolvePinnedCreationProfile(
		ctx,
		req,
		identity.CreationProfileRef,
		false,
		allowRuntimeOverride,
	)
	if err != nil {
		return looppkg.RuntimeSpec{}, err
	}
	if materialized != nil {
		return looppkg.RuntimeSpec{}, b.rollbackLoopActionEnvironment(
			ctx,
			req,
			materialized,
			errors.New("daemon: persisted profile revalidation materialized a new worktree"),
		)
	}
	profileRef, err := profile.Ref()
	if err != nil {
		return looppkg.RuntimeSpec{}, err
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return looppkg.RuntimeSpec{}, err
	}
	if profileRef != identity.CreationProfileRef || policyDigest != identity.PolicySpecDigest {
		return looppkg.RuntimeSpec{}, bindingMismatch(
			"persisted creation profile no longer passes the effective policy gate",
		)
	}
	applied := appliedRuntimeFromCreateOptions(opts)
	if allowRuntimeOverride && req.Runtime != nil {
		requested := req.RuntimeValue()
		if strings.TrimSpace(requested.Provider) != "" {
			applied.Provider = strings.TrimSpace(requested.Provider)
		}
		if strings.TrimSpace(requested.Model) != "" {
			applied.Model = strings.TrimSpace(requested.Model)
		}
		if strings.TrimSpace(requested.Reasoning) != "" {
			applied.Reasoning = strings.TrimSpace(requested.Reasoning)
		}
		if requested.Speed != "" {
			applied.Speed = requested.Speed
		}
		if len(requested.ACPOptions) > 0 {
			applied.ACPOptions = loopACPOptionsFromSession(loopACPOptionsForSession(requested.ACPOptions))
		}
	}
	return applied, nil
}

func actionBindingFromGoal(
	req looppkg.ActionSessionBindRequest,
	binding goalpkg.SessionBinding,
	appliedRuntime looppkg.RuntimeSpec,
) looppkg.ActionSessionBinding {
	return looppkg.ActionSessionBinding{
		WorkspaceID:        req.WorkspaceID,
		LoopRunID:          req.LoopRunID,
		SessionID:          binding.SessionID,
		Handle:             strings.TrimSpace(req.Handle),
		SharedKey:          binding.Key.Handle,
		ControlEpoch:       req.ExpectedControlEpoch,
		BindingEpoch:       binding.BindingEpoch,
		BindingAttemptID:   binding.BindingAttemptID,
		CreationProfileRef: binding.CreationProfileRef,
		PolicySpecDigest:   binding.PolicySpecDigest,
		CreationDigest:     binding.CreationDigest,
		State:              string(binding.State),
		Ownership:          string(binding.Ownership),
		Isolated:           req.Isolated,
		AppliedRuntime:     appliedRuntime,
	}
}

func (b *loopActionSessionBinder) projectActionBindingSpeed(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
	bindErr error,
) (looppkg.ActionSessionBinding, error) {
	if bindErr != nil || strings.TrimSpace(binding.SessionID) == "" {
		return binding, bindErr
	}
	info, err := b.sessions.Status(ctx, binding.SessionID)
	if err != nil {
		return looppkg.ActionSessionBinding{}, fmt.Errorf(
			"daemon: load applied Loop runtime for session %q: %w",
			binding.SessionID,
			err,
		)
	}
	if info == nil {
		return looppkg.ActionSessionBinding{}, fmt.Errorf(
			"daemon: load applied Loop runtime for session %q: empty session info",
			binding.SessionID,
		)
	}
	if info.Speed != "" {
		binding.AppliedRuntime.Speed = info.Speed
	}
	binding.SpeedResolution = speedpkg.CloneResolution(info.SpeedResolution)
	return binding, nil
}

func (b *loopActionSessionBinder) PromptActionSession(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
	req looppkg.ActionPromptRequest,
) (looppkg.ActionPromptResult, error) {
	if b == nil || b.sessions == nil {
		return looppkg.ActionPromptResult{}, errors.New("daemon: loop action sessions are unavailable")
	}
	return collectLoopPromptResult(ctx, b.sessions, strings.TrimSpace(binding.SessionID), req)
}

func (b *loopActionSessionBinder) CancelActionSession(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
) error {
	if b == nil || b.sessions == nil {
		return nil
	}
	_, err := b.sessions.CancelPrompt(ctx, strings.TrimSpace(binding.SessionID))
	return err
}

func bindingMismatch(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeContinuousBindingMismatch,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, strings.TrimSpace(detail)),
	}
}
