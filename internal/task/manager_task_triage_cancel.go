package task

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"time"
)

// MarkTaskRead persists the current actor-scoped triage state as read for the
// task's latest known activity snapshot.
func (m *Service) MarkTaskRead(
	ctx context.Context,
	id string,
	actor ActorContext,
) (TriageState, error) {
	return m.mutateTaskTriage(ctx, id, actor, func(state *TriageState, latestActivity time.Time) {
		state.Read = true
		state.Dismissed = false
		state.LastSeenActivityAt = latestActivity
	})
}

// ArchiveTask persists the current actor-scoped triage state as archived for
// the task's latest known activity snapshot.
func (m *Service) ArchiveTask(
	ctx context.Context,
	id string,
	actor ActorContext,
) (TriageState, error) {
	return m.mutateTaskTriage(ctx, id, actor, func(state *TriageState, latestActivity time.Time) {
		state.Read = true
		state.Archived = true
		state.Dismissed = false
		state.LastSeenActivityAt = latestActivity
	})
}

// DismissTask persists the current actor-scoped triage state as dismissed for
// the task's latest known activity snapshot.
func (m *Service) DismissTask(
	ctx context.Context,
	id string,
	actor ActorContext,
) (TriageState, error) {
	return m.mutateTaskTriage(ctx, id, actor, func(state *TriageState, latestActivity time.Time) {
		state.Read = true
		state.Dismissed = true
		state.LastSeenActivityAt = latestActivity
	})
}

func (m *Service) mutateTaskTriage(
	ctx context.Context,
	id string,
	actor ActorContext,
	mutate func(state *TriageState, latestActivity time.Time),
) (TriageState, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return TriageState{}, err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return TriageState{}, fmt.Errorf("%w: task id is required", ErrValidation)
	}

	record, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return TriageState{}, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: trimmedID})
	if err != nil {
		return TriageState{}, err
	}
	events, err := m.store.ListTaskEvents(ctx, EventQuery{TaskID: trimmedID, Limit: 1})
	if err != nil {
		return TriageState{}, err
	}

	state, err := m.loadTaskTriageState(ctx, trimmedID, actor.Actor)
	if err != nil {
		return TriageState{}, err
	}
	mutate(&state, latestTaskActivityAt(record, runs, events))
	state.TaskID = trimmedID
	state.Actor = actor.Actor
	state.UpdatedAt = m.now().UTC()
	if err := m.store.UpsertTaskTriageState(ctx, state); err != nil {
		return TriageState{}, err
	}

	return state, nil
}

func (m *Service) loadTaskTriageState(
	ctx context.Context,
	taskID string,
	actor ActorIdentity,
) (TriageState, error) {
	state, err := m.store.GetTaskTriageState(ctx, taskID, actor)
	if err == nil {
		return state, nil
	}
	if errors.Is(err, ErrTaskTriageStateNotFound) {
		return TriageState{TaskID: taskID, Actor: actor}, nil
	}
	return TriageState{}, err
}

func (m *Service) loadCancellationTree(ctx context.Context, taskID string) ([]Task, Task, error) {
	tree, err := m.collectTaskTree(ctx, taskID)
	if err != nil {
		return nil, Task{}, err
	}
	if len(tree) == 0 {
		return nil, Task{}, ErrTaskNotFound
	}
	return tree, tree[0], nil
}

