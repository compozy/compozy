package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func insertLoopGenerationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	intent looppkg.GenerationIntent,
	createdAt time.Time,
) error {
	if strings.TrimSpace(string(runID)) == "" {
		return fmt.Errorf("%w: loop run id is required", looppkg.ErrValidation)
	}
	if err := intent.Validate(); err != nil {
		return fmt.Errorf("store: validate loop generation: %w", err)
	}
	if err := sqlcgen.New(exec).InsertLoopGeneration(ctx, sqlcgen.InsertLoopGenerationParams{
		LoopRunID:        string(runID),
		Generation:       intent.Generation,
		ParentGeneration: intent.ParentGeneration,
		Origin:           string(intent.Origin),
		CreatedAt:        createdAt.UTC(),
	}); err != nil {
		return fmt.Errorf("store: insert loop generation %d for run %q: %w", intent.Generation, runID, err)
	}
	return nil
}

func insertLoopGateVerdictWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	generation int,
	intent gate.VerdictIntent,
	decidedAt time.Time,
) error {
	if strings.TrimSpace(string(runID)) == "" || generation < 1 || strings.TrimSpace(intent.GateID) == "" {
		return fmt.Errorf("%w: loop gate verdict identity is invalid", looppkg.ErrValidation)
	}
	if !intent.Outcome.Valid() {
		return fmt.Errorf("%w: loop gate verdict outcome is invalid: %q", looppkg.ErrValidation, intent.Outcome)
	}
	if !json.Valid(intent.BlockingIssues) || !json.Valid(intent.Criteria) {
		return fmt.Errorf("%w: loop gate verdict diagnostics must be JSON", looppkg.ErrValidation)
	}
	params := sqlcgen.InsertLoopGateVerdictParams{
		LoopRunID:          string(runID),
		Generation:         int64(generation),
		GateID:             strings.TrimSpace(intent.GateID),
		ItemIndex:          int64(intent.ItemIndex),
		Outcome:            string(intent.Outcome),
		BlockingIssuesJson: string(intent.BlockingIssues),
		CriteriaJson:       string(intent.Criteria),
		DecidedAt:          decidedAt.UTC(),
	}
	if intent.ItemIndex < 0 {
		return fmt.Errorf("%w: loop gate verdict item index must be non-negative", looppkg.ErrValidation)
	}
	if intent.Score != nil {
		if math.IsNaN(*intent.Score) || math.IsInf(*intent.Score, 0) {
			return fmt.Errorf("%w: loop gate verdict score must be finite", looppkg.ErrValidation)
		}
		params.Score = sql.NullFloat64{Float64: *intent.Score, Valid: true}
	}
	if intent.RouteCauseRank != nil {
		if *intent.RouteCauseRank < 0 {
			return fmt.Errorf("%w: loop gate verdict route cause rank must be non-negative", looppkg.ErrValidation)
		}
		params.RouteCauseRank = sql.NullInt64{Int64: int64(*intent.RouteCauseRank), Valid: true}
	}
	if err := sqlcgen.New(exec).InsertLoopGateVerdict(ctx, params); err != nil {
		return fmt.Errorf("store: insert loop gate verdict for run %q generation %d: %w", runID, generation, err)
	}
	return nil
}

func updateLoopRunBestWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	generation *int64,
	score *float64,
) error {
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(runID)) == "" {
		return fmt.Errorf("%w: workspace and loop run id are required", looppkg.ErrValidation)
	}
	if (generation == nil) != (score == nil) {
		return fmt.Errorf("%w: loop run best generation and score must be set together", looppkg.ErrValidation)
	}
	params := sqlcgen.UpdateLoopRunBestParams{LoopRunID: string(runID), WorkspaceID: string(workspaceID)}
	if generation != nil {
		if *generation < 1 || math.IsNaN(*score) || math.IsInf(*score, 0) {
			return fmt.Errorf("%w: loop run best state is invalid", looppkg.ErrValidation)
		}
		params.BestGeneration = sql.NullInt64{Int64: *generation, Valid: true}
		params.BestScore = sql.NullFloat64{Float64: *score, Valid: true}
	}
	affected, err := sqlcgen.New(exec).UpdateLoopRunBest(ctx, params)
	if err != nil {
		return fmt.Errorf("store: update loop run %q best state: %w", runID, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: loop run %q not found in workspace", looppkg.ErrRunNotFound, runID)
	}
	return nil
}
