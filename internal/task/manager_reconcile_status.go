package task

import (
	"context"
	"strings"
	"time"
)

func (m *Service) reconcileTaskWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	record Task,
	actor ActorContext,
) (Task, error) {
	dependencies, err := store.ListDependencies(ctx, record.ID)
	if err != nil {
		return Task{}, err
	}
	runs, err := store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
	if err != nil {
		return Task{}, err
	}

	canonicalStatus, err := m.canonicalTaskStatusWithStore(ctx, store, record, dependencies, runs)
	if err != nil {
		return Task{}, err
	}
	if record.Status.Normalize() == canonicalStatus.Normalize() {
		return record, nil
	}

	previousStatus := record.Status
	record.Status = canonicalStatus
	record.UpdatedAt = m.now().UTC()
	if isTerminalTaskStatus(record.Status) {
		record.ClosedAt = record.UpdatedAt
	} else {
		record.ClosedAt = time.Time{}
	}
	if err := store.UpdateTask(ctx, record, actor); err != nil {
		return Task{}, err
	}
	m.dispatchTaskStatusChangedAfterWrite(ctx, record, previousStatus, record.Status, actor)
	return record, nil
}

func (m *Service) reconcileTaskCascade(ctx context.Context, taskID string, actor ActorContext) (Task, error) {
	return m.reconcileTaskCascadeWithStore(ctx, m.store, taskID, actor)
}

func (m *Service) reconcileTaskCascadeWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	taskID string,
	actor ActorContext,
) (Task, error) {
	previous, err := store.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, err
	}

	reconciled, err := m.reconcileTaskWithStore(ctx, store, previous, actor)
	if err != nil {
		return Task{}, err
	}
	if previous.Status.Normalize() != reconciled.Status.Normalize() {
		if err := m.reconcileDependentTasksWithStore(
			ctx,
			store,
			taskID,
			map[string]struct{}{taskID: {}},
			actor,
		); err != nil {
			return Task{}, err
		}
	}
	return reconciled, nil
}

func (m *Service) canonicalTaskStatus(
	ctx context.Context,
	record Task,
	dependencies []Dependency,
	runs []Run,
) (Status, error) {
	return m.canonicalTaskStatusWithStore(ctx, m.store, record, dependencies, runs)
}

func (m *Service) canonicalTaskStatusWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	record Task,
	dependencies []Dependency,
	runs []Run,
) (Status, error) {
	return m.canonicalTaskStatusReadOnlyWithStore(
		ctx,
		store,
		record,
		dependencies,
		runs,
		make(map[string]struct{}, len(dependencies)+1),
	)
}

func (m *Service) canonicalTaskStatusReadOnlyWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	record Task,
	dependencies []Dependency,
	runs []Run,
	visited map[string]struct{},
) (Status, error) {
	taskID := strings.TrimSpace(record.ID)
	if taskID != "" {
		if _, seen := visited[taskID]; seen {
			// Defensive termination guard: dependency cycles are invalid, but read
			// paths should still terminate without mutating persisted state.
			return taskStatusFromPolicySnapshot(
				record.Status,
				true,
				taskApprovalBlocked(record),
				taskPausedBlocked(record),
				false,
				taskNeedsAttentionBlocked(record),
				taskAttemptsExhausted(record, runs),
				runs,
			), nil
		}
		visited[taskID] = struct{}{}
		defer delete(visited, taskID)
	}

	unresolvedDependencies, err := m.hasUnresolvedDependenciesReadOnlyWithStore(
		ctx,
		store,
		dependencies,
		visited,
	)
	if err != nil {
		return "", err
	}
	openBlocks, err := store.HasOpenTaskBlocks(ctx, record.ID)
	if err != nil {
		return "", err
	}
	return taskStatusFromPolicySnapshot(
		record.Status,
		unresolvedDependencies,
		taskApprovalBlocked(record),
		taskPausedBlocked(record),
		openBlocks,
		taskNeedsAttentionBlocked(record),
		taskAttemptsExhausted(record, runs),
		runs,
	), nil
}

func hasOpenRun(runs []Run) bool {
	for _, run := range runs {
		if !isTerminalRunStatus(run.Status) {
			return true
		}
	}
	return false
}

func isTerminalTaskStatus(status Status) bool {
	switch status.Normalize() {
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCanceled:
		return true
	default:
		return false
	}
}

