package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var (
	_ goal.BindingStore = (*GoalRepo)(nil)
)

// GetOrCreateSessionBinding atomically adopts an origin session or returns the compatible active epoch.
func (g *GoalRepo) GetOrCreateSessionBinding(
	ctx context.Context,
	req goal.GetOrCreateBindingRequest,
) (goal.SessionBinding, error) {
	if err := g.checkReady(ctx, "get or create goal session binding"); err != nil {
		return goal.SessionBinding{}, err
	}
	normalized, err := normalizeOriginBindingRequest(req, g.now())
	if err != nil {
		return goal.SessionBinding{}, err
	}
	var binding goal.SessionBinding
	err = g.withTaskImmediateTransaction(ctx, "get or create goal session binding", func(exec taskSQLExecutor) error {
		if err := validateBindingRunWorkspace(ctx, exec, normalized.Key); err != nil {
			return err
		}
		if err := validateOriginBindingAdoptionOwner(ctx, exec, normalized); err != nil {
			return err
		}
		active, found, err := getActiveSessionBindingWithExecutor(ctx, exec, normalized.Key)
		if err != nil {
			return err
		}
		if found {
			if !bindingOriginIdentityMatches(active, normalized) ||
				(active.AdoptedGeneration == normalized.CheckpointKey.Generation &&
					active.AdoptionAttemptID != normalized.BindingAttemptID) {
				return goalBindingMismatchError("active binding policy/profile or origin identity differs")
			}
			binding, err = advanceActiveOriginBindingAdoption(ctx, exec, active, normalized)
			return err
		}
		existing, found, err := findSessionBindingAttemptWithExecutor(ctx, exec, normalized.Key, 1)
		if err != nil {
			return err
		}
		if found {
			binding, err = reactivateClosedOriginBinding(ctx, exec, existing, normalized)
			return err
		}
		if err := validateBindingSessionIdentity(
			ctx,
			exec,
			normalized.Key.WorkspaceID,
			normalized.SessionID,
			normalized.CreationProfileRef,
			normalized.PolicySpecDigest,
			normalized.CreationDigest,
		); err != nil {
			return err
		}
		err = sqlcgen.New(exec).InsertOriginGoalSessionBinding(ctx, sqlcgen.InsertOriginGoalSessionBindingParams{
			LoopRunID:          string(normalized.Key.LoopRunID),
			Handle:             normalized.Key.Handle,
			BindingAttemptID:   normalized.BindingAttemptID,
			SessionID:          normalized.SessionID,
			WorkspaceID:        string(normalized.Key.WorkspaceID),
			CreationProfileRef: normalized.CreationProfileRef,
			PolicySpecDigest:   normalized.PolicySpecDigest,
			CreationDigest:     normalized.CreationDigest,
			CreatedAt: store.FormatTimestamp(
				normalized.CreatedAt,
			),
			ActivatedAt:       store.FormatTimestamp(normalized.CreatedAt),
			AdoptedGeneration: int64(normalized.CheckpointKey.Generation),
			AdoptionAttemptID: goalNullableString(normalized.BindingAttemptID),
		})
		if err != nil {
			return fmt.Errorf("store: insert origin goal binding: %w", err)
		}
		binding, err = getSessionBindingAttemptWithExecutor(ctx, exec, normalized.Key, 1)
		return err
	})
	if err != nil {
		return goal.SessionBinding{}, err
	}
	return binding, nil
}

