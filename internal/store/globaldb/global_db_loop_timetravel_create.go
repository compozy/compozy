package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func (g *LoopRepo) createRerunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.RerunStoreRequest,
	result *looppkg.RerunResult,
	replayed *bool,
) error {
	prior, found, err := getTimeTravelReplay(ctx, exec, request.WorkspaceID, request.IdempotencyKey)
	if err != nil {
		return err
	}
	if found {
		if prior.digest != request.RequestDigest || prior.kind != loopTimeTravelKindRerun {
			return timeTravelKeyReuseError(request.IdempotencyKey)
		}
		*result = looppkg.RerunResult{
			RunID: prior.resultRunID, Generation: valueOrZero(prior.resultGeneration),
			ParentGeneration: valueOrZero(prior.sourceGeneration), Replayed: true,
		}
		*replayed = true
		return nil
	}
	current, err := getLoopRunByIDWithExecutor(ctx, exec, request.Source.ID)
	if err != nil {
		return err
	}
	if current.WorkspaceID != request.WorkspaceID || current.Generation != request.Source.Generation ||
		!current.Status.Terminal() {
		return rerunBusyError()
	}
	if err := insertRerunGeneration(ctx, exec, current, request); err != nil {
		return err
	}
	if err := g.restartRerunCoordinator(ctx, exec, &current, request); err != nil {
		return err
	}
	if err := appendLoopRunStatusEvent(
		ctx, exec, current.ID, current.WorkspaceID, request.Source.Status, looppkg.StatusRunning,
		looppkg.TransitionCauseOperatorRerun, request.At,
	); err != nil {
		return err
	}
	if err := insertTimeTravelOp(ctx, exec, request.WorkspaceID, request.Operation); err != nil {
		return err
	}
	*result = looppkg.RerunResult{
		RunID: current.ID, Generation: request.Intent.Generation,
		ParentGeneration: request.Intent.ParentGeneration,
	}
	return nil
}

func insertRerunGeneration(
	ctx context.Context,
	exec taskSQLExecutor,
	current looppkg.Run,
	request looppkg.RerunStoreRequest,
) error {
	if err := insertLoopGenerationWithExecutor(ctx, exec, current.ID, request.Intent, request.At); err != nil {
		return err
	}
	if err := insertTimeTravelOutputs(ctx, exec, current.ID, request.NextOutputs); err != nil {
		return err
	}
	affected, err := exec.ExecContext(ctx, `UPDATE loop_runs SET
		status = 'running', completion_state = 'complete', pause_requested = 0,
		active_gate_id = '', active_human_criteria_json = '[]',
		generation = ?, last_progress_at = ?, completed_at = NULL
		WHERE id = ? AND workspace_id = ? AND generation = ? AND status = ?`,
		request.Intent.Generation, request.At.UTC(), current.ID, current.WorkspaceID,
		current.Generation, current.Status)
	if err != nil {
		return fmt.Errorf("store: reactivate rerun %q: %w", current.ID, err)
	}
	rows, err := affected.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: inspect rerun reactivation: %w", err)
	}
	if rows != 1 {
		return rerunBusyError()
	}
	return nil
}

func (g *LoopRepo) restartRerunCoordinator(
	ctx context.Context,
	exec taskSQLExecutor,
	current *looppkg.Run,
	request looppkg.RerunStoreRequest,
) error {
	current.Status = looppkg.StatusRunning
	current.Generation = int(request.Intent.Generation)
	current.PauseRequested = false
	if err := g.repairLoopCoordinatorTaskWithExecutor(
		ctx, exec, *current, loopCoordinatorTaskID(current.ID), request.At,
	); err != nil && !errorsIsTaskNotFound(err) {
		return err
	}
	generation := int(request.Intent.Generation)
	_, _, err := g.reserveLoopCoordinatorRunWithExecutor(
		ctx, exec, *current, loopCoordinatorStartOrigin(), request.At,
		loopCoordinatorRunID(current.ID, generation), loopCoordinatorIdempotencyKey(current.ID, generation),
	)
	return err
}

func rerunBusyError() error {
	return &looppkg.ReasonError{Code: looppkg.ReasonCodeRerunBusy, Err: looppkg.ErrRerunBusy}
}

