package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

type loopActionSessionBinder struct {
	sessions            loopPromptSessionManager
	bindings            goalpkg.BindingStore
	prompts             loopGoalPromptRuntimeStore
	creationStore       store.SessionCreationStore
	managedInputs       loopManagedInputSessionManager
	usageReporters      managedGoalUsageReporters
	globalWorkspacePath string
	policyGate          *loopSessionPolicyGate
	now                 func() time.Time
}

type loopManagedSessionManager interface {
	session.IdempotentSessionCreator
}

var _ looppkg.ActionSessionBinder = (*loopActionSessionBinder)(nil)
var _ looppkg.ManagedActionSessionBinder = (*loopActionSessionBinder)(nil)

func (b *loopActionSessionBinder) BindActionSession(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
) (looppkg.ActionSessionBinding, error) {
	if b == nil || b.sessions == nil {
		return looppkg.ActionSessionBinding{}, errors.New("daemon: loop action sessions are unavailable")
	}
	if strings.TrimSpace(string(req.LoopRunID)) == "" || req.TargetBindingEpoch < 1 {
		return b.bindEphemeralActionSession(ctx, req)
	}
	if b.bindings == nil || b.creationStore == nil {
		return looppkg.ActionSessionBinding{}, errors.New("daemon: managed loop binding stores are unavailable")
	}
	creator, ok := b.sessions.(loopManagedSessionManager)
	if !ok {
		return looppkg.ActionSessionBinding{}, errors.New("daemon: idempotent session creation is unavailable")
	}
	key := goalpkg.BindingKey{
		WorkspaceID: req.WorkspaceID,
		LoopRunID:   req.LoopRunID,
		Handle:      strings.TrimSpace(req.Handle),
	}
	if err := key.Validate(); err != nil {
		return looppkg.ActionSessionBinding{}, err
	}

	active, activeFound, err := b.loadActiveBinding(ctx, key)
	if err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	if activeFound {
		binding, handled, err := b.bindFromActiveSession(ctx, req, key, active)
		if err != nil || handled {
			return binding, err
		}
	}
	return b.bindMissingOrAdvancedSession(ctx, creator, req, key, active, activeFound)
}

func (b *loopActionSessionBinder) bindEphemeralActionSession(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
) (looppkg.ActionSessionBinding, error) {
	agent := strings.TrimSpace(req.Agent)
	if agent == "" {
		return looppkg.ActionSessionBinding{}, fmt.Errorf("%w: run-agent agent is required", looppkg.ErrValidation)
	}
	opts := b.baseCreateOptions(req, agent, "action")
	if _, err := b.policyGate.applyResolved(ctx, &opts, agent, req.AllowedTools); err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	created, err := b.sessions.Create(ctx, opts)
	if err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	if created == nil || created.Info() == nil {
		return looppkg.ActionSessionBinding{}, errors.New("daemon: loop action session create returned nil")
	}
	return looppkg.ActionSessionBinding{
		WorkspaceID: req.WorkspaceID,
		LoopRunID:   req.LoopRunID,
		SessionID:   strings.TrimSpace(created.Info().ID),
		Handle:      strings.TrimSpace(req.Handle),
		Isolated:    req.Isolated,
	}, nil
}

func (b *loopActionSessionBinder) adoptOriginBinding(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
) (looppkg.ActionSessionBinding, error) {
	if req.TargetBindingEpoch != 1 {
		return looppkg.ActionSessionBinding{}, bindingMismatch("borrowed origin must be binding epoch 1")
	}
	sessionID := strings.TrimSpace(req.OriginSessionID)
	identity, err := b.creationStore.GetSessionCreationIdentity(ctx, sessionID)
	if err != nil {
		return looppkg.ActionSessionBinding{}, fmt.Errorf("daemon: load origin session creation identity: %w", err)
	}
	if pinned := strings.TrimSpace(req.PinnedCreationProfileRef); pinned != "" &&
		pinned != identity.CreationProfileRef {
		return looppkg.ActionSessionBinding{}, bindingMismatch("origin creation profile differs from pinned profile")
	}
	if pinned := strings.TrimSpace(req.PinnedCreationDigest); pinned != "" && pinned != identity.CreationDigest {
		return looppkg.ActionSessionBinding{}, bindingMismatch("origin creation digest differs from pinned identity")
	}
	if err := b.revalidatePersistedProfile(ctx, req, identity); err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	attemptID := strings.TrimSpace(req.BindingAttemptID)
	if attemptID == "" {
		attemptID = deterministicBindingValue("bind", req, 1)
	}
	binding, err := b.bindings.GetOrCreateSessionBinding(ctx, goalpkg.GetOrCreateBindingRequest{
		Key: key,
		CheckpointKey: goalpkg.TurnKey{
			WorkspaceID: req.WorkspaceID,
			LoopRunID:   req.LoopRunID,
			Generation:  req.Generation,
			NodeID:      req.NodeID,
			ItemIndex:   req.ItemIndex,
		},
		ExpectedControlEpoch:           req.ExpectedControlEpoch,
		ExpectedCheckpointPhase:        req.ExpectedCheckpointPhase,
		ExpectedTaskRunID:              req.ExpectedTaskRunID,
		ExpectedQueueEntryID:           req.ExpectedQueueEntryID,
		ExpectedPromptID:               req.ExpectedPromptID,
		ExpectedCheckpointBindingEpoch: req.ExpectedCheckpointBindingEpoch,
		ExpectedCheckpointSessionID:    req.ExpectedCheckpointSessionID,
		ExpectedCheckpointHandle:       req.ExpectedCheckpointHandle,
		BindingAttemptID:               attemptID,
		SessionID:                      sessionID,
		CreationProfileRef:             identity.CreationProfileRef,
		PolicySpecDigest:               identity.PolicySpecDigest,
		CreationDigest:                 identity.CreationDigest,
		Ownership:                      goalpkg.BindingOwnershipOriginBorrowed,
	})
	if err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	return actionBindingFromGoal(req, binding), nil
}

