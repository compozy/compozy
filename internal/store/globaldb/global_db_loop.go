package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ looppkg.Store = (*LoopRepo)(nil)

const loopRunSelectColumnsSQL = `
	id, workspace_id, loop_name, status, generation, reattempt_strategy, created_at, started_at,
	last_progress_at, definition_version, definition_digest, active_gate_id,
	active_human_criteria_json, budget_approval_seq, start_metadata_json,
	budget_tokens, budget_wall_sec,
	budget_on_exceeded, tokens_used, parent_loop_run_id, pause_requested,
	control_actor_kind, control_actor_id, control_requested_at, inputs_json, iteration_cap,
	started_by_kind, started_by_ref, started_origin_kind, started_origin_ref,
	goal_context_nudge_ratio, origin_kind, origin_session_id,
	origin_creation_profile_ref, origin_policy_spec_digest, origin_creation_digest,
	network_spec_json, network_mode, network_channel, network_source`

// CreateLoopRunForStart atomically applies the loop concurrency policy and persists a new run.
func (g *LoopRepo) CreateLoopRunForStart(
	ctx context.Context,
	run looppkg.Run,
	policy dsl.ConcurrencyPolicy,
) (looppkg.Run, error) {
	if err := g.checkReady(ctx, "create loop run for start"); err != nil {
		return looppkg.Run{}, err
	}
	normalized, err := normalizeLoopRunForCreate(run)
	if err != nil {
		return looppkg.Run{}, err
	}
	if normalized.Origin.Kind != looppkg.RunOriginCatalog {
		return looppkg.Run{}, fmt.Errorf("%w: catalog start requires catalog Run origin", looppkg.ErrValidation)
	}
	inputsJSON, startMetadataJSON, err := marshalLoopRunCreatePayload(normalized)
	if err != nil {
		return looppkg.Run{}, err
	}
	if policy == "" {
		policy = dsl.ConcurrencyForbid
	}
	created := normalized
	err = g.withTaskImmediateTransaction(ctx, "create loop run for start", func(exec taskSQLExecutor) error {
		var decisionErr error
		created, decisionErr = applyLoopStartConcurrencyPolicy(ctx, exec, normalized, policy)
		if decisionErr != nil {
			return decisionErr
		}
		return g.persistStartedLoopRunWithExecutor(ctx, exec, created, inputsJSON, startMetadataJSON)
	})
	if err != nil {
		return looppkg.Run{}, err
	}
	return created, nil
}

func (g *LoopRepo) persistStartedLoopRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	inputsJSON []byte,
	startMetadataJSON []byte,
) error {
	if err := upsertLoopDefinitionSnapshot(ctx, exec, run, g.now()); err != nil {
		return err
	}
	if err := insertLoopRun(ctx, exec, run, inputsJSON, startMetadataJSON); err != nil {
		return err
	}
	if run.Status != looppkg.StatusRunning {
		return nil
	}
	_, _, err := g.reserveLoopCoordinatorRunWithExecutor(
		ctx,
		exec,
		run,
		loopCoordinatorStartOrigin(),
		run.CreatedAt,
		loopCoordinatorRunID(run.ID, run.Generation+1),
		loopCoordinatorIdempotencyKey(run.ID, run.Generation+1),
	)
	return err
}

func applyLoopStartConcurrencyPolicy(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	policy dsl.ConcurrencyPolicy,
) (looppkg.Run, error) {
	active, err := findActiveLoopRunWithExecutor(ctx, exec, run.WorkspaceID, run.LoopName)
	if err != nil {
		return looppkg.Run{}, err
	}
	created := run
	switch policy {
	case dsl.ConcurrencyForbid:
		if active != nil {
			return looppkg.Run{}, loopConcurrencyConflict(active)
		}
		created.Status = looppkg.StatusRunning
	case dsl.ConcurrencyQueue:
		if active != nil {
			created.Status = looppkg.StatusQueued
		} else {
			created.Status = looppkg.StatusRunning
		}
	case dsl.ConcurrencyAllow:
		created.Status = looppkg.StatusRunning
	default:
		return looppkg.Run{}, fmt.Errorf("%w: concurrency policy is invalid: %q", looppkg.ErrValidation, policy)
	}
	return created, nil
}

