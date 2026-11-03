package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const instancePauseActorPrefix = "agent_instance:"

type InstanceManagedRun struct {
	ID, WorkspaceID, Status  string
	PauseRequested, Recorded bool
}

func (r *InstanceRepo) ManagedRuns(ctx context.Context, instanceID, after string) ([]InstanceManagedRun, error) {
	if err := r.checkReady(ctx, "list instance managed runs"); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListInstanceManagedRuns(ctx, sqlcgen.ListInstanceManagedRunsParams{
		InstanceID: instanceID, AfterID: after, PageLimit: 100,
	})
	if err != nil {
		return nil, err
	}
	result := make([]InstanceManagedRun, 0, len(rows))
	for _, row := range rows {
		result = append(result, InstanceManagedRun{ID: row.ID, WorkspaceID: row.WorkspaceID, Status: row.Status,
			PauseRequested: row.PauseRequested != 0, Recorded: row.Recorded})
	}
	return result, nil
}

func (r *InstanceRepo) ForgetSettledRuns(ctx context.Context, instanceID string) error {
	if err := r.checkReady(ctx, "forget settled instance runs"); err != nil {
		return err
	}
	return r.queries.ForgetSettledInstanceRuns(ctx, instanceID)
}

// RequireInstanceRunResumable is an early read guard; the canonical mutations repeat it in their transaction.
func (g *LoopRepo) RequireInstanceRunResumable(ctx context.Context, runID looppkg.RunID) error {
	if err := g.checkReady(ctx, "check instance run resume"); err != nil {
		return err
	}
	return requireInstanceRunResumable(ctx, g.queries, string(runID))
}

func requireInstanceRunResumable(ctx context.Context, q *sqlcgen.Queries, runID string) error {
	id, err := q.GetPausedInstanceForLoopRun(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &looppkg.ReasonError{
		Code: "agent_instance_paused",
		Err:  fmt.Errorf("%w: resume agent instance %s first", looppkg.ErrTransitionConflict, id),
		Meta: map[string]string{"instance_id": id},
	}
}

func releaseInstanceRunPause(ctx context.Context, q *sqlcgen.Queries, runID string) error {
	if err := requireInstanceRunResumable(ctx, q, runID); err != nil {
		return err
	}
	return q.DeleteInstancePausedRun(ctx, runID)
}

// The record shares the pause transaction: an operator's earlier pause can never be claimed by the instance.
func recordInstanceRunPause(
	ctx context.Context,
	q *sqlcgen.Queries,
	runID string,
	actor taskpkg.ActorContext,
	now time.Time,
) error {
	if actor.Actor.Kind.Normalize() != taskpkg.ActorKindDaemon {
		return nil
	}
	id, ok := strings.CutPrefix(actor.Actor.Ref, instancePauseActorPrefix)
	if !ok {
		return nil
	}
	pausedID, err := q.GetPausedInstanceForLoopRun(ctx, runID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && pausedID != id) {
		return fmt.Errorf("%w: instance pause changed", looppkg.ErrTransitionConflict)
	}
	if err != nil {
		return err
	}
	return q.RecordInstancePausedRun(ctx, sqlcgen.RecordInstancePausedRunParams{
		InstanceID: id, RunID: runID, PausedAt: store.FormatTimestamp(now),
	})
}

func (r *InstanceRepo) QueuedSessions(ctx context.Context, instanceID, after string) ([]string, error) {
	if err := r.checkReady(ctx, "list instance queued sessions"); err != nil {
		return nil, err
	}
	return r.queries.ListInstanceQueuedSessions(
		ctx,
		sqlcgen.ListInstanceQueuedSessionsParams{InstanceID: instanceID, AfterID: after},
	)
}
