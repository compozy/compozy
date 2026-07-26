package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// EnqueueRun persists one new queue-first task run under manager authority.
func (m *Service) EnqueueRun(
	ctx context.Context,
	spec EnqueueRun,
	actor ActorContext,
) (*Run, error) {
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}
	return m.enqueueRun(ctx, spec, actor)
}

func (m *Service) enqueueRun(
	ctx context.Context,
	spec EnqueueRun,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	normalizedSpec, err := normalizeEnqueueRunSpec(spec)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleIdempotency(actor, normalizedSpec.IdempotencyKey, "enqueue_run"); err != nil {
		return nil, err
	}
	var result enqueueRunCommandResult
	command := func(store ExecutionMutationStore) error {
		var commandErr error
		result, commandErr = m.enqueueRunWithStore(ctx, store, normalizedSpec, actor)
		return commandErr
	}
	if transactions, ok := m.store.(ExecutionTransactionStore); ok {
		if err := transactions.WithTaskExecutionTransaction(ctx, command); err != nil {
			return nil, err
		}
	} else if err := command(m.store); err != nil {
		return nil, err
	}
	if result.existing {
		return &result.run, nil
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{result.event})
	m.observeCommittedRunParticipation(ctx, result.participationObservation)
	m.dispatchTaskRunEnqueued(
		ctx,
		result.run,
		result.task,
		actor,
		normalizedSpec.IdempotencyKey,
	)
	return &result.run, nil
}

type enqueueRunCommandResult struct {
	task                     Task
	run                      Run
	event                    Event
	participationObservation *participation.ResolvedObservation
	existing                 bool
}

func (m *Service) enqueueRunWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	spec EnqueueRun,
	actor ActorContext,
) (enqueueRunCommandResult, error) {
	normalizedSpec, err := normalizeEnqueueRunSpec(spec)
	if err != nil {
		return enqueueRunCommandResult{}, err
	}
	spec = normalizedSpec
	if existing, ok, err := existingQueuedRunWithStore(
		ctx,
		store,
		spec.TaskID,
		spec.IdempotencyKey,
		actor.Origin,
	); err != nil {
		return enqueueRunCommandResult{}, err
	} else if ok {
		taskRecord, taskErr := store.GetTask(ctx, existing.TaskID)
		if taskErr == nil {
			taskErr = m.authorizeTaskResource(ctx, actor, taskRecord)
		}
		return enqueueRunCommandResult{task: taskRecord, run: *existing, existing: true}, taskErr
	}
	taskRecord, err := store.GetTask(ctx, spec.TaskID)
	if err != nil {
		return enqueueRunCommandResult{}, err
	}
	if err := m.authorizeTaskResource(ctx, actor, taskRecord); err != nil {
		return enqueueRunCommandResult{}, err
	}
	run, existing, err := m.reserveQueuedRunWithStore(ctx, store, taskRecord, spec, actor)
	if err != nil {
		return enqueueRunCommandResult{}, err
	}
	if existing {
		return enqueueRunCommandResult{task: taskRecord, run: run, existing: true}, nil
	}
	return m.finishEnqueuedRunWithStore(ctx, store, run, actor)
}

func (m *Service) reserveQueuedRunWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	taskRecord Task,
	spec EnqueueRun,
	actor ActorContext,
) (Run, bool, error) {
	runID := m.newID("run")
	networkSpec, err := m.resolveQueuedRunParticipationWithStore(
		ctx,
		store,
		taskRecord,
		runID,
		spec.DesignationGroupID,
		spec.RunKind,
		spec.LoopRunID,
		spec.NetworkParticipation,
		spec.NetworkParticipationSource,
	)
	if err != nil {
		return Run{}, false, err
	}

	_, run, existing, err := store.ReserveQueuedRun(ctx, QueueRunReservation{
		TaskID:             spec.TaskID,
		RunID:              runID,
		RunKind:            spec.RunKind,
		LoopRunID:          spec.LoopRunID,
		IdempotencyKey:     spec.IdempotencyKey,
		Origin:             actor.Origin,
		NetworkSpec:        networkSpec,
		DesignationGroupID: spec.DesignationGroupID,
		Metadata:           spec.Metadata,
		QueuedAt:           m.now().UTC(),
	})
	if err != nil {
		return Run{}, false, err
	}
	return run, existing, nil
}

