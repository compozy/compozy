package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// ImportRunHistory atomically imports one already-executed Loop run together with its
// generation, output, attempt, control, wait, request, verdict, and event ledger.
func (g *LoopRepo) ImportRunHistory(ctx context.Context, command *looppkg.RunHistoryImport) error {
	if err := g.checkReady(ctx, "import loop run history"); err != nil {
		return err
	}
	if command == nil {
		return fmt.Errorf("%w: run history import command is required", looppkg.ErrValidation)
	}
	prepared, err := prepareLoopRunHistoryImport(command)
	if err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "import loop run history", func(exec taskSQLExecutor) error {
		if err := insertLoopRunHistoryRecord(ctx, exec, &prepared); err != nil {
			return err
		}
		return importLoopRunHistoryLedger(ctx, exec, &prepared.snapshot)
	})
}

// ImportRunHistoryBatch atomically imports a related set of historical runs.
// Rows are materialized before lineage validation so parent and child ledgers
// may reference each other without weakening the single-run boundary.
func (g *LoopRepo) ImportRunHistoryBatch(ctx context.Context, commands []*looppkg.RunHistoryImport) error {
	if err := g.checkReady(ctx, "import loop run history batch"); err != nil {
		return err
	}
	if len(commands) == 0 {
		return fmt.Errorf("%w: run history import batch is required", looppkg.ErrValidation)
	}
	prepared := make([]preparedLoopRunHistoryImport, 0, len(commands))
	for index, command := range commands {
		if command == nil {
			return fmt.Errorf("%w: run history import command %d is required", looppkg.ErrValidation, index)
		}
		item, err := prepareLoopRunHistoryImport(command)
		if err != nil {
			return err
		}
		prepared = append(prepared, item)
	}
	return g.withTaskImmediateTransaction(ctx, "import loop run history batch", func(exec taskSQLExecutor) error {
		for index := range prepared {
			if err := insertLoopRunHistoryRecord(ctx, exec, &prepared[index]); err != nil {
				return err
			}
		}
		for index := range prepared {
			if err := importLoopRunHistoryLedger(ctx, exec, &prepared[index].snapshot); err != nil {
				return err
			}
		}
		return nil
	})
}

type preparedLoopRunHistoryImport struct {
	snapshot          looppkg.RunHistorySnapshot
	inputsJSON        []byte
	startMetadataJSON []byte
}

func prepareLoopRunHistoryImport(command *looppkg.RunHistoryImport) (preparedLoopRunHistoryImport, error) {
	snapshot := command.Snapshot()
	run, err := normalizeLoopRunForHistoryImport(snapshot.Run)
	if err != nil {
		return preparedLoopRunHistoryImport{}, err
	}
	snapshot.Run = run
	inputsJSON, startMetadataJSON, err := marshalLoopRunCreatePayload(run)
	if err != nil {
		return preparedLoopRunHistoryImport{}, err
	}
	decisions, err := normalizeLoopGateDecisionRecords(snapshot.Decisions, run.CreatedAt)
	if err != nil {
		return preparedLoopRunHistoryImport{}, err
	}
	snapshot.Decisions = decisions
	return preparedLoopRunHistoryImport{
		snapshot: snapshot, inputsJSON: inputsJSON, startMetadataJSON: startMetadataJSON,
	}, nil
}

// DeleteRunHistory removes one imported run and its workspace-scoped event ledger.
// It refuses to touch a run that belongs to another workspace.
func (g *LoopRepo) DeleteRunHistory(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) error {
	if err := g.checkReady(ctx, "delete loop run history"); err != nil {
		return err
	}
	trimmedWorkspace := strings.TrimSpace(string(workspaceID))
	trimmedRun := strings.TrimSpace(string(runID))
	if trimmedWorkspace == "" || trimmedRun == "" {
		return fmt.Errorf("%w: workspace_id and run id are required", looppkg.ErrValidation)
	}
	return g.withTaskImmediateTransaction(ctx, "delete loop run history", func(exec taskSQLExecutor) error {
		var historical int64
		if err := exec.QueryRowContext(
			ctx,
			`SELECT historical FROM loop_runs WHERE id = ? AND workspace_id = ?`,
			trimmedRun,
			trimmedWorkspace,
		).Scan(&historical); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("store: inspect loop run history %q: %w", trimmedRun, err)
		}
		if historical != 1 {
			return fmt.Errorf(
				"%w: loop run %q is live runtime state, not imported history",
				looppkg.ErrInvalidTransition,
				trimmedRun,
			)
		}
		if _, err := exec.ExecContext(
			ctx,
			`DELETE FROM loop_run_events WHERE loop_run_id = ? AND workspace_id = ?`,
			trimmedRun, trimmedWorkspace,
		); err != nil {
			return fmt.Errorf("store: delete loop run events %q: %w", trimmedRun, err)
		}
		if _, err := exec.ExecContext(
			ctx,
			`DELETE FROM loop_runs WHERE id = ? AND workspace_id = ?`,
			trimmedRun, trimmedWorkspace,
		); err != nil {
			return fmt.Errorf("store: delete loop run %q: %w", trimmedRun, err)
		}
		return nil
	})
}

