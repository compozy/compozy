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

var (
	_ looppkg.GenerationLineageReader = (*LoopRepo)(nil)
	_ gate.VerdictReader              = (*LoopRepo)(nil)
)

func (g *LoopRepo) ListGenerations(
	ctx context.Context,
	workspaceID string,
	runID string,
) ([]looppkg.LoopGeneration, error) {
	if err := g.checkReady(ctx, "list loop generations"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	runID = strings.TrimSpace(runID)
	if workspaceID == "" || runID == "" {
		return nil, fmt.Errorf("%w: workspace and loop run id are required", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopGenerations(ctx, sqlcgen.ListLoopGenerationsParams{
		WorkspaceID: workspaceID, LoopRunID: runID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop generations: %w", err)
	}
	items := make([]looppkg.LoopGeneration, 0, len(rows))
	for _, row := range rows {
		items = append(items, looppkg.LoopGeneration{
			RunID:            runID,
			Generation:       row.Generation,
			ParentGeneration: row.ParentGeneration,
			Origin:           looppkg.GenerationOrigin(row.Origin),
			CreatedAt:        row.CreatedAt.UTC(),
		})
	}
	return items, nil
}

func (g *LoopRepo) ListGateVerdicts(
	ctx context.Context,
	workspaceID string,
	runID string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	if err := g.checkReady(ctx, "list loop gate verdicts"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	runID = strings.TrimSpace(runID)
	if workspaceID == "" || runID == "" || generation < 1 {
		return nil, fmt.Errorf("%w: loop gate verdict identity is invalid", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopGateVerdicts(ctx, sqlcgen.ListLoopGateVerdictsParams{
		WorkspaceID: workspaceID,
		LoopRunID:   runID,
		Generation:  generation,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop gate verdicts: %w", err)
	}
	return gateVerdictRecords(runID, generation, rows)
}

func (g *LoopRepo) ListRouteCausingVerdicts(
	ctx context.Context,
	workspaceID string,
	runID string,
	generation int64,
) ([]gate.VerdictRecord, error) {
	if err := g.checkReady(ctx, "list route-causing loop gate verdicts"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	runID = strings.TrimSpace(runID)
	if workspaceID == "" || runID == "" || generation < 1 {
		return nil, fmt.Errorf("%w: loop gate verdict identity is invalid", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListRouteCausingLoopGateVerdicts(
		ctx,
		sqlcgen.ListRouteCausingLoopGateVerdictsParams{
			WorkspaceID: workspaceID,
			LoopRunID:   runID,
			Generation:  generation,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("store: list route-causing loop gate verdicts: %w", err)
	}
	items := make([]gate.VerdictRecord, 0, len(rows))
	for _, row := range rows {
		item, mapErr := gateVerdictRecord(
			runID,
			generation,
			row.GateID,
			row.ItemIndex,
			row.Outcome,
			row.Score,
			row.RouteCauseRank,
			row.BlockingIssuesJson,
			row.CriteriaJson,
			row.DecidedAt,
		)
		if mapErr != nil {
			return nil, mapErr
		}
		items = append(items, item)
	}
	return items, nil
}

func gateVerdictRecords(
	runID string,
	generation int64,
	rows []sqlcgen.ListLoopGateVerdictsRow,
) ([]gate.VerdictRecord, error) {
	items := make([]gate.VerdictRecord, 0, len(rows))
	for _, row := range rows {
		item, err := gateVerdictRecord(
			runID,
			generation,
			row.GateID,
			row.ItemIndex,
			row.Outcome,
			row.Score,
			row.RouteCauseRank,
			row.BlockingIssuesJson,
			row.CriteriaJson,
			row.DecidedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func gateVerdictRecord(
	runID string,
	generation int64,
	gateID string,
	itemIndex int64,
	outcome string,
	score sql.NullFloat64,
	rank sql.NullInt64,
	blocking string,
	criteria string,
	decidedAt time.Time,
) (gate.VerdictRecord, error) {
	if !json.Valid([]byte(blocking)) || !json.Valid([]byte(criteria)) {
		return gate.VerdictRecord{}, fmt.Errorf(
			"store: decode loop gate verdict diagnostics: %w",
			looppkg.ErrValidation,
		)
	}
	if itemIndex < 0 {
		return gate.VerdictRecord{}, fmt.Errorf(
			"store: loop gate verdict item index is negative: %w",
			looppkg.ErrValidation,
		)
	}
	record := gate.VerdictRecord{
		RunID:          strings.TrimSpace(runID),
		Generation:     generation,
		GateID:         strings.TrimSpace(gateID),
		ItemIndex:      int(itemIndex),
		Outcome:        gate.VerdictOutcome(outcome),
		BlockingIssues: json.RawMessage(blocking),
		Criteria:       json.RawMessage(criteria),
		DecidedAt:      decidedAt.UTC(),
	}
	if score.Valid {
		if math.IsNaN(score.Float64) || math.IsInf(score.Float64, 0) {
			return gate.VerdictRecord{}, fmt.Errorf(
				"store: loop gate verdict score is non-finite: %w",
				looppkg.ErrValidation,
			)
		}
		value := score.Float64
		record.Score = &value
	}
	if rank.Valid {
		if rank.Int64 < 0 {
			return gate.VerdictRecord{}, fmt.Errorf(
				"store: loop gate verdict route cause rank is negative: %w",
				looppkg.ErrValidation,
			)
		}
		value := int(rank.Int64)
		record.RouteCauseRank = &value
	}
	return record, nil
}
