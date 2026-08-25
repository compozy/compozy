package task

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/network/participation"
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
	if err := m.store.WithTaskExecutionTransaction(ctx, command); err != nil {
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
		existing.ProfileID = taskRecord.ProfileID
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
	run.ProfileID = taskRecord.ProfileID
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
	runID, err := m.newID("run")
	if err != nil {
		return Run{}, false, fmt.Errorf("task: generate run id: %w", err)
	}
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
	worktreePolicy, err := m.resolveQueuedRunWorktreePolicyWithStore(
		ctx,
		store,
		taskRecord.ID,
		spec.WorktreePerRun,
	)
	if err != nil {
		return Run{}, false, err
	}

	_, run, existing, err := store.ReserveQueuedRun(ctx, QueueRunReservation{
		TaskID:               spec.TaskID,
		RunID:                runID,
		RunKind:              spec.RunKind,
		LoopRunID:            spec.LoopRunID,
		IdempotencyKey:       spec.IdempotencyKey,
		Origin:               actor.Origin,
		NetworkSpec:          networkSpec,
		DesignationGroupID:   spec.DesignationGroupID,
		ResolvedWorktreeMode: worktreePolicy.Mode,
		ResolvedWorktreeRef:  worktreePolicy.WorktreeRef,
		Metadata:             spec.Metadata,
		QueuedAt:             m.now().UTC(),
		ResultBudget:         m.resultBudget,
	})
	if err != nil {
		return Run{}, false, err
	}
	return run, existing, nil
}

func (m *Service) resolveQueuedRunWorktreePolicyWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	taskID string,
	worktreePerRun bool,
) (WorktreePolicy, error) {
	if worktreePerRun {
		return WorktreePolicy{Mode: WorktreeModePerRun}, nil
	}
	profile, err := store.GetExecutionProfile(ctx, taskID)
	if errors.Is(err, ErrExecutionProfileNotFound) {
		profile = defaultExecutionProfile(taskID)
	} else if err != nil {
		return WorktreePolicy{}, err
	}
	resolved, err := resolveWorktreePolicySnapshot(
		profile.Worktree,
		m.profileValidation.DefaultWorktreeMode,
	)
	if err != nil {
		return WorktreePolicy{}, fmt.Errorf("task: resolve worktree policy for enqueue: %w", err)
	}
	return resolved, nil
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
	if run.Status.Normalize() == TaskRunStatusQueued {
		run, taskRecord, err = m.admitQueuedRunForDirectExecution(ctx, run, actor)
		if err != nil {
			return nil, err
		}
	}
	switch run.Status.Normalize() {
	case TaskRunStatusClaimed:
		if run.RunKind.Normalize() == RunKindCoordinator {
			return m.startCoordinatorRun(ctx, taskRecord, run, normalizedReq, actor)
		}
		var failedRun *Run
		run, failedRun, err = m.transitionClaimedRunToStarting(
			ctx,
			taskRecord,
			run,
			normalizedReq.ClaimToken,
			actor,
		)
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
	return m.commitStartedRun(ctx, run, normalizedReq.ClaimToken, actor)
}

func (m *Service) commitStartedRun(
	ctx context.Context,
	run Run,
	claimToken string,
	actor ActorContext,
) (*Run, error) {
	lifecycleCtx := taskRunLifecycleContext(ctx)
	if err := m.preflightTaskEvent(
		run.TaskID,
		run.ID,
		taskEventRunStarted,
		actor,
		runTransitionPayload{
			Status:     TaskRunStatusRunning,
			TaskStatus: TaskStatusNeedsAttention,
			SessionID:  run.SessionID,
		},
	); err != nil {
		return nil, err
	}
	commandAt := m.now().UTC()
	settlement, err := m.commitNominalRunSettlement(
		lifecycleCtx,
		"transition task run running",
		taskEventRunStarted,
		actor,
		commandAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.TransitionRunRunning(
				lifecycleCtx,
				NewRunRunningMutation(run, claimToken, commandAt),
			)
		},
		transitionRunPayload,
	)
	if err != nil {
		return nil, err
	}
	return &settlement.mutation.Run, nil
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
	trimmedSessionID := strings.TrimSpace(sessionID)
	run, err := m.prepareRunSessionAttachment(ctx, runID, trimmedSessionID, actor)
	if err != nil {
		return nil, err
	}
	return m.attachAndPersistRunSession(ctx, run, trimmedSessionID, "", actor)
}

func (m *Service) prepareRunSessionAttachment(
	ctx context.Context,
	runID string,
	sessionID string,
	actor ActorContext,
) (Run, error) {
	if err := m.requireSessionExecutor("attach run session"); err != nil {
		return Run{}, err
	}
	if err := (SessionRef{SessionID: sessionID}).Validate(); err != nil {
		return Run{}, err
	}

	run, taskRecord, err := m.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return Run{}, err
	}
	if err := m.ensureTaskExecutable(ctx, taskRecord); err != nil {
		return Run{}, err
	}
	if run.Status.Normalize() == TaskRunStatusQueued {
		run, _, err = m.admitQueuedRunForDirectExecution(ctx, run, actor)
		if err != nil {
			return Run{}, err
		}
	}
	if strings.TrimSpace(run.SessionID) != "" {
		return Run{}, ErrSessionAlreadyBound
	}

	switch run.Status.Normalize() {
	case TaskRunStatusClaimed, TaskRunStatusStarting:
	default:
		return Run{}, ErrSessionAttachNotAllowed
	}
	candidate := run
	candidate.Status = TaskRunStatusStarting
	candidate.SessionID = sessionID
	if err := m.preflightRunTransition(candidate, taskEventRunSessionBound, candidate.Status, actor); err != nil {
		return Run{}, err
	}

	activeBindings, err := m.store.CountActiveSessionBindings(ctx, sessionID)
	if err != nil {
		return Run{}, err
	}
	if activeBindings > 0 {
		return Run{}, ErrSessionAlreadyBound
	}
	return run, nil
}

func (m *Service) attachAndPersistRunSession(
	ctx context.Context,
	run Run,
	sessionID string,
	claimToken string,
	actor ActorContext,
) (*Run, error) {
	var sessionRef *SessionRef
	var err error
	if executor, ok := m.sessions.(RunSessionAttachmentExecutor); ok {
		sessionRef, err = executor.AttachTaskRunSession(ctx, run, sessionID)
	} else {
		sessionRef, err = m.sessions.AttachTaskSession(ctx, run.ID, sessionID)
	}
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

	boundSessionID := strings.TrimSpace(sessionRef.SessionID)
	candidate := run
	candidate.SessionID = boundSessionID
	candidate.SetWorktreeID(sessionRef.WorktreeID)
	candidate.Status = TaskRunStatusStarting
	if err := m.preflightRunTransition(candidate, taskEventRunSessionBound, candidate.Status, actor); err != nil {
		return nil, err
	}
	commandAt := m.now().UTC()
	settlement, err := m.commitNominalRunSettlement(
		ctx,
		"attach task run session",
		taskEventRunSessionBound,
		actor,
		commandAt,
		func(store runMutationStore) (NominalRunMutationResult, error) {
			return store.BindRunSession(
				ctx,
				NewRunSessionBindingMutation(
					run,
					claimToken,
					boundSessionID,
					sessionRef.WorktreeID,
					commandAt,
				),
			)
		},
		transitionRunPayload,
	)
	if err != nil {
		return nil, err
	}
	return &settlement.mutation.Run, nil
}