func insertLoopRunHistoryRecord(
	ctx context.Context,
	exec taskSQLExecutor,
	prepared *preparedLoopRunHistoryImport,
) error {
	run := prepared.snapshot.Run
	if err := upsertLoopDefinitionSnapshot(ctx, exec, run, run.CreatedAt); err != nil {
		return err
	}
	return insertLoopRun(ctx, exec, run, prepared.inputsJSON, prepared.startMetadataJSON)
}

func importLoopRunHistoryLedger(
	ctx context.Context,
	exec taskSQLExecutor,
	snapshot *looppkg.RunHistorySnapshot,
) error {
	run := snapshot.Run
	if err := validateLoopHistoryParent(ctx, exec, run); err != nil {
		return err
	}
	finalizer := looppkg.NewStoreFinalizer()
	for _, generation := range snapshot.Generations {
		if err := validateLoopHistoryReferences(ctx, exec, run, generation.Outputs); err != nil {
			return err
		}
		if err := importLoopHistoryGeneration(ctx, exec, finalizer, run, generation); err != nil {
			return err
		}
	}
	for _, decision := range snapshot.Decisions {
		if err := insertLoopGateDecision(ctx, exec, decision); err != nil {
			return err
		}
	}
	if err := importLoopHistoryGoalTurns(ctx, exec, run, snapshot.GoalTurns); err != nil {
		return err
	}
	if snapshot.Best != nil {
		best := snapshot.Best
		if err := updateLoopRunBestWithExecutor(
			ctx, exec, run.WorkspaceID, run.ID, &best.Generation, &best.Score,
		); err != nil {
			return err
		}
	}
	return importLoopHistoryEvents(ctx, exec, run, snapshot.Events)
}

func importLoopHistoryGoalTurns(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	turns []looppkg.RunHistoryGoalTurn,
) error {
	const statement = `INSERT INTO loop_goal_turns (
		loop_run_id, seq, generation, node_id, item_index, turn,
		session_id, binding_handle, binding_epoch, prompt_id, prompt_attempt,
		usage_base_tokens, result_status, stop_reason, verdict_outcome,
		blocking_json, criteria_json, warnings_json, evidence_ref, prompt_ref,
		tokens_used, actor_kind, actor_id, started_at, ended_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'completed', ?, NULLIF(?, ''),
		?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?)`
	for _, turn := range turns {
		var sessionWorkspace string
		err := exec.QueryRowContext(
			ctx,
			`SELECT workspace_id FROM sessions WHERE id = ?`,
			turn.SessionID,
		).Scan(&sessionWorkspace)
		if err != nil {
			return fmt.Errorf("store: inspect goal turn %d session %q: %w", turn.Seq, turn.SessionID, err)
		}
		if sessionWorkspace != string(run.WorkspaceID) {
			return fmt.Errorf(
				"%w: goal turn %d session %q is not owned by workspace %q",
				looppkg.ErrValidation,
				turn.Seq,
				turn.SessionID,
				run.WorkspaceID,
			)
		}
		if _, err := exec.ExecContext(
			ctx,
			statement,
			run.ID, turn.Seq, turn.Generation, turn.NodeID, turn.ItemIndex, turn.Turn,
			turn.SessionID, turn.BindingHandle, turn.BindingEpoch, turn.PromptID,
			turn.PromptAttempt, turn.UsageBaseTokens, turn.StopReason, turn.VerdictOutcome,
			string(turn.BlockingIssues), string(turn.Criteria), string(turn.Warnings),
			turn.EvidenceRef, turn.PromptRef, turn.TokensUsed, turn.ActorKind, turn.ActorID,
			turn.StartedAt.UTC(), turn.EndedAt.UTC(),
		); err != nil {
			return fmt.Errorf("store: import goal turn %d for run %q: %w", turn.Seq, run.ID, err)
		}
	}
	return nil
}

