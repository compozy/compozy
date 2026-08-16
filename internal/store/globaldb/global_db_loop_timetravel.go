package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

var _ looppkg.TimeTravelStore = (*LoopRepo)(nil)

func (g *LoopRepo) LookupRerunReplay(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	key string,
	digest string,
) (looppkg.RerunResult, bool, error) {
	if err := g.checkReady(ctx, "lookup Loop rerun replay"); err != nil {
		return looppkg.RerunResult{}, false, err
	}
	prior, found, err := getTimeTravelReplay(ctx, g.db, workspaceID, key)
	if err != nil || !found {
		return looppkg.RerunResult{}, found, err
	}
	if prior.kind != "rerun" || prior.digest != digest {
		return looppkg.RerunResult{}, false, timeTravelKeyReuseError(strings.TrimSpace(key))
	}
	return looppkg.RerunResult{
		RunID: prior.resultRunID, Generation: valueOrZero(prior.resultGeneration),
		ParentGeneration: valueOrZero(prior.sourceGeneration), Replayed: true,
	}, true, nil
}

func (g *LoopRepo) CreateRerun(
	ctx context.Context,
	request looppkg.RerunStoreRequest,
) (result looppkg.RerunResult, replayed bool, err error) {
	if err := g.checkReady(ctx, "create Loop rerun"); err != nil {
		return looppkg.RerunResult{}, false, err
	}
	err = g.withTaskImmediateTransaction(ctx, "create Loop rerun", func(exec taskSQLExecutor) error {
		prior, found, lookupErr := getTimeTravelReplay(ctx, exec, request.WorkspaceID, request.IdempotencyKey)
		if lookupErr != nil {
			return lookupErr
		}
		if found {
			if prior.digest != request.RequestDigest || prior.kind != "rerun" {
				return timeTravelKeyReuseError(request.IdempotencyKey)
			}
			result = looppkg.RerunResult{
				RunID: prior.resultRunID, Generation: valueOrZero(prior.resultGeneration),
				ParentGeneration: valueOrZero(prior.sourceGeneration), Replayed: true,
			}
			replayed = true
			return nil
		}
		current, loadErr := getLoopRunByIDWithExecutor(ctx, exec, request.Source.ID)
		if loadErr != nil {
			return loadErr
		}
		if current.WorkspaceID != request.WorkspaceID || current.Generation != request.Source.Generation ||
			!current.Status.Terminal() {
			return &looppkg.ReasonError{Code: looppkg.ReasonCodeRerunBusy, Err: looppkg.ErrRerunBusy}
		}
		if err := insertLoopGenerationWithExecutor(ctx, exec, current.ID, request.Intent, request.At); err != nil {
			return err
		}
		if err := insertTimeTravelOutputs(ctx, exec, current.ID, request.NextOutputs); err != nil {
			return err
		}
		affected, updateErr := exec.ExecContext(ctx, `UPDATE loop_runs SET
			status = 'running', completion_state = 'complete', pause_requested = 0,
			cancel_requested = 0, cancel_kind = '', active_gate_id = '', active_human_criteria_json = '[]',
			generation = ?, last_progress_at = ?
			WHERE id = ? AND workspace_id = ? AND generation = ? AND status = ?`,
			request.Intent.Generation, request.At.UTC(), current.ID, current.WorkspaceID, current.Generation, current.Status)
		if updateErr != nil {
			return fmt.Errorf("store: reactivate rerun %q: %w", current.ID, updateErr)
		}
		rows, rowsErr := affected.RowsAffected()
		if rowsErr != nil {
			return fmt.Errorf("store: inspect rerun reactivation: %w", rowsErr)
		}
		if rows != 1 {
			return &looppkg.ReasonError{Code: looppkg.ReasonCodeRerunBusy, Err: looppkg.ErrRerunBusy}
		}
		current.Status = looppkg.StatusRunning
		current.Generation = int(request.Intent.Generation)
		current.PauseRequested, current.CancelRequested = false, false
		if err := g.repairLoopCoordinatorTaskWithExecutor(
			ctx, exec, current, loopCoordinatorTaskID(current.ID), request.At,
		); err != nil && !errorsIsTaskNotFound(err) {
			return err
		}
		generation := int(request.Intent.Generation)
		if _, _, err := g.reserveLoopCoordinatorRunWithExecutor(
			ctx, exec, current, loopCoordinatorStartOrigin(), request.At,
			loopCoordinatorRunID(current.ID, generation), loopCoordinatorIdempotencyKey(current.ID, generation),
		); err != nil {
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
		result = looppkg.RerunResult{
			RunID: current.ID, Generation: request.Intent.Generation,
			ParentGeneration: request.Intent.ParentGeneration,
		}
		return nil
	})
	return result, replayed, err
}

func (g *LoopRepo) CreateFork(
	ctx context.Context,
	request looppkg.ForkStoreRequest,
) (created looppkg.Run, replayed bool, err error) {
	if err := g.checkReady(ctx, "create Loop fork"); err != nil {
		return looppkg.Run{}, false, err
	}
	err = g.withTaskImmediateTransaction(ctx, "create Loop fork", func(exec taskSQLExecutor) error {
		prior, found, lookupErr := getTimeTravelReplay(
			ctx, exec, request.Source.WorkspaceID, request.IdempotencyKey,
		)
		if lookupErr != nil {
			return lookupErr
		}
		if found {
			if prior.digest != request.RequestDigest || prior.kind != "fork" {
				return timeTravelKeyReuseError(request.IdempotencyKey)
			}
			replayedRun, loadErr := getLoopRunByIDWithExecutor(ctx, exec, prior.resultRunID)
			if loadErr != nil {
				return loadErr
			}
			created, replayed = replayedRun, true
			return nil
		}
		source, loadErr := getLoopRunByIDWithExecutor(ctx, exec, request.Source.ID)
		if loadErr != nil {
			return loadErr
		}
		if source.WorkspaceID != request.Source.WorkspaceID {
			return looppkg.ErrRunNotFound
		}
		candidate, policyErr := applyLoopStartConcurrencyPolicy(ctx, exec, request.Child, request.Concurrency)
		if policyErr != nil {
			return policyErr
		}
		inputsJSON, metadataJSON, marshalErr := marshalLoopRunCreatePayload(candidate)
		if marshalErr != nil {
			return marshalErr
		}
		if err := upsertLoopDefinitionSnapshot(ctx, exec, candidate, request.At); err != nil {
			return err
		}
		if err := insertLoopRun(ctx, exec, candidate, inputsJSON, metadataJSON); err != nil {
			return err
		}
		if candidate.BestGeneration != nil {
			if err := updateLoopRunBestWithExecutor(
				ctx, exec, candidate.WorkspaceID, candidate.ID, candidate.BestGeneration, candidate.BestScore,
			); err != nil {
				return err
			}
		}
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
		if err := insertLoopGenerationWithExecutor(ctx, exec, candidate.ID, looppkg.GenerationIntent{
			Generation: 2, ParentGeneration: 1, Origin: looppkg.OriginInitial,
		}, request.At); err != nil {
			return err
		}
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
		if err := insertTimeTravelOp(ctx, exec, source.WorkspaceID, request.Operation); err != nil {
			return err
		}
		created = candidate
		return nil
	})
	return created, replayed, err
}

func (g *LoopRepo) ListForks(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (forks []looppkg.ForkRef, err error) {
	if err := g.checkReady(ctx, "list Loop forks"); err != nil {
		return nil, err
	}
	rows, err := g.db.QueryContext(ctx, `SELECT id, forked_from_generation FROM loop_runs
		WHERE workspace_id = ? AND forked_from_run_id = ? ORDER BY created_at ASC, id ASC`, workspaceID, runID)
	if err != nil {
		return nil, fmt.Errorf("store: list Loop forks: %w", err)
	}
	defer func() { err = joinRowsCloseError(rows, err, "Loop forks") }()
	for rows.Next() {
		var ref looppkg.ForkRef
		if err := rows.Scan(&ref.RunID, &ref.Generation); err != nil {
			return nil, fmt.Errorf("store: scan Loop fork: %w", err)
		}
		forks = append(forks, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate Loop forks: %w", err)
	}
	return forks, nil
}

type storedTimeTravelReplay struct {
	kind             string
	digest           string
	resultRunID      looppkg.RunID
	resultGeneration sql.NullInt64
	sourceGeneration sql.NullInt64
}

func getTimeTravelReplay(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	key string,
) (storedTimeTravelReplay, bool, error) {
	if strings.TrimSpace(key) == "" {
		return storedTimeTravelReplay{}, false, nil
	}
	var replay storedTimeTravelReplay
	err := exec.QueryRowContext(ctx, `SELECT kind, request_digest, result_run_id, result_generation,
		source_generation FROM loop_timetravel_ops WHERE workspace_id = ? AND idempotency_key = ?`,
		workspaceID, strings.TrimSpace(key)).Scan(
		&replay.kind, &replay.digest, &replay.resultRunID, &replay.resultGeneration, &replay.sourceGeneration,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedTimeTravelReplay{}, false, nil
	}
	if err != nil {
		return storedTimeTravelReplay{}, false, fmt.Errorf("store: read time-travel replay: %w", err)
	}
	return replay, true, nil
}

func insertTimeTravelOp(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	op looppkg.TimeTravelOp,
) error {
	_, err := exec.ExecContext(ctx, `INSERT INTO loop_timetravel_ops (
		workspace_id, op_id, kind, idempotency_key, request_digest, source_run_id,
		source_generation, from_node, item_index, actor_kind, actor_id, reason,
		result_run_id, result_generation, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workspaceID, op.ID, op.Kind, op.IdempotencyKey, op.RequestDigest, op.SourceRunID,
		nullInt64Pointer(op.SourceGeneration), nullNodeID(op.FromNode), nullIntPointer(op.ItemIndex),
		op.Actor.Actor.Kind, op.Actor.Actor.Ref, nullTrimmed(op.Reason), op.ResultRunID,
		nullInt64Pointer(op.ResultGeneration), op.CreatedAt.UTC())
	if err != nil {
		return fmt.Errorf("store: insert time-travel operation %q: %w", op.ID, err)
	}
	return nil
}

func insertTimeTravelOutputs(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	outputs []looppkg.GenerationOutput,
) error {
	for _, output := range outputs {
		_, err := exec.ExecContext(ctx, `INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status, output_ref, attempt, epoch
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, runID, output.Generation, output.NodeID, output.ItemIndex,
			output.Status, nullTrimmed(output.OutputRef), maxInt(output.Attempt, 1), output.Epoch)
		if err != nil {
			return fmt.Errorf("store: insert time-travel output %s/%d: %w", output.NodeID, output.ItemIndex, err)
		}
	}
	return nil
}

func validateForkSeedBlobs(ctx context.Context, exec taskSQLExecutor, outputs []looppkg.GenerationOutput) error {
	for _, output := range outputs {
		if strings.TrimSpace(output.OutputRef) == "" {
			continue
		}
		var exists int
		if err := exec.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM loop_output_blobs WHERE output_ref = ?)`, output.OutputRef).Scan(&exists); err != nil {
			return fmt.Errorf("store: validate fork seed blob %q: %w", output.OutputRef, err)
		}
		if exists != 1 {
			return fmt.Errorf("%w: fork seed output %q", looppkg.ErrOutputRefNotFound, output.OutputRef)
		}
	}
	return nil
}

func timeTravelKeyReuseError(key string) error {
	return &looppkg.ReasonError{Code: looppkg.ReasonCodeTimeTravelKeyReuse,
		Err: looppkg.ErrTimeTravelKeyReuse, Meta: map[string]string{"request_id": key}}
}

func valueOrZero(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}
func nullInt64Pointer(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
func nullIntPointer(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
func nullNodeID(value looppkg.NodeID) any { return nullTrimmed(string(value)) }
func nullTrimmed(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