func insertLoopRun(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	inputsJSON []byte,
	startMetadataJSON []byte,
) error {
	params, err := loopRunInsertParams(run, inputsJSON, startMetadataJSON)
	if err != nil {
		return err
	}
	err = sqlcgen.New(exec).InsertLoopRun(ctx, params)
	if err != nil {
		return fmt.Errorf("store: insert loop run %q: %w", run.ID, err)
	}
	return appendLoopRunStatusEvent(
		ctx,
		exec,
		run.ID,
		run.WorkspaceID,
		"",
		run.Status,
		looppkg.TransitionCauseStart,
		run.CreatedAt,
	)
}

func marshalLoopRunCreatePayload(run looppkg.Run) ([]byte, []byte, error) {
	inputsJSON, err := json.Marshal(run.Inputs)
	if err != nil {
		return nil, nil, fmt.Errorf("store: marshal loop run inputs: %w", err)
	}
	startMetadataJSON, err := json.Marshal(run.StartMetadata)
	if err != nil {
		return nil, nil, fmt.Errorf("store: marshal loop run start metadata: %w", err)
	}
	return inputsJSON, startMetadataJSON, nil
}

// GetLoopRun loads one workspace-scoped loop_run.
func (g *LoopRepo) GetLoopRun(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	runID looppkg.RunID,
) (looppkg.Run, error) {
	if err := g.checkReady(ctx, "get loop run"); err != nil {
		return looppkg.Run{}, err
	}
	row, err := g.queries.GetLoopRun(ctx, sqlcgen.GetLoopRunParams{WorkspaceID: string(ws), ID: string(runID)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.Run{}, looppkg.ErrRunNotFound
		}
		return looppkg.Run{}, fmt.Errorf("store: get loop run: %w", err)
	}
	return loopRunFromGenerated(&row)
}

// GetLoopRunByID loads one loop_run without a workspace filter for ancestry/transition internals.
func (g *LoopRepo) GetLoopRunByID(ctx context.Context, runID looppkg.RunID) (looppkg.Run, error) {
	if err := g.checkReady(ctx, "get loop run by id"); err != nil {
		return looppkg.Run{}, err
	}
	row, err := g.queries.GetLoopRunByID(ctx, string(runID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.Run{}, looppkg.ErrRunNotFound
		}
		return looppkg.Run{}, fmt.Errorf("store: get loop run by id: %w", err)
	}
	return loopRunFromGenerated(&row)
}

// FindActiveLoopRun returns the oldest live run for a workspace loop.
func (g *LoopRepo) FindActiveLoopRun(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	loopName string,
) (*looppkg.Run, error) {
	if err := g.checkReady(ctx, "find active loop run"); err != nil {
		return nil, err
	}
	return findActiveLoopRunWithExecutor(ctx, g.db, ws, loopName)
}

func findActiveLoopRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	ws looppkg.WorkspaceID,
	loopName string,
) (*looppkg.Run, error) {
	row, err := sqlcgen.New(exec).FindActiveLoopRun(ctx, sqlcgen.FindActiveLoopRunParams{
		WorkspaceID: string(ws), LoopName: strings.TrimSpace(loopName),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: find active loop run: %w", err)
	}
	run, err := loopRunFromGenerated(&row)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func loopConcurrencyConflict(active *looppkg.Run) error {
	if active == nil {
		return looppkg.ErrConcurrencyConflict
	}
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeActiveRunExists,
		Err:  looppkg.ErrConcurrencyConflict,
		Meta: map[string]string{
			"active_run_id":            string(active.ID),
			columnLoopName:             active.LoopName,
			globalDBTaskClaimStatusKey: string(active.Status),
		},
	}
}

// CompareAndSwapLoopRunStatus updates status only from the expected durable value.
func (g *LoopRepo) CompareAndSwapLoopRunStatus(
	ctx context.Context,
	runID looppkg.RunID,
	from looppkg.Status,
	to looppkg.Status,
	cause looppkg.TransitionCause,
	at time.Time,
) error {
	if err := g.checkReady(ctx, "transition loop run status"); err != nil {
		return err
	}
	if strings.TrimSpace(string(runID)) == "" {
		return fmt.Errorf("%w: run id is required", looppkg.ErrValidation)
	}
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("%w: loop status is invalid", looppkg.ErrValidation)
	}
	if strings.TrimSpace(string(cause)) == "" {
		return fmt.Errorf("%w: transition cause is required", looppkg.ErrValidation)
	}
	if from == to {
		return nil
	}
	if at.IsZero() {
		at = g.now()
	}
	return g.withTaskImmediateTransaction(
		ctx,
		"transition loop run status",
		func(exec taskSQLExecutor) error {
			return compareAndSwapLoopRunStatusWithExecutor(ctx, exec, runID, from, to, cause, at)
		},
	)
}