func (m *Service) finishEnqueuedRunWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	run Run,
	actor ActorContext,
) (enqueueRunCommandResult, error) {
	reconciledTask, err := m.reconcileTaskCascadeWithStore(ctx, store, run.TaskID, actor)
	if err != nil {
		return enqueueRunCommandResult{}, err
	}
	event, err := m.newTaskEvent(run.TaskID, run.ID, taskEventRunEnqueued, actor, runEnqueuedPayload{
		Attempt:        int(run.Attempt),
		Status:         run.Status,
		TaskStatus:     reconciledTask.Status,
		IdempotencyKey: run.IdempotencyKey,
	})
	if err != nil {
		return enqueueRunCommandResult{}, err
	}
	if err := store.CreateTaskEvent(ctx, event); err != nil {
		return enqueueRunCommandResult{}, err
	}
	var participationObservation *participation.ResolvedObservation
	networkSpec := run.NetworkSpecSnapshot()
	workspaceID := strings.TrimSpace(networkSpec.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(run.WorkspaceID)
	}
	if workspaceID != "" {
		participationObservation = &participation.ResolvedObservation{
			WorkspaceID: workspaceID,
			Owner: participation.OwnerRef{
				WorkspaceID: workspaceID,
				Kind:        participation.OwnerKindTaskRun,
				ID:          strings.TrimSpace(run.ID),
			},
			Spec: networkSpec,
		}
	}
	return enqueueRunCommandResult{
		task:                     reconciledTask,
		run:                      run,
		event:                    event,
		participationObservation: participationObservation,
	}, nil
}

func validateTaskForEnqueue(taskRecord Task) error {
	switch taskRecord.Status.Normalize() {
	case TaskStatusDraft:
		return fmt.Errorf("%w: task %q is draft", ErrInvalidStatusTransition, taskRecord.ID)
	case TaskStatusCanceled:
		return fmt.Errorf("%w: task %q is canceled", ErrInvalidStatusTransition, taskRecord.ID)
	default:
		return nil
	}
}

// StartRun transitions one claimed or starting run into active execution.
func (m *Service) StartRun(
	ctx context.Context,
	runID string,
	req StartRun,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}

	normalizedReq, err := normalizeStartRun(req)
	if err != nil {
		return nil, err
	}
	if err := requireLifecycleIdempotency(actor, normalizedReq.IdempotencyKey, "start_run"); err != nil {
		return nil, err
	}

	run, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.ensureTaskExecutable(ctx, taskRecord); err != nil {
		return nil, err
	}
	switch run.Status.Normalize() {
	case TaskRunStatusClaimed:
		if run.RunKind.Normalize() == RunKindCoordinator {
			return m.startCoordinatorRun(ctx, taskRecord, run, normalizedReq, actor)
		}
		var failedRun *Run
		run, failedRun, err = m.transitionClaimedRunToStarting(ctx, taskRecord, run, actor)
		if err != nil {
			if failedRun != nil {
				return failedRun, err
			}
			return nil, err
		}
	case TaskRunStatusStarting:
		if err := validateRunningSessionBinding(run); err != nil {
			return nil, err
		}
	default:
		return nil, requireRunTransition(run, TaskRunStatusRunning)
	}

	lifecycleCtx := taskRunLifecycleContext(ctx)
	run.Status = TaskRunStatusRunning
	run.StartedAt = m.now().UTC()
	if err := m.store.UpdateTaskRun(lifecycleCtx, run); err != nil {
		return nil, err
	}

	reconciledTask, err := m.reconcileTaskCascade(lifecycleCtx, run.TaskID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(lifecycleCtx, run.TaskID, run.ID, taskEventRunStarted, actor, runTransitionPayload{
		Status:     run.Status,
		TaskStatus: reconciledTask.Status,
		SessionID:  run.SessionID,
	}); err != nil {
		return nil, err
	}

	return &run, nil
}

// AttachRunSession binds one existing session to a claimed or starting run.
func (m *Service) AttachRunSession(
	ctx context.Context,
	runID string,
	sessionID string,
	actor ActorContext,
) (*Run, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return nil, err
	}
	if err := m.requireSessionExecutor("attach run session"); err != nil {
		return nil, err
	}

	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return nil, fmt.Errorf("%w: session id is required", ErrValidation)
	}

	run, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.ensureTaskExecutable(ctx, taskRecord); err != nil {
		return nil, err
	}
	if strings.TrimSpace(run.SessionID) != "" {
		return nil, ErrSessionAlreadyBound
	}

	switch run.Status.Normalize() {
	case TaskRunStatusClaimed, TaskRunStatusStarting:
	default:
		return nil, ErrSessionAttachNotAllowed
	}

	activeBindings, err := m.store.CountActiveSessionBindings(ctx, trimmedSessionID)
	if err != nil {
		return nil, err
	}
	if activeBindings > 0 {
		return nil, ErrSessionAlreadyBound
	}

	sessionRef, err := m.sessions.AttachTaskSession(ctx, run.ID, trimmedSessionID)
	if err != nil {
		return nil, err
	}
	if sessionRef == nil {
		return nil, fmt.Errorf(
			"%w: attach_task_session returned nil session reference",
			ErrValidation,
		)
	}
	if err := sessionRef.Validate(); err != nil {
		return nil, err
	}

	run.SessionID = strings.TrimSpace(sessionRef.SessionID)
	if run.Status.Normalize() == TaskRunStatusClaimed {
		run.Status = TaskRunStatusStarting
	}
	if err := m.store.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}

	reconciledTask, err := m.reconcileTaskCascade(ctx, run.TaskID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunSessionBound, actor, runTransitionPayload{
		Status:     run.Status,
		TaskStatus: reconciledTask.Status,
		SessionID:  run.SessionID,
	}); err != nil {
		return nil, err
	}

	return &run, nil
}
