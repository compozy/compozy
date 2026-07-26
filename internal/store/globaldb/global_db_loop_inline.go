package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ looppkg.InlineRunStore = (*LoopRepo)(nil)
var _ looppkg.InlineGoalClearStore = (*LoopRepo)(nil)

// CreateInlineLoopRunForStart atomically validates the origin, starts the Run, and enqueues discovery.
func (g *LoopRepo) CreateInlineLoopRunForStart(
	ctx context.Context,
	run looppkg.Run,
) (looppkg.Run, error) {
	if err := g.checkReady(ctx, "create inline Loop Run"); err != nil {
		return looppkg.Run{}, err
	}
	normalized, inputsJSON, metadataJSON, err := normalizeInlineLoopRun(run)
	if err != nil {
		return looppkg.Run{}, err
	}
	err = g.withTaskImmediateTransaction(ctx, "create inline Loop Run", func(exec taskSQLExecutor) error {
		if err := validateInlineRunOrigin(ctx, exec, normalized); err != nil {
			return err
		}
		active, err := findLiveSessionGoalWithExecutor(
			ctx,
			exec,
			normalized.WorkspaceID,
			normalized.Origin.SessionID,
		)
		if err != nil {
			return err
		}
		if active != nil {
			return goalReplaceRequiredError(*active)
		}
		if err := g.persistStartedLoopRunWithExecutor(
			ctx,
			exec,
			normalized,
			inputsJSON,
			metadataJSON,
		); err != nil {
			return mapInlineStartPersistError(ctx, exec, normalized, err)
		}
		boundSessionID := normalized.Origin.SessionID
		return enqueueGoalProjectionOutboxForSessionOrigin(
			ctx,
			exec,
			normalized.Origin.SessionID,
			normalized.WorkspaceID,
			normalized.ID,
			&boundSessionID,
			goal.SessionOutboxCauseStart,
			string(normalized.ID),
			normalized.CreatedAt,
		)
	})
	if err != nil {
		return looppkg.Run{}, err
	}
	return normalized, nil
}

// ReplaceInlineLoopRun atomically revokes the expected live Goal and starts its prepared successor.
func (g *LoopRepo) ReplaceInlineLoopRun(
	ctx context.Context,
	request looppkg.InlineReplaceStoreRequest,
) (looppkg.InlineReplaceStoreResult, error) {
	if err := g.checkReady(ctx, "replace inline Loop Run"); err != nil {
		return looppkg.InlineReplaceStoreResult{}, err
	}
	if err := validateInlineReplaceRequest(request); err != nil {
		return looppkg.InlineReplaceStoreResult{}, err
	}
	normalized, inputsJSON, metadataJSON, err := normalizeInlineLoopRun(*request.Run)
	if err != nil {
		return looppkg.InlineReplaceStoreResult{}, err
	}
	request.Run = &normalized
	request.ReplacedAt = request.ReplacedAt.UTC()
	return g.replaceInlineLoopRunTransaction(ctx, request, inputsJSON, metadataJSON)
}

func (g *LoopRepo) replaceInlineLoopRunTransaction(
	ctx context.Context,
	request looppkg.InlineReplaceStoreRequest,
	inputsJSON []byte,
	metadataJSON []byte,
) (looppkg.InlineReplaceStoreResult, error) {
	run := *request.Run
	result := newInlineReplaceStoreResult(request.ExpectedRunID)
	err := g.withTaskImmediateTransaction(ctx, "replace inline Loop Run", func(exec taskSQLExecutor) error {
		if err := validateInlineRunOrigin(ctx, exec, run); err != nil {
			return err
		}
		current, err := findLiveSessionGoalWithExecutor(
			ctx,
			exec,
			run.WorkspaceID,
			run.Origin.SessionID,
		)
		if err != nil {
			return err
		}
		if current == nil || current.ID != request.ExpectedRunID {
			return goalReplaceStaleError(current)
		}
		stopRequest := looppkg.GoalRunStopRequest{
			WorkspaceID: run.WorkspaceID,
			RunID:       current.ID, ExpectedStatus: current.Status,
			Actor: request.Actor, StoppedAt: request.ReplacedAt,
		}
		leases, err := stopGoalCheckpointsWithExecutor(ctx, exec, stopRequest, false)
		if err != nil {
			return err
		}
		if err := closeStoppedGoalBindings(ctx, exec, stopRequest); err != nil {
			return err
		}
		if err := compareAndSwapLoopRunStatusWithExecutor(
			ctx,
			exec,
			current.ID,
			current.Status,
			looppkg.StatusFailed,
			looppkg.TransitionCauseGoalReplace,
			request.ReplacedAt,
		); err != nil {
			return err
		}
		if err := g.persistStartedLoopRunWithExecutor(
			ctx,
			exec,
			run,
			inputsJSON,
			metadataJSON,
		); err != nil {
			return err
		}
		boundSessionID := run.Origin.SessionID
		if err := enqueueGoalProjectionOutboxForSessionOrigin(
			ctx,
			exec,
			run.Origin.SessionID,
			run.WorkspaceID,
			run.ID,
			&boundSessionID,
			goal.SessionOutboxCauseReplace,
			string(run.ID),
			request.ReplacedAt,
		); err != nil {
			return err
		}
		result.ReplacedRun = *current
		result.ReplacedRun.Status = looppkg.StatusFailed
		result.Run = run
		result.RevokedPromptLeases = leases
		return nil
	})
	if err != nil {
		return looppkg.InlineReplaceStoreResult{}, err
	}
	return result, nil
}