func (b *loopActionSessionBinder) ensureRunOwnedBinding(
	ctx context.Context,
	creator loopManagedSessionManager,
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
	active goalpkg.SessionBinding,
	activeFound bool,
) (looppkg.ActionSessionBinding, error) {
	profile, opts, err := b.resolveRunOwnedBindingProfile(ctx, req, active, activeFound)
	if err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	epoch := req.TargetBindingEpoch
	attemptID, sessionID := bindingAttemptIdentity(req, epoch)
	identity, err := bindingCreationIdentity(profile, opts, sessionID)
	if err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	prepared, err := b.prepareBindingAttempt(ctx, key, epoch, attemptID, sessionID, identity)
	if err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	opts.DesiredSessionID = prepared.SessionID
	opts.CreationProfile = cloneStoreCreationProfile(profile)
	opts.CreationIdentity = cloneStoreCreationIdentity(identity)
	_, createErr := creator.EnsureCreated(ctx, opts)
	if createErr != nil {
		if _, settleErr := b.bindings.SettleStoppedSessionBindingCreation(
			context.WithoutCancel(ctx),
			goalpkg.SettleStoppedBindingCreationRequest{
				Key: key, ExpectedBindingEpoch: prepared.BindingEpoch,
				SessionID: prepared.SessionID,
			},
		); settleErr != nil {
			return actionBindingFromGoal(req, prepared), errors.Join(createErr, settleErr)
		}
		binding := actionBindingFromGoal(req, prepared)
		if creationErr, ok := errors.AsType[*session.CreationError](createErr); ok {
			return binding, &looppkg.ActionSessionCreationError{
				EffectKnownFalse: creationErr.Effect == session.EffectKnownFalse,
				Code:             creationErr.Code,
				Err:              createErr,
			}
		}
		return binding, createErr
	}
	activated, stopped, err := b.bindings.FinalizeSessionBindingCreation(
		context.WithoutCancel(ctx),
		activationRequest(req, key, prepared),
	)
	if err != nil {
		return looppkg.ActionSessionBinding{}, err
	}
	if stopped {
		return actionBindingFromGoal(req, prepared), fmt.Errorf(
			"%w: Goal session creation completed after its Run was stopped",
			looppkg.ErrTransitionConflict,
		)
	}
	return actionBindingFromGoal(req, activated), nil
}