func validateLoopHistoryReferences(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	outputs []looppkg.GenerationOutput,
) error {
	for _, output := range outputs {
		if output.TaskRunID != "" {
			var workspaceID string
			err := exec.QueryRowContext(
				ctx,
				`SELECT workspace_id FROM task_runs WHERE id = ?`,
				output.TaskRunID,
			).Scan(&workspaceID)
			if err != nil {
				return fmt.Errorf("store: inspect output %q task run %q: %w", output.NodeID, output.TaskRunID, err)
			}
			if workspaceID != string(run.WorkspaceID) {
				return fmt.Errorf(
					"%w: output %q task run %q is not owned by workspace %q",
					looppkg.ErrValidation, output.NodeID, output.TaskRunID, run.WorkspaceID,
				)
			}
		}
		if output.ChildLoopRunID != "" {
			var workspaceID string
			err := exec.QueryRowContext(
				ctx,
				`SELECT workspace_id FROM loop_runs WHERE id = ?`,
				output.ChildLoopRunID,
			).Scan(&workspaceID)
			if err != nil {
				return fmt.Errorf(
					"store: inspect output %q child run %q: %w",
					output.NodeID,
					output.ChildLoopRunID,
					err,
				)
			}
			if workspaceID != string(run.WorkspaceID) {
				return fmt.Errorf(
					"%w: output %q child run %q is not owned by workspace %q",
					looppkg.ErrValidation, output.NodeID, output.ChildLoopRunID, run.WorkspaceID,
				)
			}
		}
	}
	return nil
}

func validateLoopHistoryParent(ctx context.Context, exec taskSQLExecutor, run looppkg.Run) error {
	parentID := strings.TrimSpace(string(run.ParentLoopRunID))
	if parentID == "" {
		return nil
	}
	if parentID == strings.TrimSpace(string(run.ID)) {
		return fmt.Errorf("%w: loop run %q cannot be its own parent", looppkg.ErrValidation, run.ID)
	}
	var workspaceID string
	if err := exec.QueryRowContext(
		ctx,
		`SELECT workspace_id FROM loop_runs WHERE id = ?`,
		parentID,
	).Scan(&workspaceID); err != nil {
		return fmt.Errorf("store: inspect parent loop run %q: %w", parentID, err)
	}
	if workspaceID != string(run.WorkspaceID) {
		return fmt.Errorf(
			"%w: parent loop run %q is not owned by workspace %q",
			looppkg.ErrValidation, parentID, run.WorkspaceID,
		)
	}
	return nil
}

func importLoopHistoryGeneration(
	ctx context.Context,
	exec taskSQLExecutor,
	finalizer *looppkg.StoreFinalizer,
	run looppkg.Run,
	generation looppkg.RunHistoryGeneration,
) error {
	number := int(generation.Intent.Generation)
	if err := insertLoopGenerationWithExecutor(
		ctx, exec, run.ID, generation.Intent, generation.CreatedAt,
	); err != nil {
		return err
	}
	payload := looppkg.GenerationSnapshotPayload{
		Outputs:     generation.Outputs,
		OutputBlobs: generation.OutputBlobs,
		Attempts:    generation.Attempts,
		Controls:    generation.Controls,
		Waits:       generation.Waits,
		Requests:    generation.Requests,
	}
	if err := finalizer.WriteGenerationSnapshot(ctx, exec, taskpkg.GenerationSnapshot{
		LoopRunID: string(run.ID), Generation: number, Payload: payload,
	}); err != nil {
		return err
	}
	for _, verdict := range generation.Verdicts {
		if err := insertLoopGateVerdictWithExecutor(
			ctx, exec, run.ID, number, verdict.Intent, verdict.DecidedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func importLoopHistoryEvents(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	events []looppkg.RunHistoryEvent,
) error {
	for _, event := range events {
		if err := appendLoopRunEventWithExecutor(
			ctx, exec, run.ID, run.WorkspaceID, string(event.Kind), event.Payload, event.At,
		); err != nil {
			return err
		}
	}
	return nil
}

func normalizeLoopRunForHistoryImport(run looppkg.Run) (looppkg.Run, error) {
	if err := normalizeLoopRunIdentity(&run); err != nil {
		return looppkg.Run{}, err
	}
	run.DefinitionDigest = strings.TrimSpace(run.DefinitionDigest)
	run.ActiveGateID = looppkg.NodeID(strings.TrimSpace(string(run.ActiveGateID)))
	if len(run.ActiveHumanCriteria) == 0 {
		run.ActiveHumanCriteria = json.RawMessage(`[]`)
	}
	if run.ReattemptStrategy == "" {
		run.ReattemptStrategy = looppkg.ReattemptFailedOnly
	}
	if run.BudgetOnExceeded == "" {
		run.BudgetOnExceeded = dsl.BudgetExceededHalt
	}
	if run.Inputs == nil {
		run.Inputs = map[string]any{}
	}
	if run.StartMetadata == nil {
		run.StartMetadata = map[string]any{}
	}
	if err := (looppkg.GoalRunPolicy{ContextNudgeRatio: run.GoalContextNudgeRatio}).Validate(); err != nil {
		return looppkg.Run{}, err
	}
	origin := looppkg.RunOrigin{}
	if run.Origin != nil {
		origin = *run.Origin
	}
	origin = origin.Normalize()
	if err := origin.Validate(); err != nil {
		return looppkg.Run{}, err
	}
	run.Origin = &origin
	return run, nil
}
