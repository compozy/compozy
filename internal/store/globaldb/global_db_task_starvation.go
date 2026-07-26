package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// LoadRunStarvation reads a run's durable escalation budget. The bool reports row presence;
// absence is not an error — the convergence backstop treats it as a fresh budget.
func (g *TaskRunRepo) LoadRunStarvation(
	ctx context.Context,
	runID string,
) (taskpkg.RunStarvation, bool, error) {
	if err := g.checkReady(ctx, "load run starvation"); err != nil {
		return taskpkg.RunStarvation{}, false, err
	}
	id, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return taskpkg.RunStarvation{}, false, err
	}
	row, err := g.queries.GetRunStarvation(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return taskpkg.RunStarvation{}, false, nil
	}
	if err != nil {
		return taskpkg.RunStarvation{}, false, err
	}
	record, err := runStarvationFromGenerated(row)
	if err != nil {
		return taskpkg.RunStarvation{}, false, err
	}
	return record, true, nil
}

func runStarvationFromGenerated(row sqlcgen.TaskRunStarvation) (taskpkg.RunStarvation, error) {
	record := taskpkg.RunStarvation{
		RunID: row.RunID, WakeCount: int(row.WakeCount), EscalationTier: int(row.EscalationTier),
	}
	parsed, err := store.ParseTimestamp(row.FirstStarvedAt)
	if err != nil {
		return taskpkg.RunStarvation{}, fmt.Errorf("store: parse run starvation first_starved_at: %w", err)
	}
	record.FirstStarvedAt = parsed
	if record.UpdatedAt, err = store.ParseTimestamp(row.UpdatedAt); err != nil {
		return taskpkg.RunStarvation{}, fmt.Errorf("store: parse run starvation updated_at: %w", err)
	}
	last, err := parseNullableStarvationTime(row.LastWakeAt)
	if err != nil {
		return taskpkg.RunStarvation{}, err
	}
	if last != nil {
		record.LastWakeAt = *last
	}
	if record.SpawnRequestedAt, err = parseNullableStarvationTime(row.SpawnRequestedAt); err != nil {
		return taskpkg.RunStarvation{}, err
	}
	if record.StarvedEventAt, err = parseNullableStarvationTime(row.StarvedEventAt); err != nil {
		return taskpkg.RunStarvation{}, err
	}
	return record, nil
}

// ListRunStarvation returns every escalation budget row so the convergence backstop can reconcile
// rows whose run has left the queued set.
func (g *TaskRunRepo) ListRunStarvation(ctx context.Context) ([]taskpkg.RunStarvation, error) {
	if err := g.checkReady(ctx, "list run starvation"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListRunStarvation(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list run starvation: %w", err)
	}
	records := make([]taskpkg.RunStarvation, 0, len(rows))
	for _, row := range rows {
		record, mapErr := runStarvationFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		records = append(records, record)
	}
	return records, nil
}

// UpsertRunStarvation writes the run's escalation budget, advancing it in place on conflict.
func (g *TaskRunRepo) UpsertRunStarvation(
	ctx context.Context,
	mutation taskpkg.RunStarvationMutation,
) (taskpkg.RunStarvation, error) {
	if err := g.checkReady(ctx, "upsert run starvation"); err != nil {
		return taskpkg.RunStarvation{}, err
	}
	id, err := requireTaskValue(mutation.RunID, "task run id")
	if err != nil {
		return taskpkg.RunStarvation{}, err
	}
	if err := g.queries.UpsertRunStarvation(ctx, sqlcgen.UpsertRunStarvationParams{
		RunID: id, WakeCount: int64(mutation.WakeCount),
		FirstStarvedAt:   store.FormatTimestamp(mutation.FirstStarvedAt),
		LastWakeAt:       nullableStarvationTime(mutation.LastWakeAt),
		EscalationTier:   int64(mutation.EscalationTier),
		SpawnRequestedAt: nullableStarvationTimePtr(mutation.SpawnRequestedAt),
		StarvedEventAt:   nullableStarvationTimePtr(mutation.StarvedEventAt),
		UpdatedAt:        store.FormatTimestamp(mutation.UpdatedAt),
	}); err != nil {
		return taskpkg.RunStarvation{}, fmt.Errorf("store: upsert run starvation: %w", err)
	}
	return taskpkg.RunStarvation{
		RunID:            id,
		WakeCount:        mutation.WakeCount,
		FirstStarvedAt:   mutation.FirstStarvedAt,
		LastWakeAt:       mutation.LastWakeAt,
		EscalationTier:   mutation.EscalationTier,
		SpawnRequestedAt: cloneTimePointer(mutation.SpawnRequestedAt),
		StarvedEventAt:   cloneTimePointer(mutation.StarvedEventAt),
		UpdatedAt:        mutation.UpdatedAt,
	}, nil
}

// ClearRunStarvation removes a run's escalation budget once it leaves the starved set.
func (g *TaskRunRepo) ClearRunStarvation(ctx context.Context, runID string) error {
	if err := g.checkReady(ctx, "clear run starvation"); err != nil {
		return err
	}
	id, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return err
	}
	if err := g.queries.DeleteRunStarvation(ctx, id); err != nil {
		return fmt.Errorf("store: clear run starvation: %w", err)
	}
	return nil
}

func parseNullableStarvationTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := store.ParseTimestamp(value.String)
	if err != nil {
		return nil, fmt.Errorf("store: parse run starvation timestamp: %w", err)
	}
	return &parsed, nil
}

func nullableStarvationTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func nullableStarvationTimePtr(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return nullableStarvationTime(*value)
}