// UpsertLoopConfig persists a no-fork per-loop config override.
func (g *LoopRepo) UpsertLoopConfig(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	loopName string,
	cfg looppkg.LoopConfig,
) error {
	if err := g.checkReady(ctx, "upsert loop config"); err != nil {
		return err
	}
	workspaceID := strings.TrimSpace(string(ws))
	trimmedLoopName := strings.TrimSpace(loopName)
	if workspaceID == "" {
		return fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	if trimmedLoopName == "" {
		return fmt.Errorf("%w: loop_name is required", looppkg.ErrValidation)
	}
	normalized, err := normalizeLoopConfigForStore(cfg)
	if err != nil {
		return err
	}
	patch := loopConfigPatchFlagsForStore(cfg, normalized)
	err = g.withImmediateTransaction(ctx, "upsert loop config", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		insert := sqlcgen.InsertLoopConfigIfMissingParams{
			WorkspaceID:      workspaceID,
			LoopName:         trimmedLoopName,
			HumanGateEnabled: int64(boolPtrToInt(normalized.HumanGateEnabled)),
			ReattemptStrategy: nullStringPtr(
				normalized.ReattemptStrategy,
			),
			EnabledChecksJson: enabledChecksForStore(normalized.EnabledChecks),
			IterationCap:      nullIntPtr(normalized.IterationCap),
			BudgetTokens:      nullIntPtr(normalized.BudgetTokens),
			BudgetWallSec: nullIntPtr(
				normalized.BudgetWallSec,
			),
			BudgetOnExceeded:   nullStringPtr(normalized.BudgetOnExceeded),
			NoProgressWindow:   nullIntPtr(normalized.NoProgressWindow),
			FanOutWidth:        nullIntPtr(normalized.FanOutWidth),
			GateMaxRevisions:   nullIntPtr(normalized.GateMaxRevisions),
			ModelDefaultWorker: modelDefaultNullString(normalized.ModelDefaults, true),
			ModelDefaultJudge:  modelDefaultNullString(normalized.ModelDefaults, false),
		}
		if insertErr := queries.InsertLoopConfigIfMissing(ctx, insert); insertErr != nil {
			return insertErr
		}
		return queries.PatchLoopConfig(ctx, loopConfigPatchParams(insert, patch))
	})
	if err != nil {
		return fmt.Errorf("store: upsert loop config %q/%q: %w", workspaceID, trimmedLoopName, err)
	}
	return nil
}

// GetLoopConfig loads a no-fork per-loop config override.
func (g *LoopRepo) GetLoopConfig(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	loopName string,
) (*looppkg.LoopConfig, error) {
	if err := g.checkReady(ctx, "get loop config"); err != nil {
		return nil, err
	}
	row, err := g.queries.GetLoopConfig(ctx, sqlcgen.GetLoopConfigParams{
		WorkspaceID: string(ws), LoopName: strings.TrimSpace(loopName),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, looppkg.ErrConfigNotFound
		}
		return nil, err
	}
	cfg := loopConfigFromGenerated(row)
	return &cfg, nil
}

func getLoopRunByIDWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
) (looppkg.Run, error) {
	row, err := sqlcgen.New(exec).GetLoopRunByID(ctx, string(runID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.Run{}, looppkg.ErrRunNotFound
		}
		return looppkg.Run{}, fmt.Errorf("store: get loop run by id: %w", err)
	}
	return loopRunFromGenerated(&row)
}
