package task

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// PublishTask transitions one durable draft into manager-owned runnable reconciliation,
// then enqueues the executable run for the explicit execution boundary.
func (m *Service) PublishTask(
	ctx context.Context,
	id string,
	req ExecutionRequest,
	actor ActorContext,
) (*Execution, error) {
	return m.executeTaskBoundary(ctx, id, req, ExecutionActionPublish, actor)
}

// StartTask enqueues one executable run for an already-created task.
func (m *Service) StartTask(
	ctx context.Context,
	id string,
	req ExecutionRequest,
	actor ActorContext,
) (*Execution, error) {
	return m.executeTaskBoundary(ctx, id, req, ExecutionActionStart, actor)
}

// ApproveTask records one approval decision for a manual-approval task that is
// currently awaiting a decision, then enqueues the approved run.
func (m *Service) ApproveTask(
	ctx context.Context,
	id string,
	req ExecutionRequest,
	actor ActorContext,
) (*Execution, error) {
	return m.executeTaskBoundary(ctx, id, req, ExecutionActionApproval, actor)
}

// RejectTask records one rejection decision for a manual-approval task that is
// currently awaiting a decision and reconciles the resulting task status.
func (m *Service) RejectTask(ctx context.Context, id string, actor ActorContext) (*Task, error) {
	return m.transitionTaskApproval(ctx, id, ApprovalStateRejected, taskEventRejected, actor)
}

func (m *Service) executeTaskBoundary(
	ctx context.Context,
	id string,
	req ExecutionRequest,
	action ExecutionAction,
	actor ActorContext,
) (*Execution, error) {
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: task id is required", ErrValidation)
	}
	if _, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor); err != nil {
		return nil, err
	}
	normalizedReq, err := normalizeTaskExecutionRequest(req)
	if err != nil {
		return nil, err
	}
	idempotencyKey := taskExecutionIdempotencyKey(trimmedID, action, normalizedReq.IdempotencyKey)
	var result taskExecutionCommandResult
	command := func(store ExecutionMutationStore) error {
		var commandErr error
		result, commandErr = m.executeTaskBoundaryWithStore(
			ctx,
			store,
			trimmedID,
			normalizedReq,
			idempotencyKey,
			action,
			actor,
		)
		return commandErr
	}
	if transactions, ok := m.store.(ExecutionTransactionStore); ok {
		if err := transactions.WithTaskExecutionTransaction(ctx, command); err != nil {
			return nil, err
		}
	} else if err := command(m.store); err != nil {
		return nil, err
	}
	m.publishTaskEventsAfterCommand(ctx, result.events)
	m.observeCommittedRunParticipation(ctx, result.participationObservation)
	if result.enqueued {
		m.dispatchTaskRunEnqueued(
			ctx,
			result.execution.Run,
			result.execution.Task,
			actor,
			result.execution.Run.IdempotencyKey,
		)
	}
	return &result.execution, nil
}

type taskExecutionCommandResult struct {
	execution                Execution
	events                   []Event
	participationObservation *participation.ResolvedObservation
	enqueued                 bool
}

