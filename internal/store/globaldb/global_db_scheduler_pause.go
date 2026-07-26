package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

// GetSchedulerPauseState returns the singleton scheduler pause state for inspect diagnostics.
func (g *TaskRepo) GetSchedulerPauseState(ctx context.Context) (taskpkg.InspectSchedulerState, error) {
	state, err := g.GetSchedulerPause(ctx)
	if err != nil {
		return taskpkg.InspectSchedulerState{}, err
	}
	return taskpkg.InspectSchedulerState(state), nil
}

// GetSchedulerPause returns the singleton scheduler pause state.
func (g *TaskRepo) GetSchedulerPause(ctx context.Context) (taskpkg.SchedulerPauseState, error) {
	if err := g.checkReady(ctx, "get scheduler pause state"); err != nil {
		return taskpkg.SchedulerPauseState{}, err
	}
	row, err := g.queries.GetSchedulerPause(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return taskpkg.SchedulerPauseState{}, nil
	}
	if err != nil {
		return taskpkg.SchedulerPauseState{}, fmt.Errorf("store: get scheduler pause state: %w", err)
	}
	return schedulerPauseStateFromSQLC(row), nil
}

// SetSchedulerPaused marks the scheduler-wide pause singleton as paused.
func (g *TaskRepo) SetSchedulerPaused(
	ctx context.Context,
	actor string,
	reason string,
) (taskpkg.SchedulerPauseState, error) {
	if err := g.checkReady(ctx, "pause scheduler"); err != nil {
		return taskpkg.SchedulerPauseState{}, err
	}
	now := g.now().UTC()
	if err := g.queries.SetSchedulerPaused(ctx, sqlcgen.SetSchedulerPausedParams{
		PausedBy:  strings.TrimSpace(actor),
		PausedAt:  sql.NullString{String: store.FormatTimestamp(now), Valid: true},
		Reason:    strings.TrimSpace(reason),
		UpdatedAt: store.FormatTimestamp(now),
	}); err != nil {
		return taskpkg.SchedulerPauseState{}, fmt.Errorf("store: pause scheduler: %w", err)
	}
	return g.GetSchedulerPause(ctx)
}

// SetSchedulerResumed clears the scheduler-wide pause singleton.
func (g *TaskRepo) SetSchedulerResumed(ctx context.Context) (taskpkg.SchedulerPauseState, error) {
	if err := g.checkReady(ctx, "resume scheduler"); err != nil {
		return taskpkg.SchedulerPauseState{}, err
	}
	now := g.now().UTC()
	if err := g.queries.SetSchedulerResumed(ctx, store.FormatTimestamp(now)); err != nil {
		return taskpkg.SchedulerPauseState{}, fmt.Errorf("store: resume scheduler: %w", err)
	}
	return g.GetSchedulerPause(ctx)
}

func schedulerPauseStateFromSQLC(row sqlcgen.GetSchedulerPauseRow) taskpkg.SchedulerPauseState {
	state := taskpkg.SchedulerPauseState{
		Paused:   row.Paused != 0,
		PausedBy: strings.TrimSpace(row.PausedBy),
		Reason:   strings.TrimSpace(row.Reason),
	}
	// paused_at and updated_at are observability fields; `paused` is the authoritative
	// signal. A non-canonical timestamp (e.g. a SQLite CURRENT_TIMESTAMP default) must
	// never disable the scheduler control plane, so both are parsed best-effort.
	if row.PausedAt.Valid {
		if pausedAt, parseErr := store.ParseNullableTimestamp(row.PausedAt.String); parseErr == nil && pausedAt != nil {
			state.PausedAt = *pausedAt
		}
	}
	if updatedAt, parseErr := store.ParseTimestamp(row.UpdatedAt); parseErr == nil {
		state.UpdatedAt = updatedAt
	}
	return state
}