func (b *loopActionSessionBinder) AdvanceActionSessionRetry(
	ctx context.Context,
	req *looppkg.ActionSessionRetryRequest,
) error {
	if b == nil || b.bindings == nil || b.creationStore == nil {
		return errors.New("daemon: managed loop binding stores are unavailable")
	}
	failed := req.FailedBinding
	key := goalpkg.BindingKey{
		WorkspaceID: failed.WorkspaceID,
		LoopRunID:   failed.LoopRunID,
		Handle:      failed.Handle,
	}
	advance := goalpkg.AdvanceBindingCreationFailureRequest{
		Key: key,
		CheckpointKey: goalpkg.TurnKey{
			WorkspaceID: req.BindRequest.WorkspaceID,
			LoopRunID:   req.BindRequest.LoopRunID,
			Generation:  req.BindRequest.Generation,
			NodeID:      req.BindRequest.NodeID,
			ItemIndex:   req.BindRequest.ItemIndex,
		},
		ExpectedControlEpoch:           req.BindRequest.ExpectedControlEpoch,
		ExpectedCheckpointPhase:        req.BindRequest.ExpectedCheckpointPhase,
		ExpectedTaskRunID:              req.BindRequest.ExpectedTaskRunID,
		ExpectedQueueEntryID:           req.BindRequest.ExpectedQueueEntryID,
		ExpectedPromptID:               req.BindRequest.ExpectedPromptID,
		ExpectedCheckpointBindingEpoch: req.BindRequest.ExpectedCheckpointBindingEpoch,
		ExpectedCheckpointSessionID:    req.BindRequest.ExpectedCheckpointSessionID,
		ExpectedCheckpointHandle:       req.BindRequest.ExpectedCheckpointHandle,
		ExpectedPromptAttempt:          req.ExpectedPromptAttempt,
		ExpectedBindingEpoch:           failed.BindingEpoch,
		ExpectedBindingAttemptID:       failed.BindingAttemptID,
		ExpectedBindingSessionID:       failed.SessionID,
		ExpectedBindingProfileRef:      failed.CreationProfileRef,
		ExpectedBindingPolicyDigest:    failed.PolicySpecDigest,
		ExpectedBindingCreationDigest:  failed.CreationDigest,
		FailureCode:                    strings.TrimSpace(req.FailureCode),
		PrepareSuccessor:               req.RetryWithFreshSession,
	}
	if req.RetryWithFreshSession {
		active, activeFound, err := b.loadActiveBinding(ctx, key)
		if err != nil {
			return err
		}
		profile, opts, err := b.resolveRunOwnedBindingProfile(ctx, req.BindRequest, active, activeFound)
		if err != nil {
			return err
		}
		nextEpoch := failed.BindingEpoch + 1
		nextAttemptID, nextSessionID := bindingAttemptIdentity(req.BindRequest, nextEpoch)
		identity, err := bindingCreationIdentity(profile, opts, nextSessionID)
		if err != nil {
			return err
		}
		advance.SuccessorBindingEpoch = nextEpoch
		advance.SuccessorBindingAttemptID = nextAttemptID
		advance.SuccessorSessionID = nextSessionID
		advance.CreationProfileRef = identity.CreationProfileRef
		advance.PolicySpecDigest = identity.PolicySpecDigest
		advance.SuccessorCreationDigest = identity.CreationDigest
	}
	return b.bindings.AdvanceBindingCreationFailure(ctx, advance)
}

func (b *loopActionSessionBinder) resolveRunOwnedBindingProfile(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	active goalpkg.SessionBinding,
	activeFound bool,
) (store.SessionCreationProfile, session.CreateOpts, error) {
	profileRef := strings.TrimSpace(req.PinnedCreationProfileRef)
	if activeFound {
		profileRef = active.CreationProfileRef
	}
	profile, opts, err := b.resolveEffectiveCreationProfile(ctx, req, profileRef)
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, err
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return store.SessionCreationProfile{}, session.CreateOpts{}, err
	}
	if activeFound && (active.CreationProfileRef != profileRef || active.PolicySpecDigest != policyDigest) {
		return store.SessionCreationProfile{}, session.CreateOpts{}, bindingMismatch(
			"active binding policy/profile drifted",
		)
	}
	return profile, opts, nil
}

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

func activationRequest(
	req looppkg.ActionSessionBindRequest,
	key goalpkg.BindingKey,
	prepared goalpkg.SessionBinding,
) goalpkg.ActivateBindingRequest {
	checkpointKey := &goalpkg.TurnKey{
		WorkspaceID: req.WorkspaceID,
		LoopRunID:   req.LoopRunID,
		Generation:  req.Generation,
		NodeID:      req.NodeID,
		ItemIndex:   req.ItemIndex,
	}
	return goalpkg.ActivateBindingRequest{
		Key:                  key,
		CheckpointKey:        checkpointKey,
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
) error {
	identity := store.SessionCreationIdentity{
		CreationProfileRef: binding.CreationProfileRef,
		PolicySpecDigest:   binding.PolicySpecDigest,
		CreationDigest:     binding.CreationDigest,
	}
	return b.revalidatePersistedProfile(ctx, req, identity)
}

func (b *loopActionSessionBinder) revalidatePersistedProfile(
	ctx context.Context,
	req looppkg.ActionSessionBindRequest,
	identity store.SessionCreationIdentity,
) error {
	profile, _, err := b.resolveEffectiveCreationProfile(ctx, req, identity.CreationProfileRef)
	if err != nil {
		return err
	}
	profileRef, err := profile.Ref()
	if err != nil {
		return err
	}
	policyDigest, err := profile.PolicySpecDigest()
	if err != nil {
		return err
	}
	if profileRef != identity.CreationProfileRef || policyDigest != identity.PolicySpecDigest {
		return bindingMismatch("persisted creation profile no longer passes the effective policy gate")
	}
	return nil
}

func actionBindingFromGoal(
	req looppkg.ActionSessionBindRequest,
	binding goalpkg.SessionBinding,
) looppkg.ActionSessionBinding {
	return looppkg.ActionSessionBinding{
		WorkspaceID:        req.WorkspaceID,
		LoopRunID:          req.LoopRunID,
		SessionID:          binding.SessionID,
		Handle:             binding.Key.Handle,
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
	}
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
	return b.sessions.CancelPrompt(ctx, strings.TrimSpace(binding.SessionID))
}

func bindingMismatch(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeContinuousBindingMismatch,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, strings.TrimSpace(detail)),
	}
}
