package globaldb

import (
	"context"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListNodeAttempts returns the ordered workspace-scoped attempt ledger for one run.
func (g *LoopRepo) ListNodeAttempts(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.NodeAttempt, error) {
	if err := g.checkReady(ctx, "list node attempts"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListLoopNodeAttempts(ctx, sqlcgen.ListLoopNodeAttemptsParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop node attempts: %w", err)
	}
	attempts := make([]looppkg.NodeAttempt, 0, len(rows))
	for _, row := range rows {
		attempt, mapErr := loopNodeAttemptFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		attempts = append(attempts, attempt)
	}
	return attempts, nil
}

func loopNodeAttemptFromGenerated(row sqlcgen.LoopNodeAttempt) (looppkg.NodeAttempt, error) {
	disposition := looppkg.AttemptDisposition(row.Disposition)
	if !disposition.Valid() {
		return looppkg.NodeAttempt{}, fmt.Errorf(
			"store: invalid loop node attempt disposition %q: %w",
			row.Disposition,
			looppkg.ErrValidation,
		)
	}
	attempt := looppkg.NodeAttempt{
		LoopRunID:     looppkg.RunID(row.LoopRunID),
		Generation:    int(row.Generation),
		NodeID:        looppkg.NodeID(row.NodeID),
		ItemIndex:     int(row.ItemIndex),
		Attempt:       int(row.Attempt),
		FailureCode:   row.FailureCode,
		Cause:         row.Cause,
		Hint:          row.Hint,
		Target:        row.Target,
		Disposition:   disposition,
		StartedAt:     row.StartedAt.UTC(),
		EndedAt:       loopTimePointer(row.EndedAt),
		NextAttemptAt: loopTimePointer(row.NextAttemptAt),
	}
	if row.FailureClass.Valid {
		failureClass := looppkg.FailureClass(row.FailureClass.String)
		if !looppkg.IsKnownFailureClass(failureClass) {
			return looppkg.NodeAttempt{}, fmt.Errorf(
				"store: invalid loop node failure class %q: %w",
				failureClass,
				looppkg.ErrValidation,
			)
		}
		attempt.FailureClass = &failureClass
	}
	return attempt, nil
}
