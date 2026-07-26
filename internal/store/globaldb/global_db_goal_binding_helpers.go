package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func normalizeOriginBindingRequest(
	req goal.GetOrCreateBindingRequest,
	now time.Time,
) (goal.GetOrCreateBindingRequest, error) {
	if err := req.Key.Validate(); err != nil {
		return goal.GetOrCreateBindingRequest{}, err
	}
	if err := req.CheckpointKey.Validate(); err != nil {
		return goal.GetOrCreateBindingRequest{}, err
	}
	req = normalizeOriginBindingStrings(req)
	if err := validateOriginBindingRequest(req); err != nil {
		return goal.GetOrCreateBindingRequest{}, err
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	return req, nil
}

func normalizeOriginBindingStrings(req goal.GetOrCreateBindingRequest) goal.GetOrCreateBindingRequest {
	req.BindingAttemptID = strings.TrimSpace(req.BindingAttemptID)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.CreationProfileRef = strings.TrimSpace(req.CreationProfileRef)
	req.PolicySpecDigest = strings.TrimSpace(req.PolicySpecDigest)
	req.CreationDigest = strings.TrimSpace(req.CreationDigest)
	req.ExpectedCheckpointPhase = strings.TrimSpace(req.ExpectedCheckpointPhase)
	req.ExpectedTaskRunID = strings.TrimSpace(req.ExpectedTaskRunID)
	req.ExpectedQueueEntryID = strings.TrimSpace(req.ExpectedQueueEntryID)
	req.ExpectedPromptID = strings.TrimSpace(req.ExpectedPromptID)
	req.ExpectedCheckpointSessionID = strings.TrimSpace(req.ExpectedCheckpointSessionID)
	req.ExpectedCheckpointHandle = strings.TrimSpace(req.ExpectedCheckpointHandle)
	return req
}

func validateOriginBindingRequest(req goal.GetOrCreateBindingRequest) error {
	if req.Key.WorkspaceID != req.CheckpointKey.WorkspaceID ||
		req.Key.LoopRunID != req.CheckpointKey.LoopRunID {
		return fmt.Errorf(
			"%w: origin binding checkpoint does not own the binding Run",
			loop.ErrValidation,
		)
	}
	checkpointBindingEmpty := req.ExpectedCheckpointBindingEpoch == 0 &&
		req.ExpectedCheckpointSessionID == "" && req.ExpectedCheckpointHandle == ""
	checkpointBindingComplete := req.ExpectedCheckpointBindingEpoch > 0 &&
		req.ExpectedCheckpointSessionID != "" && req.ExpectedCheckpointHandle != ""
	if req.BindingAttemptID == "" || req.SessionID == "" || req.CreationProfileRef == "" ||
		req.PolicySpecDigest == "" || req.CreationDigest == "" || req.ExpectedControlEpoch < 1 ||
		req.CheckpointKey.Generation < 1 || !goalCheckpointPhaseValid(req.ExpectedCheckpointPhase) ||
		req.ExpectedCheckpointPhase == goalCheckpointPhaseTerminal ||
		req.ExpectedTaskRunID == "" || (!checkpointBindingEmpty && !checkpointBindingComplete) {
		return fmt.Errorf(
			"%w: origin binding identity fields are required",
			loop.ErrValidation,
		)
	}
	if req.Ownership != goal.BindingOwnershipOriginBorrowed {
		return fmt.Errorf(
			"%w: get-or-create requires origin-borrowed ownership",
			loop.ErrValidation,
		)
	}
	return nil
}

func normalizePrepareBindingRequest(
	req goal.PrepareBindingAttemptRequest,
	now time.Time,
) (goal.PrepareBindingAttemptRequest, error) {
	if err := req.Key.Validate(); err != nil {
		return goal.PrepareBindingAttemptRequest{}, err
	}
	req.BindingAttemptID = strings.TrimSpace(req.BindingAttemptID)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.CreationProfileRef = strings.TrimSpace(req.CreationProfileRef)
	req.PolicySpecDigest = strings.TrimSpace(req.PolicySpecDigest)
	req.CreationDigest = strings.TrimSpace(req.CreationDigest)
	if req.BindingEpoch < 1 || req.BindingAttemptID == "" || req.SessionID == "" ||
		req.CreationProfileRef == "" || req.PolicySpecDigest == "" || req.CreationDigest == "" {
		return goal.PrepareBindingAttemptRequest{}, fmt.Errorf(
			"%w: complete run-owned binding identity is required",
			loop.ErrValidation,
		)
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	return req, nil
}

func bindingOriginIdentityMatches(
	binding goal.SessionBinding,
	req goal.GetOrCreateBindingRequest,
) bool {
	return binding.BindingEpoch == 1 &&
		binding.Ownership == goal.BindingOwnershipOriginBorrowed &&
		binding.SessionID == req.SessionID &&
		binding.CreationProfileRef == req.CreationProfileRef &&
		binding.PolicySpecDigest == req.PolicySpecDigest &&
		binding.CreationDigest == req.CreationDigest
}

func bindingMatchesPrepareRequest(
	binding goal.SessionBinding,
	req goal.PrepareBindingAttemptRequest,
) bool {
	return binding.State == goal.BindingStateCreating &&
		binding.Ownership == goal.BindingOwnershipRunOwned &&
		binding.BindingAttemptID == req.BindingAttemptID &&
		binding.SessionID == req.SessionID &&
		binding.CreationProfileRef == req.CreationProfileRef &&
		binding.PolicySpecDigest == req.PolicySpecDigest &&
		binding.CreationDigest == req.CreationDigest
}

func validateBindingSessionIdentity(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID loop.WorkspaceID,
	sessionID string,
	profileRef string,
	policyDigest string,
	creationDigest string,
) error {
	storedWorkspace, _, identity, found, err := readSessionCreationIdentity(ctx, exec, sessionID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(storedWorkspace) != strings.TrimSpace(string(workspaceID)) ||
		identity.CreationProfileRef != strings.TrimSpace(profileRef) ||
		identity.PolicySpecDigest != strings.TrimSpace(policyDigest) ||
		identity.CreationDigest != strings.TrimSpace(creationDigest) {
		return sessionCreationIdentityMismatch(sessionID)
	}
	return nil
}

func getActiveSessionBindingWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.BindingKey,
) (goal.SessionBinding, bool, error) {
	row, err := sqlcgen.New(exec).GetActiveGoalSessionBinding(ctx, sqlcgen.GetActiveGoalSessionBindingParams{
		LoopRunID: string(key.LoopRunID), WorkspaceID: string(key.WorkspaceID), Handle: key.Handle,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goal.SessionBinding{}, false, nil
	}
	if err != nil {
		return goal.SessionBinding{}, false, err
	}
	return goalSessionBindingFromGenerated(row), true, nil
}

func findSessionBindingAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.BindingKey,
	epoch int64,
) (goal.SessionBinding, bool, error) {
	row, err := sqlcgen.New(exec).FindGoalSessionBindingAttempt(ctx, sqlcgen.FindGoalSessionBindingAttemptParams{
		LoopRunID: string(key.LoopRunID), WorkspaceID: string(key.WorkspaceID), Handle: key.Handle, BindingEpoch: epoch,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goal.SessionBinding{}, false, nil
	}
	if err != nil {
		return goal.SessionBinding{}, false, err
	}
	return goalSessionBindingFromGenerated(row), true, nil
}

func getSessionBindingAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.BindingKey,
	epoch int64,
) (goal.SessionBinding, error) {
	binding, found, err := findSessionBindingAttemptWithExecutor(ctx, exec, key, epoch)
	if err != nil {
		return goal.SessionBinding{}, err
	}
	if !found {
		return goal.SessionBinding{}, fmt.Errorf("%w: %s epoch %d", goal.ErrBindingNotFound, key.Handle, epoch)
	}
	return binding, nil
}

func requireGoalRowsAffected(result sql.Result, action string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: rows affected for %s: %w", action, err)
	}
	if affected != 1 {
		return goalControlStaleError(action + " lost compare-and-swap")
	}
	return nil
}

func requireGoalAffectedCount(affected int64, action string) error {
	if affected != 1 {
		return goalControlStaleError(action + " lost compare-and-swap")
	}
	return nil
}
