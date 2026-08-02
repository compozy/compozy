package task

import (
	"context"
	"fmt"
	"strings"
)

type cancelRunOptions struct {
	propagatedFromTaskID string
	reconcileTask        bool
}

func (m *Service) cancelRunRecord(
	ctx context.Context,
	taskRecord Task,
	run Run,
	req CancelRun,
	actor ActorContext,
	opts cancelRunOptions,
) (*Run, error) {
	command, err := m.prepareCancellationTerminalRunCommand(taskRecord, run, req, actor, opts)
	if err != nil {
		return nil, err
	}
	return m.executeCancellationTerminalRunCommand(ctx, command)
}

func (m *Service) prepareCancellationTerminalRunCommand(
	taskRecord Task,
	run Run,
	req CancelRun,
	actor ActorContext,
	opts cancelRunOptions,
) (TerminalRunCommand, error) {
	status := run.Status.Normalize()
	switch status {
	case TaskRunStatusQueued, TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
	default:
		return TerminalRunCommand{}, requireRunTransition(run, TaskRunStatusCanceled)
	}
	normalizedReq, err := normalizeCancelRun(req)
	if err != nil {
		return TerminalRunCommand{}, err
	}
	activeSession := (status == TaskRunStatusStarting || status == TaskRunStatusRunning) &&
		strings.TrimSpace(run.SessionID) != ""
	if activeSession {
		if err := m.requireSessionExecutor("cancel active run"); err != nil {
			return TerminalRunCommand{}, err
		}
	}
	if err := m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunCanceled,
		actor,
		cancelledRunPayload{
			Status: TaskRunStatusCanceled, TaskStatus: taskRecord.Status,
			Reason: normalizedReq.Reason, Metadata: cloneRawJSON(normalizedReq.Metadata),
			SessionID: run.SessionID, PropagatedFromTaskID: opts.propagatedFromTaskID,
			CooperativeStopRequested: activeSession,
		},
	); err != nil {
		return TerminalRunCommand{}, err
	}
	if activeSession {
		if err := m.preflightTaskEvent(
			run.TaskID,
			run.ID,
			taskEventRunForceStopped,
			actor,
			forceStoppedRunPayload{
				SessionID: run.SessionID, GraceTimeoutMillis: m.cancelGracePeriod.Milliseconds(),
				PropagatedFromTaskID: opts.propagatedFromTaskID,
			},
		); err != nil {
			return TerminalRunCommand{}, err
		}
	}
	commandID, err := m.newID("terminal-command")
	if err != nil {
		return TerminalRunCommand{}, fmt.Errorf("task: generate terminal command id: %w", err)
	}
	eventCount := 1
	if activeSession {
		eventCount++
	}
	eventIDs, err := m.reserveTaskEventIDs(eventCount)
	if err != nil {
		return TerminalRunCommand{}, err
	}
	command, err := newCancellationTerminalRunCommand(
		commandID,
		eventIDs,
		run,
		normalizedReq,
		actor,
		m.now().UTC(),
		opts,
	)
	if err != nil {
		return TerminalRunCommand{}, err
	}
	return command, nil
}

func (m *Service) executeCancellationTerminalRunCommand(
	ctx context.Context,
	command TerminalRunCommand,
) (*Run, error) {
	settlement, err := m.executeTerminalRunCommand(ctx, command)
	if err != nil {
		return nil, err
	}
	if err := m.publishTerminalRunCommandSettlement(ctx, &settlement); err != nil {
		return &settlement.run, err
	}
	return &settlement.run, nil
}

func (m *Service) persistCanceledRun(
	ctx context.Context,
	store runMutationStore,
	mutation TerminalRunMutation,
	req CancelRun,
	actor ActorContext,
	opts cancelRunOptions,
	activeSession bool,
	cooperativeStopRequested bool,
	eventIDs []string,
) (taskRunMutationSettlement, error) {
	settledRun, err := store.TransitionTerminalRun(ctx, mutation)
	if err != nil {
		return taskRunMutationSettlement{}, err
	}
	reconciledTask, transitions, err := m.reconcileCanceledRunTask(ctx, store, &settledRun, actor, opts)
	if err != nil {
		return taskRunMutationSettlement{}, err
	}
	events, err := m.createCanceledRunEvents(
		ctx,
		store,
		&settledRun,
		&reconciledTask,
		req,
		actor,
		opts,
		activeSession,
		cooperativeStopRequested,
		eventIDs,
	)
	if err != nil {
		return taskRunMutationSettlement{}, err
	}
	return taskRunMutationSettlement{
		Run: settledRun, Task: reconciledTask, StatusTransitions: transitions, Events: events,
	}, nil
}

func (m *Service) reconcileCanceledRunTask(
	ctx context.Context,
	store runMutationStore,
	run *Run,
	actor ActorContext,
	opts cancelRunOptions,
) (Task, []StatusTransition, error) {
	if opts.reconcileTask {
		return m.reconcileTaskSettlementCascadeWithStore(
			ctx,
			store,
			run.TaskID,
			actor,
			run.EndedAt,
		)
	}
	taskRecord, err := store.GetTask(ctx, run.TaskID)
	return taskRecord, nil, err
}

func (m *Service) createCanceledRunEvents(
	ctx context.Context,
	store runMutationStore,
	run *Run,
	taskRecord *Task,
	req CancelRun,
	actor ActorContext,
	opts cancelRunOptions,
	activeSession bool,
	cooperativeStopRequested bool,
	eventIDs []string,
) ([]Event, error) {
	if len(eventIDs) == 0 {
		return nil, fmt.Errorf("%w: canceled run event id is required", ErrValidation)
	}
	canceledEvent, err := m.newTaskEventWithIDAt(
		eventIDs[0],
		run.TaskID,
		run.ID,
		taskEventRunCanceled,
		actor,
		run.EndedAt,
		cancelledRunPayload{
			Status:                   run.Status,
			TaskStatus:               taskRecord.Status,
			Reason:                   req.Reason,
			Metadata:                 cloneRawJSON(req.Metadata),
			SessionID:                run.SessionID,
			PropagatedFromTaskID:     opts.propagatedFromTaskID,
			CooperativeStopRequested: cooperativeStopRequested,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := store.CreateTaskEvent(ctx, canceledEvent); err != nil {
		return nil, err
	}
	events := []Event{canceledEvent}
	if !activeSession {
		return events, nil
	}
	if len(eventIDs) < 2 {
		return nil, fmt.Errorf("%w: force-stopped run event id is required", ErrValidation)
	}
	forceStoppedEvent, err := m.newTaskEventWithIDAt(
		eventIDs[1],
		run.TaskID,
		run.ID,
		taskEventRunForceStopped,
		actor,
		run.EndedAt,
		forceStoppedRunPayload{
			SessionID:            run.SessionID,
			GraceTimeoutMillis:   m.cancelGracePeriod.Milliseconds(),
			PropagatedFromTaskID: opts.propagatedFromTaskID,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := store.CreateTaskEvent(ctx, forceStoppedEvent); err != nil {
		return nil, err
	}
	return append(events, forceStoppedEvent), nil
}