func (m *Service) executeTaskBoundaryWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	taskID string,
	req ExecutionRequest,
	idempotencyKey string,
	action ExecutionAction,
	actor ActorContext,
) (taskExecutionCommandResult, error) {
	if existing, ok, err := m.taskExecutionFromIdempotencyWithStore(
		ctx,
		store,
		taskID,
		idempotencyKey,
		action,
		actor,
	); err != nil {
		return taskExecutionCommandResult{}, err
	} else if ok {
		return taskExecutionCommandResult{execution: *existing}, nil
	}

	approvalTask, boundaryEvents, err := m.prepareTaskExecutionBoundaryWithStore(
		ctx,
		store,
		taskID,
		action,
		actor,
	)
	if err != nil {
		return taskExecutionCommandResult{}, err
	}
	if execution, ok, err := m.approvalExistingRunExecutionWithStore(ctx, store, approvalTask); err != nil {
		return taskExecutionCommandResult{}, err
	} else if ok {
		return taskExecutionCommandResult{execution: *execution, events: boundaryEvents}, nil
	}
	if result, ok, err := m.approvalAutoEnqueueExecutionWithStore(
		ctx,
		store,
		approvalTask,
		req,
		idempotencyKey,
		actor,
	); err != nil {
		return taskExecutionCommandResult{}, err
	} else if ok {
		result.events = append(boundaryEvents, result.events...)
		return result, nil
	}

	enqueued, err := m.enqueueRunWithStore(ctx, store, EnqueueRun{
		TaskID:               taskID,
		IdempotencyKey:       idempotencyKey,
		NetworkParticipation: req.NetworkParticipation,
		Metadata:             req.Metadata,
	}, actor)
	if err != nil {
		return taskExecutionCommandResult{}, err
	}
	return taskExecutionCommandResult{
		execution: Execution{
			Task:        enqueued.task,
			Run:         enqueued.run,
			Action:      action,
			ExistingRun: enqueued.existing,
		},
		events:                   append(boundaryEvents, enqueued.event),
		participationObservation: enqueued.participationObservation,
		enqueued:                 !enqueued.existing,
	}, nil
}

func (m *Service) approvalExistingRunExecutionWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	approvalTask *Task,
) (*Execution, bool, error) {
	if approvalTask == nil {
		return nil, false, nil
	}
	runs, err := store.ListTaskRuns(ctx, RunQuery{TaskID: approvalTask.ID})
	if err != nil {
		return nil, false, fmt.Errorf(
			"task: list runs while approving task %q: %w",
			approvalTask.ID,
			err,
		)
	}
	var openRun *Run
	for idx := range runs {
		if isTerminalRunStatus(runs[idx].Status) {
			continue
		}
		if openRun != nil {
			return nil, false, fmt.Errorf(
				"%w: task %q has multiple open runs %q and %q",
				ErrConflict,
				approvalTask.ID,
				openRun.ID,
				runs[idx].ID,
			)
		}
		openRun = &runs[idx]
	}
	if openRun == nil {
		return nil, false, nil
	}
	if strings.TrimSpace(openRun.TaskID) != approvalTask.ID {
		return nil, false, fmt.Errorf(
			"%w: task %q current run %q belongs to task %q",
			ErrValidation,
			approvalTask.ID,
			openRun.ID,
			openRun.TaskID,
		)
	}
	taskRecord, err := store.GetTask(ctx, approvalTask.ID)
	if err != nil {
		return nil, false, err
	}
	return &Execution{
		Task:        taskRecord,
		Run:         *openRun,
		Action:      ExecutionActionApproval,
		ExistingRun: true,
	}, true, nil
}

func (m *Service) prepareTaskExecutionBoundaryWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	taskID string,
	action ExecutionAction,
	actor ActorContext,
) (*Task, []Event, error) {
	switch action {
	case ExecutionActionPublish:
		_, events, err := m.publishTaskIntentWithStore(ctx, store, taskID, actor)
		return nil, events, err
	case ExecutionActionApproval:
		record, event, err := m.transitionTaskApprovalWithStore(
			ctx,
			store,
			taskID,
			ApprovalStateApproved,
			taskEventApproved,
			actor,
		)
		if err != nil {
			return nil, nil, err
		}
		return record, []Event{event}, nil
	case ExecutionActionStart:
		taskRecord, err := store.GetTask(ctx, taskID)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, m.ensureTaskExecutableWithStore(ctx, store, taskRecord)
	default:
		return nil, nil, fmt.Errorf("%w: unsupported task execution action %q", ErrValidation, action)
	}
}