func isTerminalRunStatus(status RunStatus) bool {
	switch status.Normalize() {
	case TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCanceled:
		return true
	default:
		return false
	}
}

func taskStatusFromSnapshot(currentStatus Status, unresolvedDependencies bool, runs []Run) Status {
	return taskStatusFromPolicySnapshot(
		currentStatus,
		unresolvedDependencies,
		false,
		false,
		false,
		false,
		false,
		runs,
	)
}

func taskStatusFromPolicySnapshot(
	currentStatus Status,
	unresolvedDependencies bool,
	approvalBlocked bool,
	pausedBlocked bool,
	openBlocks bool,
	needsAttention bool,
	attemptsExhausted bool,
	runs []Run,
) Status {
	status := currentStatus.Normalize()
	if status == TaskStatusCanceled || status == TaskStatusDraft {
		return status
	}

	runnableBlocked := unresolvedDependencies || approvalBlocked || pausedBlocked || openBlocks
	runState := taskRunPolicyState(runs)
	if runState.hasActive {
		return TaskStatusInProgress
	}
	if runState.hasLatestTerminal && !runState.hasQueuedOrClaimed {
		if terminalStatus, ok := taskStatusFromTerminalRun(runState.latestTerminal, attemptsExhausted); ok {
			return terminalStatus
		}
	}

	if isTerminalTaskStatus(status) && !runState.hasQueuedOrClaimed {
		return status
	}
	if needsAttention {
		return TaskStatusNeedsAttention
	}
	if runState.hasQueuedOrClaimed {
		if runnableBlocked {
			return TaskStatusBlocked
		}
		return TaskStatusReady
	}
	if runnableBlocked {
		return TaskStatusBlocked
	}
	return TaskStatusReady
}

type taskRunPolicySnapshot struct {
	hasActive          bool
	hasQueuedOrClaimed bool
	latestTerminal     Run
	hasLatestTerminal  bool
}

func taskRunPolicyState(runs []Run) taskRunPolicySnapshot {
	state := taskRunPolicySnapshot{}
	for idx := range runs {
		run := runs[idx]
		switch run.Status.Normalize() {
		case TaskRunStatusStarting, TaskRunStatusRunning:
			state.hasActive = true
		case TaskRunStatusQueued, TaskRunStatusClaimed:
			state.hasQueuedOrClaimed = true
		case TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCanceled:
			if !state.hasLatestTerminal || runComesAfter(run, state.latestTerminal) {
				state.latestTerminal = run
				state.hasLatestTerminal = true
			}
		}
	}
	return state
}

func taskStatusFromTerminalRun(run Run, attemptsExhausted bool) (Status, bool) {
	switch run.Status.Normalize() {
	case TaskRunStatusCompleted:
		return TaskStatusCompleted, true
	case TaskRunStatusFailed:
		return TaskStatusFailed, attemptsExhausted
	case TaskRunStatusCanceled:
		return TaskStatusCanceled, true
	default:
		return "", false
	}
}

func taskApprovalBlocked(record Task) bool {
	return approvalStateBlocksExecution(record.ApprovalPolicy, record.ApprovalState)
}

func taskPausedBlocked(record Task) bool {
	return record.Paused
}

func taskNeedsAttentionBlocked(record Task) bool {
	return record.NeedsAttention != nil && !record.NeedsAttention.At.IsZero()
}

func approvalStateBlocksExecution(policy ApprovalPolicy, state ApprovalState) bool {
	normalizedPolicy := normalizeApprovalPolicyOrDefault(policy)
	if normalizedPolicy != ApprovalPolicyManual {
		return false
	}
	return normalizeApprovalStateOrDefault(normalizedPolicy, state) != ApprovalStateApproved
}

func taskAttemptsExhausted(record Task, runs []Run) bool {
	return nextRunAttempt(runs) > normalizeTaskMaxAttemptsOrDefault(record.MaxAttempts)
}

func runComesAfter(left Run, right Run) bool {
	switch {
	case left.Attempt != right.Attempt:
		return left.Attempt > right.Attempt
	case !left.QueuedAt.Equal(right.QueuedAt):
		return left.QueuedAt.After(right.QueuedAt)
	default:
		return left.ID > right.ID
	}
}
