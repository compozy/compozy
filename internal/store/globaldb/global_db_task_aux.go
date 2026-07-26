package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

var _ taskpkg.DependencyStore = (*TaskRepo)(nil)
var _ taskpkg.EventStore = (*TaskRepo)(nil)
var _ taskpkg.EventSequenceStore = (*TaskRepo)(nil)
var _ taskpkg.IdempotencyStore = (*TaskRepo)(nil)
var _ taskpkg.TriageStore = (*TaskRepo)(nil)

type taskSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type queuedRunReservationInput struct {
	taskID             string
	runID              string
	runKind            taskpkg.RunKind
	loopRunID          string
	idempotencyKey     string
	origin             taskpkg.Origin
	networkSpec        participation.Spec
	designationGroupID string
	metadata           json.RawMessage
	queuedAt           time.Time
}

// GetTaskTriageState returns the durable actor-scoped triage state for one task.
func (g *TaskRepo) GetTaskTriageState(
	ctx context.Context,
	taskID string,
	actor taskpkg.ActorIdentity,
) (taskpkg.TriageState, error) {
	if err := g.checkReady(ctx, "get task triage state"); err != nil {
		return taskpkg.TriageState{}, err
	}

	trimmedTaskID, normalizedActor, err := normalizeTaskTriageLookup(taskID, actor)
	if err != nil {
		return taskpkg.TriageState{}, err
	}

	row, err := g.queries.GetTaskTriageState(ctx, sqlcgen.GetTaskTriageStateParams{
		TaskID: trimmedTaskID, ActorKind: string(normalizedActor.Kind), ActorID: normalizedActor.Ref,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.TriageState{}, taskpkg.ErrTaskTriageStateNotFound
		}
		return taskpkg.TriageState{}, err
	}
	return taskTriageFromGenerated(row)
}

// UpsertTaskTriageState inserts or replaces one durable actor-scoped triage state.
func (g *TaskRepo) UpsertTaskTriageState(ctx context.Context, state taskpkg.TriageState) error {
	if err := g.checkReady(ctx, "upsert task triage state"); err != nil {
		return err
	}

	normalized, err := g.normalizeTaskTriageStateForUpsert(state)
	if err != nil {
		return err
	}
	if err := g.ensureTaskExists(ctx, normalized.TaskID); err != nil {
		return err
	}

	err = g.queries.UpsertTaskTriageState(ctx, sqlcgen.UpsertTaskTriageStateParams{
		TaskID:             normalized.TaskID,
		ActorKind:          string(normalized.Actor.Kind),
		ActorID:            normalized.Actor.Ref,
		IsRead:             normalized.Read,
		Archived:           normalized.Archived,
		Dismissed:          normalized.Dismissed,
		LastSeenActivityAt: nullableTaskTime(normalized.LastSeenActivityAt),
		UpdatedAt:          store.FormatTimestamp(normalized.UpdatedAt),
	})
	if err != nil {
		return fmt.Errorf(
			"store: upsert task triage state for task %q actor %q/%q: %w",
			normalized.TaskID,
			normalized.Actor.Kind,
			normalized.Actor.Ref,
			err,
		)
	}

	return nil
}

// ListTaskTriageStates returns all durable triage states persisted for one actor.
func (g *TaskRepo) ListTaskTriageStates(
	ctx context.Context,
	actor taskpkg.ActorIdentity,
) ([]taskpkg.TriageState, error) {
	if err := g.checkReady(ctx, "list task triage states"); err != nil {
		return nil, err
	}

	normalizedActor, err := normalizeTaskTriageActor(actor)
	if err != nil {
		return nil, err
	}

	rows, err := g.queries.ListTaskTriageStates(ctx, sqlcgen.ListTaskTriageStatesParams{
		ActorKind: string(normalizedActor.Kind), ActorID: normalizedActor.Ref,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"store: query task triage states for actor %q/%q: %w",
			normalizedActor.Kind,
			normalizedActor.Ref,
			err,
		)
	}

	states := make([]taskpkg.TriageState, 0, len(rows))
	for _, row := range rows {
		record, mapErr := taskTriageFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		states = append(states, record)
	}
	return states, nil
}

// CreateDependency inserts one durable task-dependency edge under a single SQLite write lock.
func (g *TaskRepo) CreateDependency(ctx context.Context, dependency taskpkg.Dependency) error {
	if err := g.checkReady(ctx, "create task dependency"); err != nil {
		return err
	}

	normalized, err := g.normalizeTaskDependencyForCreate(dependency)
	if err != nil {
		return err
	}

	return g.withTaskImmediateTransaction(ctx, "create task dependency", func(exec taskSQLExecutor) error {
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, normalized.TaskID); err != nil {
			return err
		}
		if err := g.ensureTaskExistsWithExecutor(ctx, exec, normalized.DependsOnTaskID); err != nil {
			return err
		}

		exists, err := g.taskDependencyExists(ctx, exec, normalized)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf(
				"%w: dependency edge %q -> %q already exists",
				taskpkg.ErrValidation,
				normalized.TaskID,
				normalized.DependsOnTaskID,
			)
		}

		count, err := g.countDependenciesWithExecutor(ctx, exec, normalized.TaskID)
		if err != nil {
			return err
		}
		if err := taskpkg.ValidateDependencyCount(count + 1); err != nil {
			return err
		}

		hasPath, err := g.hasDependencyPathWithExecutor(ctx, exec, normalized.DependsOnTaskID, normalized.TaskID)
		if err != nil {
			return err
		}
		if hasPath {
			return fmt.Errorf(
				"%w: adding dependency %q -> %q would create a cycle",
				taskpkg.ErrCycleDetected,
				normalized.TaskID,
				normalized.DependsOnTaskID,
			)
		}

		if err := sqlcgen.New(exec).InsertTaskDependency(ctx, sqlcgen.InsertTaskDependencyParams{
			TaskID: normalized.TaskID, DependsOnTaskID: normalized.DependsOnTaskID,
			Kind: string(normalized.Kind), CreatedAt: store.FormatTimestamp(normalized.CreatedAt),
		}); err != nil {
			return fmt.Errorf(
				"store: create task dependency %q -> %q: %w",
				normalized.TaskID,
				normalized.DependsOnTaskID,
				err,
			)
		}

		return nil
	})
}