func (g *LoopRepo) createForkWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.ForkStoreRequest,
	created *looppkg.Run,
	replayed *bool,
) error {
	prior, found, err := getTimeTravelReplay(ctx, exec, request.Source.WorkspaceID, request.IdempotencyKey)
	if err != nil {
		return err
	}
	if found {
		return replayFork(ctx, exec, request, prior, created, replayed)
	}
	source, err := getLoopRunByIDWithExecutor(ctx, exec, request.Source.ID)
	if err != nil {
		return err
	}
	if source.WorkspaceID != request.Source.WorkspaceID {
		return looppkg.ErrRunNotFound
	}
	candidate, err := g.insertForkCandidate(ctx, exec, request)
	if err != nil {
		return err
	}
	if err := g.initializeForkGenerations(ctx, exec, candidate, request); err != nil {
		return err
	}
	if err := g.finishFork(ctx, exec, source, candidate, request); err != nil {
		return err
	}
	*created = candidate
	return nil
}

func replayFork(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.ForkStoreRequest,
	prior storedTimeTravelReplay,
	created *looppkg.Run,
	replayed *bool,
) error {
	if prior.digest != request.RequestDigest || prior.kind != "fork" {
		return timeTravelKeyReuseError(request.IdempotencyKey)
	}
	run, err := getLoopRunByIDWithExecutor(ctx, exec, prior.resultRunID)
	if err != nil {
		return err
	}
	*created, *replayed = run, true
	return nil
}

func (g *LoopRepo) insertForkCandidate(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.ForkStoreRequest,
) (looppkg.Run, error) {
	candidate, err := applyLoopStartConcurrencyPolicy(ctx, exec, *request.Child, request.Concurrency)
	if err != nil {
		return looppkg.Run{}, err
	}
	inputsJSON, metadataJSON, err := marshalLoopRunCreatePayload(candidate)
	if err != nil {
		return looppkg.Run{}, err
	}
	if err := upsertLoopDefinitionSnapshot(ctx, exec, candidate, request.At); err != nil {
		return looppkg.Run{}, err
	}
	if err := insertLoopRun(ctx, exec, candidate, inputsJSON, metadataJSON); err != nil {
		return looppkg.Run{}, err
	}
	if candidate.BestGeneration != nil {
		if err := updateLoopRunBestWithExecutor(
			ctx, exec, candidate.WorkspaceID, candidate.ID, candidate.BestGeneration, candidate.BestScore,
		); err != nil {
			return looppkg.Run{}, err
		}
	}
	return candidate, nil
}

func (g *LoopRepo) initializeForkGenerations(
	ctx context.Context,
	exec taskSQLExecutor,
	candidate looppkg.Run,
	request looppkg.ForkStoreRequest,
) error {
	if err := insertLoopGenerationWithExecutor(ctx, exec, candidate.ID, looppkg.GenerationIntent{
		Generation: 1, ParentGeneration: 0, Origin: looppkg.OriginForkSeed,
	}, request.At); err != nil {
		return err
	}
	if err := validateForkSeedBlobs(ctx, exec, request.SeedOutputs); err != nil {
		return err
	}
	if err := insertTimeTravelOutputs(ctx, exec, candidate.ID, request.SeedOutputs); err != nil {
		return err
	}
	return insertLoopGenerationWithExecutor(ctx, exec, candidate.ID, looppkg.GenerationIntent{
		Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginInitial,
	}, request.At)
}

func (g *LoopRepo) finishFork(
	ctx context.Context,
	exec taskSQLExecutor,
	source looppkg.Run,
	candidate looppkg.Run,
	request looppkg.ForkStoreRequest,
) error {
	if candidate.Status == looppkg.StatusRunning {
		if _, _, err := g.reserveLoopCoordinatorRunWithExecutor(
			ctx, exec, candidate, loopCoordinatorStartOrigin(), request.At,
			loopCoordinatorRunID(candidate.ID, 2), loopCoordinatorIdempotencyKey(candidate.ID, 2),
		); err != nil {
			return err
		}
	}
	if err := appendLoopRunEventWithExecutor(ctx, exec, source.ID, source.WorkspaceID,
		loopRunEventRunForked, map[string]any{
			"source_run_id": source.ID, "source_generation": request.Operation.SourceGeneration,
			"fork_run_id": candidate.ID,
		}, request.At); err != nil {
		return err
	}
	return insertTimeTravelOp(ctx, exec, source.WorkspaceID, request.Operation)
}
