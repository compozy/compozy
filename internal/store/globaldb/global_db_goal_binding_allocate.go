package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// AllocateSessionBindingAttempt atomically selects and persists the next Goal-owned binding epoch.
func (g *GoalRepo) AllocateSessionBindingAttempt(
	ctx context.Context,
	req goal.AllocateBindingAttemptRequest,
) (goal.SessionBinding, error) {
	if err := g.checkReady(ctx, "allocate goal session binding attempt"); err != nil {
		return goal.SessionBinding{}, err
	}
	normalized, err := normalizeAllocateBindingRequest(req, g.now())
	if err != nil {
		return goal.SessionBinding{}, err
	}
	var prepared goal.SessionBinding
	err = g.withTaskImmediateTransaction(
		ctx,
		"allocate goal session binding attempt",
		func(exec taskSQLExecutor) error {
			var allocateErr error
			prepared, allocateErr = allocateSessionBindingAttemptWithExecutor(ctx, exec, normalized)
			return allocateErr
		},
	)
	if err != nil {
		return goal.SessionBinding{}, err
	}
	return prepared, nil
}

func normalizeAllocateBindingRequest(
	req goal.AllocateBindingAttemptRequest,
	now time.Time,
) (goal.AllocateBindingAttemptRequest, error) {
	if err := req.Key.Validate(); err != nil {
		return goal.AllocateBindingAttemptRequest{}, err
	}
	if err := req.CheckpointKey.Validate(); err != nil {
		return goal.AllocateBindingAttemptRequest{}, err
	}
	req.ExpectedCheckpointPhase = strings.TrimSpace(req.ExpectedCheckpointPhase)
	req.ExpectedTaskRunID = strings.TrimSpace(req.ExpectedTaskRunID)
	req.ExpectedQueueEntryID = strings.TrimSpace(req.ExpectedQueueEntryID)
	req.ExpectedPromptID = strings.TrimSpace(req.ExpectedPromptID)
	req.ExpectedCheckpointSessionID = strings.TrimSpace(req.ExpectedCheckpointSessionID)
	req.ExpectedCheckpointHandle = strings.TrimSpace(req.ExpectedCheckpointHandle)
	req.IdentityHandle = strings.TrimSpace(req.IdentityHandle)
	req.CreationProfile = store.NormalizeSessionCreationProfile(req.CreationProfile)
	req.CreationOptions.SessionID = strings.TrimSpace(req.CreationOptions.SessionID)
	if err := validateAllocateBindingRequest(req); err != nil {
		return goal.AllocateBindingAttemptRequest{}, err
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	return req, nil
}

func validateAllocateBindingRequest(req goal.AllocateBindingAttemptRequest) error {
	checkpointBindingEmpty := req.ExpectedCheckpointBindingEpoch == 0 &&
		req.ExpectedCheckpointSessionID == "" && req.ExpectedCheckpointHandle == ""
	checkpointBindingComplete := req.ExpectedCheckpointBindingEpoch > 0 &&
		req.ExpectedCheckpointSessionID != "" && req.ExpectedCheckpointHandle != ""
	if req.Key.WorkspaceID != req.CheckpointKey.WorkspaceID ||
		req.Key.LoopRunID != req.CheckpointKey.LoopRunID || req.ExpectedControlEpoch < 1 ||
		!goalCheckpointPhaseValid(req.ExpectedCheckpointPhase) ||
		req.ExpectedCheckpointPhase == goalCheckpointPhaseTerminal || req.ExpectedTaskRunID == "" ||
		req.ExpectedCheckpointBindingEpoch < 0 || (!checkpointBindingEmpty && !checkpointBindingComplete) ||
		req.IdentityHandle == "" || req.CreationOptions.SessionID != "" {
		return fmt.Errorf("%w: complete Goal binding allocation fence is required", looppkg.ErrValidation)
	}
	if err := req.CreationProfile.Validate(); err != nil {
		return err
	}
	if req.CreationProfile.WorkspaceID != string(req.Key.WorkspaceID) {
		return fmt.Errorf("%w: binding profile workspace differs from the Goal Run", looppkg.ErrValidation)
	}
	return nil
}

func allocateSessionBindingAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AllocateBindingAttemptRequest,
) (goal.SessionBinding, error) {
	if err := validateBindingRunWorkspace(ctx, exec, req.Key); err != nil {
		return goal.SessionBinding{}, err
	}
	if err := validateBindingAllocationOwner(ctx, exec, req); err != nil {
		return goal.SessionBinding{}, err
	}
	profileRef, err := req.CreationProfile.Ref()
	if err != nil {
		return goal.SessionBinding{}, err
	}
	policyDigest, err := req.CreationProfile.PolicySpecDigest()
	if err != nil {
		return goal.SessionBinding{}, err
	}
	active, found, err := getActiveSessionBindingWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return goal.SessionBinding{}, err
	}
	if found {
		if active.CreationProfileRef != profileRef || active.PolicySpecDigest != policyDigest {
			return goal.SessionBinding{}, goalBindingMismatchError("active binding policy/profile differs")
		}
		return active, nil
	}
	return allocateNextSessionBindingAttempt(ctx, exec, req, profileRef, policyDigest)
}