// DeleteDependency removes one persisted dependency edge.
func (g *TaskRepo) DeleteDependency(ctx context.Context, taskID string, dependsOnID string) error {
	if err := g.checkReady(ctx, "delete task dependency"); err != nil {
		return err
	}

	trimmedTaskID, err := requireTaskValue(taskID, "task dependency task id")
	if err != nil {
		return err
	}
	trimmedDependsOnID, err := requireTaskValue(dependsOnID, "task dependency depends_on_task_id")
	if err != nil {
		return err
	}

	affected, err := g.queries.DeleteTaskDependency(ctx, sqlcgen.DeleteTaskDependencyParams{
		TaskID: trimmedTaskID, DependsOnTaskID: trimmedDependsOnID,
	})
	if err != nil {
		return fmt.Errorf(
			"store: delete task dependency %q -> %q: %w",
			trimmedTaskID,
			trimmedDependsOnID,
			err,
		)
	}

	if affected == 0 {
		return fmt.Errorf(
			"%w: task dependency %q",
			taskpkg.ErrTaskDependencyNotFound,
			trimmedTaskID+"->"+trimmedDependsOnID,
		)
	}
	return nil
}

// ListDependencies returns the persisted dependency edges for one task.
func (g *TaskRepo) ListDependencies(ctx context.Context, taskID string) ([]taskpkg.Dependency, error) {
	if err := g.checkReady(ctx, "list task dependencies"); err != nil {
		return nil, err
	}

	return g.listDependenciesWithExecutor(ctx, g.db, taskID)
}

func (g *TaskRepo) listDependenciesWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) ([]taskpkg.Dependency, error) {
	trimmedTaskID, err := requireTaskValue(taskID, "task dependency task id")
	if err != nil {
		return nil, err
	}

	rows, err := sqlcgen.New(exec).ListTaskDependencies(ctx, trimmedTaskID)
	if err != nil {
		return nil, fmt.Errorf("store: query task dependencies for %q: %w", trimmedTaskID, err)
	}

	dependencies := make([]taskpkg.Dependency, 0, len(rows))
	for _, row := range rows {
		record, mapErr := taskDependencyFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		dependencies = append(dependencies, record)
	}
	return dependencies, nil
}

// ListDependents returns persisted dependency edges that point at one task.
func (g *TaskRepo) ListDependents(ctx context.Context, dependsOnTaskID string) ([]taskpkg.Dependency, error) {
	if err := g.checkReady(ctx, "list task dependents"); err != nil {
		return nil, err
	}

	return g.listDependentsWithExecutor(ctx, g.db, dependsOnTaskID)
}

func (g *TaskRepo) listDependentsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	dependsOnTaskID string,
) ([]taskpkg.Dependency, error) {
	trimmedDependsOnID, err := requireTaskValue(dependsOnTaskID, "task dependent depends_on_task_id")
	if err != nil {
		return nil, err
	}

	rows, err := sqlcgen.New(exec).ListTaskDependents(ctx, trimmedDependsOnID)
	if err != nil {
		return nil, fmt.Errorf("store: query task dependents for %q: %w", trimmedDependsOnID, err)
	}

	dependents := make([]taskpkg.Dependency, 0, len(rows))
	for _, row := range rows {
		record, mapErr := taskDependencyFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		dependents = append(dependents, record)
	}
	return dependents, nil
}

// CountDependencies reports how many dependency edges are stored for one task.
func (g *TaskRepo) CountDependencies(ctx context.Context, taskID string) (int, error) {
	if err := g.checkReady(ctx, "count task dependencies"); err != nil {
		return 0, err
	}

	trimmedTaskID, err := requireTaskValue(taskID, "task dependency task id")
	if err != nil {
		return 0, err
	}

	return g.countDependenciesWithExecutor(ctx, g.db, trimmedTaskID)
}

// HasDependencyPath reports whether the dependency graph already contains a path from one task to another.
func (g *TaskRepo) HasDependencyPath(ctx context.Context, fromTaskID string, toTaskID string) (bool, error) {
	if err := g.checkReady(ctx, "check task dependency path"); err != nil {
		return false, err
	}

	trimmedFromTaskID, err := requireTaskValue(fromTaskID, "task dependency path from_task_id")
	if err != nil {
		return false, err
	}
	trimmedToTaskID, err := requireTaskValue(toTaskID, "task dependency path to_task_id")
	if err != nil {
		return false, err
	}

	return g.hasDependencyPathWithExecutor(ctx, g.db, trimmedFromTaskID, trimmedToTaskID)
}