func (m *Service) authorizeTaskTreeMutation(
	ctx context.Context,
	actor ActorContext,
	tree []Task,
) error {
	for _, record := range tree {
		if err := m.authorizeTaskResource(ctx, actor, record); err != nil {
			return err
		}
		runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
		if err != nil {
			return err
		}
		for _, run := range runs {
			if err := m.authorizeRunResource(ctx, actor, run, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Service) ensureTaskCancelable(ctx context.Context, root Task) error {
	rootRuns, rootStatus, err := m.loadTaskRuntimeState(ctx, root)
	if err != nil {
		return err
	}
	if isTerminalTaskStatus(rootStatus) && rootStatus != TaskStatusCanceled &&
		!hasOpenRun(rootRuns) {
		return fmt.Errorf(
			"%w: task %q cannot transition from %q to %q",
			ErrInvalidStatusTransition,
			root.ID,
			rootStatus,
			TaskStatusCanceled,
		)
	}
	return nil
}

func (m *Service) cancelTaskTreeRecord(
	ctx context.Context,
	rootTaskID string,
	idx int,
	record Task,
	req CancelTask,
	actor ActorContext,
) (Task, error) {
	runs, status, err := m.loadTaskRuntimeState(ctx, record)
	if err != nil {
		return Task{}, err
	}
	record.Status = status
	if idx > 0 && isTerminalTaskStatus(status) {
		return record, nil
	}

	propagatedFromTaskID := cancellationPropagationRoot(rootTaskID, idx)
	cancelledRunIDs, err := m.cancelOpenTaskRuns(
		ctx,
		record,
		runs,
		req,
		actor,
		propagatedFromTaskID,
	)
	if err != nil {
		return Task{}, err
	}
	if status.Normalize() == TaskStatusCanceled && len(cancelledRunIDs) == 0 {
		return record, nil
	}
	return m.persistCancelledTask(ctx, record, req, actor, propagatedFromTaskID, cancelledRunIDs)
}

func (m *Service) loadTaskRuntimeState(
	ctx context.Context,
	record Task,
) ([]Run, Status, error) {
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
	if err != nil {
		return nil, "", err
	}
	dependencies, err := m.store.ListDependencies(ctx, record.ID)
	if err != nil {
		return nil, "", err
	}
	status, err := m.canonicalTaskStatus(ctx, record, dependencies, runs)
	if err != nil {
		return nil, "", err
	}
	return runs, status, nil
}

func cancellationPropagationRoot(rootTaskID string, idx int) string {
	if idx == 0 {
		return ""
	}
	return rootTaskID
}

func (m *Service) cancelOpenTaskRuns(
	ctx context.Context,
	record Task,
	runs []Run,
	req CancelTask,
	actor ActorContext,
	propagatedFromTaskID string,
) ([]string, error) {
	cancelledRunIDs := make([]string, 0)
	for _, run := range runs {
		if isTerminalRunStatus(run.Status) {
			continue
		}
		cancelledRun, err := m.cancelRunRecord(
			ctx,
			record,
			run,
			CancelRun(req),
			actor,
			cancelRunOptions{
				propagatedFromTaskID: propagatedFromTaskID,
				reconcileTask:        false,
			},
		)
		if err != nil {
			return nil, err
		}
		cancelledRunIDs = append(cancelledRunIDs, cancelledRun.ID)
	}
	return cancelledRunIDs, nil
}

func (m *Service) persistCancelledTask(
	ctx context.Context,
	record Task,
	req CancelTask,
	actor ActorContext,
	propagatedFromTaskID string,
	cancelledRunIDs []string,
) (Task, error) {
	previousStatus := record.Status
	record.Status = TaskStatusCanceled
	record.UpdatedAt = m.now().UTC()
	record.ClosedAt = record.UpdatedAt
	if err := m.store.UpdateTask(ctx, record, actor); err != nil {
		return Task{}, err
	}
	m.dispatchTaskStatusChangedAfterWrite(ctx, record, previousStatus, record.Status, actor)
	if err := m.recordTaskEvent(ctx, record.ID, "", taskEventCanceled, actor, cancelledTaskPayload{
		Reason:               req.Reason,
		Metadata:             cloneRawJSON(req.Metadata),
		Status:               record.Status,
		PropagatedFromTaskID: propagatedFromTaskID,
		CancelledRunIDs:      append([]string(nil), cancelledRunIDs...),
	}); err != nil {
		return Task{}, err
	}
	if err := m.reconcileDependentTasks(ctx, record.ID, map[string]struct{}{record.ID: {}}, actor); err != nil {
		return Task{}, err
	}
	return record, nil
}