func validateBindingAllocationOwner(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AllocateBindingAttemptRequest,
) error {
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.CheckpointKey)
	if err != nil {
		return err
	}
	if checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
		checkpoint.Phase != req.ExpectedCheckpointPhase || checkpoint.TaskRunID != req.ExpectedTaskRunID ||
		checkpoint.QueueEntryID != req.ExpectedQueueEntryID || checkpoint.PromptID != req.ExpectedPromptID ||
		checkpoint.BindingEpoch != req.ExpectedCheckpointBindingEpoch ||
		checkpoint.SessionID != req.ExpectedCheckpointSessionID ||
		checkpoint.BindingHandle != req.ExpectedCheckpointHandle {
		return goalControlStaleError("binding allocation checkpoint owner changed")
	}
	owner, err := sqlcgen.New(exec).GetGoalOriginBindingAdoptionOwner(
		ctx,
		sqlcgen.GetGoalOriginBindingAdoptionOwnerParams{
			ID: string(req.Key.LoopRunID), WorkspaceID: string(req.Key.WorkspaceID),
		},
	)
	if err != nil {
		return fmt.Errorf("store: load Goal binding allocation Run owner: %w", err)
	}
	if owner.Status != string(looppkg.StatusRunning) || int(owner.Generation) != req.CheckpointKey.Generation {
		return goalControlStaleError("binding allocation Run is not live at the requested generation")
	}
	return nil
}

func allocateNextSessionBindingAttempt(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AllocateBindingAttemptRequest,
	profileRef string,
	policyDigest string,
) (goal.SessionBinding, error) {
	maximumEpoch, err := sqlcgen.New(exec).GetMaxGoalBindingEpoch(ctx, sqlcgen.GetMaxGoalBindingEpochParams{
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle,
	})
	if err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: load maximum goal binding epoch: %w", err)
	}
	if maximumEpoch > 0 {
		existing, found, findErr := findSessionBindingAttemptWithExecutor(ctx, exec, req.Key, maximumEpoch)
		if findErr != nil {
			return goal.SessionBinding{}, findErr
		}
		if found {
			matches, matchErr := allocatedBindingMatchesRequest(existing, req, profileRef, policyDigest)
			if matchErr != nil {
				return goal.SessionBinding{}, matchErr
			}
			if matches {
				return existing, nil
			}
		}
	}
	creatingCount, err := sqlcgen.New(exec).CountCreatingGoalBindings(ctx, sqlcgen.CountCreatingGoalBindingsParams{
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle,
	})
	if err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: count creating goal bindings: %w", err)
	}
	if creatingCount != 0 {
		return goal.SessionBinding{}, goalControlStaleError("another binding creation attempt is already pending")
	}
	epoch := maximumEpoch + 1
	attemptID, sessionID := goal.DeriveBindingIdentity(req.CheckpointKey, req.IdentityHandle, epoch)
	creationOptions := req.CreationOptions
	creationOptions.SessionID = sessionID
	creationDigest, err := req.CreationProfile.CreationDigest(creationOptions)
	if err != nil {
		return goal.SessionBinding{}, err
	}
	prepare := goal.PrepareBindingAttemptRequest{
		Key: req.Key, BindingEpoch: epoch, BindingAttemptID: attemptID, SessionID: sessionID,
		CreationProfileRef: profileRef, PolicySpecDigest: policyDigest, CreationDigest: creationDigest,
		CreatedAt: req.CreatedAt,
	}
	if err := sqlcgen.New(exec).InsertGoalSessionBindingAttempt(ctx, sqlcgen.InsertGoalSessionBindingAttemptParams{
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle, BindingEpoch: epoch,
		BindingAttemptID: attemptID, SessionID: sessionID, WorkspaceID: string(req.Key.WorkspaceID),
		CreationProfileRef: profileRef, PolicySpecDigest: policyDigest, CreationDigest: creationDigest,
		CreatedAt: store.FormatTimestamp(req.CreatedAt),
	}); err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: insert allocated goal binding: %w", err)
	}
	return getSessionBindingAttemptWithExecutor(ctx, exec, prepare.Key, prepare.BindingEpoch)
}

func allocatedBindingMatchesRequest(
	binding goal.SessionBinding,
	req goal.AllocateBindingAttemptRequest,
	profileRef string,
	policyDigest string,
) (bool, error) {
	attemptID, sessionID := goal.DeriveBindingIdentity(
		req.CheckpointKey,
		req.IdentityHandle,
		binding.BindingEpoch,
	)
	options := req.CreationOptions
	options.SessionID = sessionID
	creationDigest, err := req.CreationProfile.CreationDigest(options)
	if err != nil {
		return false, err
	}
	return binding.State == goal.BindingStateCreating && binding.Ownership == goal.BindingOwnershipRunOwned &&
		binding.BindingAttemptID == attemptID && binding.SessionID == sessionID &&
		binding.CreationProfileRef == profileRef && binding.PolicySpecDigest == policyDigest &&
		binding.CreationDigest == creationDigest, nil
}
