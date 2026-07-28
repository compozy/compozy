package task

import (
	"context"

	"fmt"

	"strings"

	"time"
)

func (m *Service) loadRunWithTask(ctx context.Context, runID string) (Run, Task, error) {
	run, err := m.loadRun(ctx, runID)
	if err != nil {
		return Run{}, Task{}, err
	}
	taskRecord, err := m.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return Run{}, Task{}, err
	}
	return run, taskRecord, nil
}

func (m *Service) loadRun(ctx context.Context, runID string) (Run, error) {
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return Run{}, fmt.Errorf("%w: task run id is required", ErrValidation)
	}

	run, err := m.store.GetTaskRun(ctx, trimmedRunID)
	if err != nil {
		return Run{}, err
	}
	return run, nil
}

func (m *Service) ensureTaskExecutable(ctx context.Context, record Task) error {
	return m.ensureTaskExecutableWithStore(ctx, m.store, record)
}

func (m *Service) ensureTaskExecutableWithStore(
	ctx context.Context,
	store DeleteTaskMutationStore,
	record Task,
) error {
	dependencies, err := store.ListDependencies(ctx, record.ID)
	if err != nil {
		return err
	}
	runs, err := store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
	if err != nil {
		return err
	}
	status, err := m.canonicalTaskStatusWithStore(ctx, store, record, dependencies, runs)
	if err != nil {
		return err
	}

	switch status.Normalize() {
	case TaskStatusBlocked:
		return fmt.Errorf("%w: task %q is blocked", ErrInvalidStatusTransition, record.ID)
	case TaskStatusDraft:
		return fmt.Errorf("%w: task %q is draft", ErrInvalidStatusTransition, record.ID)
	case TaskStatusCanceled:
		return fmt.Errorf("%w: task %q is canceled", ErrInvalidStatusTransition, record.ID)
	default:
		return nil
	}
}

func (m *Service) requireSessionExecutor(action string) error {
	if m.sessions == nil {
		return fmt.Errorf("%w: session executor is required to %s", ErrValidation, action)
	}
	return nil
}

func (m *Service) collectTaskTree(ctx context.Context, rootTaskID string) ([]Task, error) {
	root, err := m.store.GetTask(ctx, rootTaskID)
	if err != nil {
		return nil, err
	}

	tree := []Task{root}
	queue := []Task{root}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		children, err := m.store.ListTasks(ctx, Query{ParentTaskID: current.ID})
		if err != nil {
			return nil, err
		}
		for idx := range children {
			child := &children[idx]
			record, err := m.store.GetTask(ctx, child.ID)
			if err != nil {
				return nil, err
			}
			tree = append(tree, record)
			queue = append(queue, record)
		}
	}

	return tree, nil
}

type cancelRunOptions struct {
	propagatedFromTaskID string
	reconcileTask        bool
}

type failRunRecordOptions struct {
	stopTerminalSession bool
}

func (m *Service) failRunRecord(
	ctx context.Context,
	taskRecord Task,
	run Run,
	failure RunFailure,
	actor ActorContext,
) (*Run, error) {
	return m.failRunRecordWithOptions(ctx, taskRecord, run, failure, actor, failRunRecordOptions{
		stopTerminalSession: true,
	})
}

func (m *Service) failRunRecordWithOptions(
	ctx context.Context,
	taskRecord Task,
	run Run,
	failure RunFailure,
	actor ActorContext,
	opts failRunRecordOptions,
) (*Run, error) {
	switch run.Status.Normalize() {
	case TaskRunStatusStarting, TaskRunStatusRunning:
	default:
		return nil, requireRunTransition(run, TaskRunStatusFailed)
	}

	run.Status = TaskRunStatusFailed
	run.Error = failure.Error
	run.Result = nil
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	run.EndedAt = m.now().UTC()
	if opts.stopTerminalSession {
		if err := m.stopTerminalRunSession(ctx, run, StopReasonFailed); err != nil {
			return nil, err
		}
	}
	if err := m.store.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}

	reconciledTask, err := m.reconcileTaskCascade(ctx, taskRecord.ID, actor)
	if err != nil {
		return nil, err
	}
	if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunFailed, actor, failedRunPayload{
		Status:     run.Status,
		TaskStatus: reconciledTask.Status,
		Error:      run.Error,
		Metadata:   cloneRawJSON(failure.Metadata),
	}); err != nil {
		return nil, err
	}
	m.dispatchTerminalWake(ctx, reconciledTask, run, actor)

	return &run, nil
}