func validateOriginBindingAdoptionOwner(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.GetOrCreateBindingRequest,
) error {
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.CheckpointKey)
	if err != nil {
		return err
	}
	if checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
		checkpoint.Phase != req.ExpectedCheckpointPhase ||
		checkpoint.TaskRunID != req.ExpectedTaskRunID ||
		checkpoint.QueueEntryID != req.ExpectedQueueEntryID ||
		checkpoint.PromptID != req.ExpectedPromptID ||
		checkpoint.BindingEpoch != req.ExpectedCheckpointBindingEpoch ||
		checkpoint.SessionID != req.ExpectedCheckpointSessionID ||
		checkpoint.BindingHandle != req.ExpectedCheckpointHandle {
		return goalControlStaleError("origin binding checkpoint owner changed")
	}
	owner, err := sqlcgen.New(exec).
		GetGoalOriginBindingAdoptionOwner(ctx, sqlcgen.GetGoalOriginBindingAdoptionOwnerParams{
			ID: string(req.Key.LoopRunID), WorkspaceID: string(req.Key.WorkspaceID),
		})
	if err != nil {
		return fmt.Errorf("store: load origin binding Run owner: %w", err)
	}
	if owner.Status != string(looppkg.StatusRunning) || int(owner.Generation) != req.CheckpointKey.Generation {
		return goalControlStaleError("origin binding Run is not live at the requested generation")
	}
	return nil
}

func advanceActiveOriginBindingAdoption(
	ctx context.Context,
	exec taskSQLExecutor,
	existing goal.SessionBinding,
	req goal.GetOrCreateBindingRequest,
) (goal.SessionBinding, error) {
	generation := req.CheckpointKey.Generation
	if existing.AdoptedGeneration > generation {
		return goal.SessionBinding{}, goalControlStaleError("origin binding adoption generation advanced")
	}
	if existing.AdoptedGeneration == generation {
		if existing.AdoptionAttemptID != req.BindingAttemptID {
			return goal.SessionBinding{}, goalControlStaleError("origin binding adoption attempt changed")
		}
		return existing, nil
	}
	affected, err := sqlcgen.New(exec).
		AdvanceGoalOriginBindingAdoption(ctx, sqlcgen.AdvanceGoalOriginBindingAdoptionParams{
			AdoptedGeneration: int64(generation), AdoptionAttemptID: goalNullableString(req.BindingAttemptID),
			LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle,
			ExpectedAdoptedGeneration: int64(existing.AdoptedGeneration),
		})
	if err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: advance origin binding adoption: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "advance origin binding adoption"); err != nil {
		return goal.SessionBinding{}, err
	}
	return getSessionBindingAttemptWithExecutor(ctx, exec, req.Key, 1)
}

func reactivateClosedOriginBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	existing goal.SessionBinding,
	req goal.GetOrCreateBindingRequest,
) (goal.SessionBinding, error) {
	if !bindingOriginIdentityMatches(existing, req) {
		return goal.SessionBinding{}, goalBindingMismatchError(
			"origin binding policy/profile or identity differs",
		)
	}
	if existing.State != goal.BindingStateClosed {
		return goal.SessionBinding{}, goalControlStaleError("origin binding is not reusable")
	}
	if existing.AdoptedGeneration >= req.CheckpointKey.Generation {
		return goal.SessionBinding{}, goalControlStaleError("closed origin binding requires a later generation")
	}
	if err := validateBindingSessionIdentity(
		ctx,
		exec,
		req.Key.WorkspaceID,
		req.SessionID,
		req.CreationProfileRef,
		req.PolicySpecDigest,
		req.CreationDigest,
	); err != nil {
		return goal.SessionBinding{}, err
	}
	affected, err := sqlcgen.New(exec).ReactivateGoalOriginBinding(ctx, sqlcgen.ReactivateGoalOriginBindingParams{
		AdoptedGeneration: int64(
			req.CheckpointKey.Generation,
		),
		AdoptionAttemptID:         goalNullableString(req.BindingAttemptID),
		LoopRunID:                 string(req.Key.LoopRunID),
		Handle:                    req.Key.Handle,
		ExpectedAdoptedGeneration: int64(existing.AdoptedGeneration),
	})
	if err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: reactivate origin goal binding: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "reactivate origin goal binding"); err != nil {
		return goal.SessionBinding{}, err
	}
	return getSessionBindingAttemptWithExecutor(ctx, exec, req.Key, 1)
}