func newInlineReplaceStoreResult(expectedRunID looppkg.RunID) looppkg.InlineReplaceStoreResult {
	return looppkg.InlineReplaceStoreResult{
		ReplacedRunID:       expectedRunID,
		RevokedPromptLeases: make([]looppkg.GoalPromptLease, 0),
	}
}

// ClearInlineGoal atomically selects and tombstones the newest session Goal, stopping it when live.
func (g *LoopRepo) ClearInlineGoal(
	ctx context.Context,
	request looppkg.InlineGoalClearStoreRequest,
) (looppkg.InlineGoalClearStoreResult, error) {
	if err := g.checkReady(ctx, "clear inline Goal"); err != nil {
		return looppkg.InlineGoalClearStoreResult{}, err
	}
	request.WorkspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(request.WorkspaceID)))
	request.OriginSessionID = strings.TrimSpace(request.OriginSessionID)
	request.ClearedAt = request.ClearedAt.UTC()
	if err := validateInlineGoalClearRequest(request); err != nil {
		return looppkg.InlineGoalClearStoreResult{}, err
	}
	result := looppkg.InlineGoalClearStoreResult{RevokedPromptLeases: make([]looppkg.GoalPromptLease, 0)}
	err := g.withTaskImmediateTransaction(ctx, "clear inline Goal", func(exec taskSQLExecutor) error {
		run, alreadyCleared, err := findNewestSessionGoalWithExecutor(
			ctx,
			exec,
			request.WorkspaceID,
			request.OriginSessionID,
		)
		if err != nil {
			return err
		}
		if run == nil || alreadyCleared {
			return goalNotActiveError("session has no visible Goal")
		}
		result.Run = *run
		if run.Status.Live() {
			stopRequest := looppkg.GoalRunStopRequest{
				WorkspaceID:    request.WorkspaceID,
				RunID:          run.ID,
				ExpectedStatus: run.Status,
				Actor:          request.Actor,
				StoppedAt:      request.ClearedAt,
			}
			leases, err := stopGoalCheckpointsWithExecutor(ctx, exec, stopRequest, false)
			if err != nil {
				return err
			}
			if err := closeStoppedGoalBindings(ctx, exec, stopRequest); err != nil {
				return err
			}
			if err := compareAndSwapLoopRunStatusWithExecutor(
				ctx,
				exec,
				run.ID,
				run.Status,
				looppkg.StatusFailed,
				looppkg.TransitionCauseGoalClear,
				request.ClearedAt,
			); err != nil {
				return err
			}
			result.Run.Status = looppkg.StatusFailed
			result.Terminalized = true
			result.RevokedPromptLeases = leases
		}
		if err := markInlineGoalRunCleared(ctx, exec, run.ID, request.WorkspaceID, request.ClearedAt); err != nil {
			return err
		}
		return enqueueGoalProjectionOutboxForSessionOrigin(
			ctx,
			exec,
			request.OriginSessionID,
			request.WorkspaceID,
			run.ID,
			nil,
			goal.SessionOutboxCauseClear,
			string(run.ID),
			request.ClearedAt,
		)
	})
	if err != nil {
		return looppkg.InlineGoalClearStoreResult{}, err
	}
	return result, nil
}

func validateInlineGoalClearRequest(request looppkg.InlineGoalClearStoreRequest) error {
	if request.WorkspaceID == "" || request.OriginSessionID == "" || request.ClearedAt.IsZero() {
		return fmt.Errorf("%w: inline Goal clear identity is incomplete", looppkg.ErrValidation)
	}
	if err := request.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: inline Goal clear actor: %w", looppkg.ErrValidation, err)
	}
	return nil
}

func findNewestSessionGoalWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (*looppkg.Run, bool, error) {
	row, err := sqlcgen.New(exec).FindNewestSessionGoal(ctx, sqlcgen.FindNewestSessionGoalParams{
		WorkspaceID: string(workspaceID), OriginSessionID: nullString(strings.TrimSpace(sessionID)),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("store: find newest session Goal: %w", err)
	}
	run, err := getLoopRunByIDWithExecutor(ctx, exec, looppkg.RunID(strings.TrimSpace(row.ID)))
	if err != nil {
		return nil, false, err
	}
	return &run, row.GoalClearedAt.Valid, nil
}

func markInlineGoalRunCleared(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	workspaceID looppkg.WorkspaceID,
	clearedAt time.Time,
) error {
	affected, err := sqlcgen.New(exec).MarkInlineGoalRunCleared(ctx, sqlcgen.MarkInlineGoalRunClearedParams{
		GoalClearedAt: nullableLoopTime(clearedAt), ID: string(runID), WorkspaceID: string(workspaceID),
	})
	if err != nil {
		return fmt.Errorf("store: mark inline Goal cleared: %w", err)
	}
	if affected != 1 {
		return goalControlStaleError("mark inline Goal cleared lost compare-and-swap")
	}
	return nil
}

func normalizeInlineLoopRun(run looppkg.Run) (looppkg.Run, []byte, []byte, error) {
	normalized, err := normalizeLoopRunForCreate(run)
	if err != nil {
		return looppkg.Run{}, nil, nil, err
	}
	if normalized.LoopName != looppkg.InlineGoalLoopName || normalized.Status != looppkg.StatusRunning ||
		normalized.Origin.Kind != looppkg.RunOriginSession {
		return looppkg.Run{}, nil, nil, fmt.Errorf(
			"%w: inline Goal Run identity is invalid",
			looppkg.ErrValidation,
		)
	}
	inputsJSON, metadataJSON, err := marshalLoopRunCreatePayload(normalized)
	if err != nil {
		return looppkg.Run{}, nil, nil, err
	}
	return normalized, inputsJSON, metadataJSON, nil
}

func validateInlineRunOrigin(ctx context.Context, exec taskSQLExecutor, run looppkg.Run) error {
	row, err := sqlcgen.New(exec).GetInlineGoalOriginSession(ctx, run.Origin.SessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return inlineOriginReasonError(looppkg.ReasonCodeGoalOriginInvalid, store.ErrSessionNotFound)
	}
	if err != nil {
		return fmt.Errorf("store: load inline Goal origin session: %w", err)
	}
	if strings.TrimSpace(row.WorkspaceID) != strings.TrimSpace(string(run.WorkspaceID)) {
		return inlineOriginReasonError(looppkg.ReasonCodeGoalOriginWorkspaceMismatch, looppkg.ErrValidation)
	}
	if strings.TrimSpace(row.State) != globalDBSessionStateActive {
		return inlineOriginReasonError(looppkg.ReasonCodeGoalOriginInvalid, looppkg.ErrInvalidTransition)
	}
	if !row.CreationProfileRef.Valid || !row.PolicySpecDigest.Valid || !row.CreationDigest.Valid ||
		strings.TrimSpace(row.CreationProfileRef.String) != run.Origin.CreationProfileRef ||
		strings.TrimSpace(row.PolicySpecDigest.String) != run.Origin.PolicySpecDigest ||
		strings.TrimSpace(row.CreationDigest.String) != run.Origin.CreationDigest {
		return inlineOriginReasonError(looppkg.ReasonCodeGoalOriginProfileUnavailable, looppkg.ErrValidation)
	}
	return nil
}

func findLiveSessionGoalWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (*looppkg.Run, error) {
	row, err := sqlcgen.New(exec).FindLiveSessionGoal(ctx, sqlcgen.FindLiveSessionGoalParams{
		WorkspaceID: string(workspaceID), OriginSessionID: nullString(strings.TrimSpace(sessionID)),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: find live session Goal: %w", err)
	}
	run, err := loopRunFromGenerated(&row)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func validateInlineReplaceRequest(request looppkg.InlineReplaceStoreRequest) error {
	if request.Run == nil || strings.TrimSpace(string(request.ExpectedRunID)) == "" || request.ReplacedAt.IsZero() {
		return fmt.Errorf("%w: inline replacement identity is incomplete", looppkg.ErrValidation)
	}
	if err := request.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: inline replacement actor: %w", looppkg.ErrValidation, err)
	}
	return nil
}

func goalReplaceRequiredError(active looppkg.Run) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeGoalReplaceRequired,
		Err:  looppkg.ErrConcurrencyConflict,
		Meta: map[string]string{"active_run_id": string(active.ID)},
	}
}

func mapInlineStartPersistError(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	cause error,
) error {
	if !isSQLiteUniqueConstraint(cause) {
		return cause
	}
	active, err := findLiveSessionGoalWithExecutor(
		ctx,
		exec,
		run.WorkspaceID,
		run.Origin.SessionID,
	)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("store: classify inline Goal uniqueness conflict: %w", err))
	}
	if active == nil {
		return &looppkg.ReasonError{
			Code: looppkg.ReasonCodeGoalReplaceRequired,
			Err:  looppkg.ErrConcurrencyConflict,
		}
	}
	return goalReplaceRequiredError(*active)
}

func goalReplaceStaleError(current *looppkg.Run) error {
	meta := map[string]string{}
	if current != nil {
		meta["active_run_id"] = string(current.ID)
	}
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeGoalReplaceStale,
		Err:  looppkg.ErrTransitionConflict,
		Meta: meta,
	}
}

func inlineOriginReasonError(code looppkg.ReasonCode, cause error) error {
	return &looppkg.ReasonError{Code: code, Err: cause}
}