func (m *Service) cancelRunRecord(
	ctx context.Context,
	taskRecord Task,
	run Run,
	req CancelRun,
	actor ActorContext,
	opts cancelRunOptions,
) (*Run, error) {
	status := run.Status.Normalize()
	switch status {
	case TaskRunStatusQueued, TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
	default:
		return nil, requireRunTransition(run, TaskRunStatusCanceled)
	}

	sessionID := strings.TrimSpace(run.SessionID)
	activeSession := (status == TaskRunStatusStarting || status == TaskRunStatusRunning) &&
		sessionID != ""
	if activeSession {
		if err := m.requireSessionExecutor("cancel active run"); err != nil {
			return nil, err
		}
	}

	run.Status = TaskRunStatusCanceled
	run.Result = nil
	run.Error = ""
	run.EndedAt = m.now().UTC()

	cooperativeStopRequested := false
	if activeSession {
		if err := m.sessions.RequestTaskStop(ctx, sessionID, StopReasonCancellation); err != nil {
			return nil, fmt.Errorf("task: request stop for session %q: %w", sessionID, err)
		}
		cooperativeStopRequested = true
		if err := m.waitAndForceStopRun(ctx, sessionID, StopReasonCancellation); err != nil {
			return nil, err
		}
	}

	if err := m.store.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}

	reconciledTask := taskRecord
	if opts.reconcileTask {
		var err error
		reconciledTask, err = m.reconcileTaskCascade(ctx, taskRecord.ID, actor)
		if err != nil {
			return nil, err
		}
	}
	if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunCanceled, actor, cancelledRunPayload{
		Status:                   run.Status,
		TaskStatus:               reconciledTask.Status,
		Reason:                   req.Reason,
		Metadata:                 cloneRawJSON(req.Metadata),
		SessionID:                sessionID,
		PropagatedFromTaskID:     opts.propagatedFromTaskID,
		CooperativeStopRequested: cooperativeStopRequested,
	}); err != nil {
		return nil, err
	}
	m.dispatchTerminalWake(ctx, reconciledTask, run, actor)

	if activeSession {
		if err := m.recordTaskEvent(ctx, run.TaskID, run.ID, taskEventRunForceStopped, actor, forceStoppedRunPayload{
			SessionID:            sessionID,
			GraceTimeoutMillis:   m.cancelGracePeriod.Milliseconds(),
			PropagatedFromTaskID: opts.propagatedFromTaskID,
		}); err != nil {
			return nil, err
		}
	}

	return &run, nil
}

func (m *Service) stopTerminalRunSession(ctx context.Context, run Run, reason StopReason) error {
	sessionID := strings.TrimSpace(run.SessionID)
	if sessionID == "" {
		return nil
	}
	if err := m.requireSessionExecutor("stop terminal run session"); err != nil {
		return err
	}
	if err := m.sessions.RequestTaskStop(ctx, sessionID, reason); err != nil {
		return fmt.Errorf("task: request stop for session %q: %w", sessionID, err)
	}
	if err := m.waitAndForceStopRun(ctx, sessionID, reason); err != nil {
		return err
	}
	return nil
}

func (m *Service) stopRecoveredRunSession(
	ctx context.Context,
	run Run,
	recovery RunBootRecovery,
) error {
	sessionID := strings.TrimSpace(run.SessionID)
	if sessionID == "" {
		return nil
	}
	if err := m.requireSessionExecutor("stop recovered run session"); err != nil {
		return err
	}
	if recoverySessionStateRequiresCooperativeStop(recovery.SessionState) {
		return m.stopTerminalRunSession(ctx, run, StopReasonFailed)
	}
	if err := m.sessions.ForceTaskStop(ctx, sessionID, StopReasonFailed); err != nil {
		return fmt.Errorf("task: force stop session %q: %w", sessionID, err)
	}
	return nil
}

func (m *Service) waitAndForceStopRun(
	ctx context.Context,
	sessionID string,
	reason StopReason,
) error {
	if m.cancelGracePeriod > 0 {
		timer := time.NewTimer(m.cancelGracePeriod)
		defer timer.Stop()

		select {
		case <-timer.C:
		case <-ctx.Done():
			return fmt.Errorf(
				"task: wait for force-stop grace period on session %q: %w",
				sessionID,
				ctx.Err(),
			)
		}
	}
	if err := m.sessions.ForceTaskStop(ctx, sessionID, reason); err != nil {
		return fmt.Errorf("task: force stop session %q: %w", sessionID, err)
	}
	return nil
}

func recoverySessionStateRequiresCooperativeStop(state string) bool {
	switch strings.TrimSpace(state) {
	case "starting", managerActiveKey, "stopping":
		return true
	default:
		return false
	}
}

func requireRunTransition(run Run, next RunStatus) error {
	current := run.Status.Normalize()
	target := next.Normalize()
	if allowsRunTransition(current, target) {
		return nil
	}
	return fmt.Errorf(
		"%w: task run %q cannot transition from %q to %q",
		ErrInvalidStatusTransition,
		run.ID,
		current,
		target,
	)
}

func allowsRunTransition(current RunStatus, next RunStatus) bool {
	switch current.Normalize() {
	case TaskRunStatusQueued:
		switch next.Normalize() {
		case TaskRunStatusClaimed, TaskRunStatusCanceled, TaskRunStatusNeedsAttention:
			return true
		}
	case TaskRunStatusClaimed:
		switch next.Normalize() {
		case TaskRunStatusStarting, TaskRunStatusCanceled, TaskRunStatusNeedsAttention:
			return true
		}
	case TaskRunStatusStarting:
		switch next.Normalize() {
		case TaskRunStatusRunning, TaskRunStatusFailed, TaskRunStatusCanceled, TaskRunStatusNeedsAttention:
			return true
		}
	case TaskRunStatusRunning:
		switch next.Normalize() {
		case TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCanceled, TaskRunStatusNeedsAttention:
			return true
		}
	}
	return false
}