// PrepareSessionBindingAttempt persists one run-owned creating epoch before session creation.
func (g *GoalRepo) PrepareSessionBindingAttempt(
	ctx context.Context,
	req goal.PrepareBindingAttemptRequest,
) (goal.SessionBinding, error) {
	if err := g.checkReady(ctx, "prepare goal session binding attempt"); err != nil {
		return goal.SessionBinding{}, err
	}
	normalized, err := normalizePrepareBindingRequest(req, g.now())
	if err != nil {
		return goal.SessionBinding{}, err
	}
	var prepared goal.SessionBinding
	err = g.withTaskImmediateTransaction(ctx, "prepare goal session binding attempt", func(exec taskSQLExecutor) error {
		var err error
		prepared, err = prepareSessionBindingAttemptWithExecutor(ctx, exec, normalized)
		return err
	})
	if err != nil {
		return goal.SessionBinding{}, err
	}
	return prepared, nil
}

func prepareSessionBindingAttemptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.PrepareBindingAttemptRequest,
) (goal.SessionBinding, error) {
	if err := validateBindingRunWorkspace(ctx, exec, req.Key); err != nil {
		return goal.SessionBinding{}, err
	}
	active, found, err := getActiveSessionBindingWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return goal.SessionBinding{}, err
	}
	if found && (active.PolicySpecDigest != req.PolicySpecDigest ||
		active.CreationProfileRef != req.CreationProfileRef) {
		return goal.SessionBinding{}, goalBindingMismatchError(
			"successor policy/profile differs from active binding",
		)
	}
	existing, found, err := findSessionBindingAttemptWithExecutor(ctx, exec, req.Key, req.BindingEpoch)
	if err != nil {
		return goal.SessionBinding{}, err
	}
	if found {
		if !bindingMatchesPrepareRequest(existing, req) {
			return goal.SessionBinding{}, goalBindingMismatchError(
				"binding epoch already has different creation identity",
			)
		}
		return existing, nil
	}
	if err := validateNextBindingAttemptEpoch(ctx, exec, req); err != nil {
		return goal.SessionBinding{}, err
	}
	if err := sqlcgen.New(exec).InsertGoalSessionBindingAttempt(ctx, sqlcgen.InsertGoalSessionBindingAttemptParams{
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle, BindingEpoch: req.BindingEpoch,
		BindingAttemptID: req.BindingAttemptID, SessionID: req.SessionID, WorkspaceID: string(req.Key.WorkspaceID),
		CreationProfileRef: req.CreationProfileRef, PolicySpecDigest: req.PolicySpecDigest,
		CreationDigest: req.CreationDigest, CreatedAt: store.FormatTimestamp(req.CreatedAt),
	}); err != nil {
		return goal.SessionBinding{}, fmt.Errorf("store: insert creating goal binding: %w", err)
	}
	return getSessionBindingAttemptWithExecutor(ctx, exec, req.Key, req.BindingEpoch)
}

func validateNextBindingAttemptEpoch(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.PrepareBindingAttemptRequest,
) error {
	maximumEpoch, err := sqlcgen.New(exec).GetMaxGoalBindingEpoch(ctx, sqlcgen.GetMaxGoalBindingEpochParams{
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle,
	})
	if err != nil {
		return fmt.Errorf("store: load maximum goal binding epoch: %w", err)
	}
	if req.BindingEpoch != maximumEpoch+1 {
		return goalControlStaleError("binding attempt must use the next consecutive epoch")
	}
	creatingCount, err := sqlcgen.New(exec).CountCreatingGoalBindings(ctx, sqlcgen.CountCreatingGoalBindingsParams{
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle,
	})
	if err != nil {
		return fmt.Errorf("store: count creating goal bindings: %w", err)
	}
	if creatingCount != 0 {
		return goalControlStaleError("another binding creation attempt is already pending")
	}
	return nil
}