func (m *Service) approvalAutoEnqueueExecutionWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	approvalTask *Task,
	req ExecutionRequest,
	executionIdempotencyKey string,
	actor ActorContext,
) (taskExecutionCommandResult, bool, error) {
	if approvalTask == nil ||
		!approvalTask.AutoEnqueueOnReady ||
		approvalTask.Status.Normalize() != TaskStatusReady {
		return taskExecutionCommandResult{}, false, nil
	}
	trigger := autoEnqueueTrigger{
		Kind: autoEnqueueTriggerApprovalGranted,
		Ref:  approvalTask.ID,
	}
	enqueued, err := m.enqueueRunWithStore(ctx, store, EnqueueRun{
		TaskID:               approvalTask.ID,
		IdempotencyKey:       trigger.idempotencyKey(approvalTask.ID),
		NetworkParticipation: req.NetworkParticipation,
		Metadata:             req.Metadata,
	}, actor)
	if err != nil {
		return taskExecutionCommandResult{}, false, err
	}
	if err := m.saveApprovalIdempotencyAliasWithStore(
		ctx,
		store,
		approvalTask.ID,
		enqueued.run.ID,
		executionIdempotencyKey,
		actor,
	); err != nil {
		return taskExecutionCommandResult{}, false, err
	}
	taskRecord, err := store.GetTask(ctx, enqueued.run.TaskID)
	if err != nil {
		return taskExecutionCommandResult{}, false, err
	}
	autoEvent, err := m.newTaskEvent(
		approvalTask.ID,
		enqueued.run.ID,
		taskEventAutoEnqueueTriggered,
		actor,
		autoEnqueueTriggeredPayload{
			Status:      approvalTask.Status,
			RunStatus:   enqueued.run.Status,
			TriggerKind: trigger.Kind,
			TriggerRef:  trigger.Ref,
		},
	)
	if err != nil {
		return taskExecutionCommandResult{}, false, err
	}
	if err := store.CreateTaskEvent(ctx, autoEvent); err != nil {
		return taskExecutionCommandResult{}, false, err
	}
	events := []Event{enqueued.event, autoEvent}
	if enqueued.existing {
		events = []Event{autoEvent}
	}
	return taskExecutionCommandResult{
		execution: Execution{
			Task:        taskRecord,
			Run:         enqueued.run,
			Action:      ExecutionActionApproval,
			ExistingRun: enqueued.existing,
		},
		events:                   events,
		participationObservation: enqueued.participationObservation,
		enqueued:                 !enqueued.existing,
	}, true, nil
}

func (m *Service) saveApprovalIdempotencyAliasWithStore(
	ctx context.Context,
	store IdempotencyStore,
	taskID string,
	runID string,
	idempotencyKey string,
	actor ActorContext,
) error {
	if err := store.SaveTaskRunIdempotency(ctx, RunIdempotency{
		IdempotencyKey: idempotencyKey,
		RunID:          runID,
		Origin:         actor.Origin,
		CreatedAt:      m.now().UTC(),
	}); err != nil {
		return fmt.Errorf(
			"task: save approval idempotency alias for task %q run %q: %w",
			taskID,
			runID,
			err,
		)
	}
	return nil
}

func (m *Service) taskExecutionFromIdempotencyWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	taskID string,
	idempotencyKey string,
	action ExecutionAction,
	actor ActorContext,
) (*Execution, bool, error) {
	run, err := store.GetTaskRunByIdempotencyKey(ctx, idempotencyKey, actor.Origin)
	switch {
	case errors.Is(err, ErrTaskRunIdempotencyNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	if strings.TrimSpace(run.TaskID) != taskID {
		return nil, false, fmt.Errorf(
			"%w: idempotency key %q is already bound to task %q",
			ErrValidation,
			idempotencyKey,
			run.TaskID,
		)
	}
	taskRecord, err := store.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, false, err
	}
	return &Execution{
		Task:        taskRecord,
		Run:         run,
		Action:      action,
		ExistingRun: true,
	}, true, nil
}

func taskApprovalDecisionAllowed(record Task, target ApprovalState) bool {
	normalizedPolicy := normalizeApprovalPolicyOrDefault(record.ApprovalPolicy)
	if normalizedPolicy != ApprovalPolicyManual {
		return false
	}
	switch target.Normalize() {
	case ApprovalStateApproved, ApprovalStateRejected:
	default:
		return false
	}
	return normalizeApprovalStateOrDefault(
		normalizedPolicy,
		record.ApprovalState,
	) == ApprovalStatePending
}
