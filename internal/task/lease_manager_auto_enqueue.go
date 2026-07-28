package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// autoEnqueueDispatchTimeout bounds the detached post-commit auto-enqueue side effect.
// The completion is already durable, so this work must survive request cancellation
// (context.WithoutCancel) but still cannot run unbounded across a large dependent set.
const autoEnqueueDispatchTimeout = 30 * time.Second

const (
	autoEnqueueTriggerDependencyCompletion = "dependency_completion"
	autoEnqueueTriggerBlockClear           = "block_clear"
	autoEnqueueTriggerTransientExpiry      = "transient_expiry"
	autoEnqueueTriggerApprovalGranted      = "approval_granted"
	autoEnqueueTriggerRecover              = "recover"
)

type autoEnqueueTrigger struct {
	Kind string
	Ref  string
}

func (t autoEnqueueTrigger) normalized() autoEnqueueTrigger {
	return autoEnqueueTrigger{
		Kind: strings.TrimSpace(t.Kind),
		Ref:  strings.TrimSpace(t.Ref),
	}
}

func (t autoEnqueueTrigger) idempotencyKey(taskID string) string {
	normalized := t.normalized()
	return fmt.Sprintf(
		"task.auto_enqueue.%s.%s.%s",
		strings.TrimSpace(taskID),
		normalized.Kind,
		normalized.Ref,
	)
}

// autoEnqueueReadyDependents enqueues runs for dependents that became ready due to a
// completed blocker and opted in via AutoEnqueueOnReady. A failed/expired blocker does
// not satisfy a "blocks" edge, so only completion calls this dependent traversal.
func (m *Service) autoEnqueueReadyDependents(
	ctx context.Context,
	completedTaskID string,
	trigger autoEnqueueTrigger,
	actor ActorContext,
) {
	dependents, err := m.store.ListDependents(ctx, completedTaskID)
	if err != nil {
		slog.Error("task: auto-enqueue list dependents failed", "task_id", completedTaskID, "error", err)
		return
	}
	taskIDs := make([]string, 0, len(dependents))
	for _, dep := range dependents {
		dependentID := strings.TrimSpace(dep.TaskID)
		if dependentID == "" {
			continue
		}
		taskIDs = append(taskIDs, dependentID)
	}
	m.autoEnqueueReadyTasks(ctx, taskIDs, trigger, actor)
}

func (m *Service) autoEnqueueReadyTaskDetached(
	ctx context.Context,
	taskID string,
	trigger autoEnqueueTrigger,
	actor ActorContext,
) {
	autoCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), autoEnqueueDispatchTimeout)
	defer cancel()
	m.autoEnqueueReadyTasks(autoCtx, []string{taskID}, trigger, actor)
}

func (m *Service) autoEnqueueReadyTasks(
	ctx context.Context,
	taskIDs []string,
	trigger autoEnqueueTrigger,
	actor ActorContext,
) {
	normalizedTrigger := trigger.normalized()
	if normalizedTrigger.Kind == "" || normalizedTrigger.Ref == "" {
		slog.Warn(
			"task: auto-enqueue skipped invalid trigger",
			"trigger_kind",
			trigger.Kind,
			"trigger_ref",
			trigger.Ref,
		)
		return
	}
	var pauseReader effectiveTaskPauseReader
	if reader, ok := m.store.(effectiveTaskPauseReader); ok {
		pauseReader = reader
	}
	seen := make(map[string]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		trimmedTaskID := strings.TrimSpace(taskID)
		if trimmedTaskID == "" {
			continue
		}
		if _, ok := seen[trimmedTaskID]; ok {
			continue
		}
		seen[trimmedTaskID] = struct{}{}
		m.autoEnqueueReadyTask(ctx, trimmedTaskID, normalizedTrigger, pauseReader, actor)
	}
}

func (m *Service) autoEnqueueReadyTask(
	ctx context.Context,
	taskID string,
	trigger autoEnqueueTrigger,
	pauseReader effectiveTaskPauseReader,
	actor ActorContext,
) {
	taskRecord, err := m.store.GetTask(ctx, taskID)
	if err != nil {
		slog.Error("task: auto-enqueue load task failed", "task_id", taskID, "error", err)
		return
	}
	if !taskRecord.AutoEnqueueOnReady || taskRecord.Status.Normalize() != TaskStatusReady {
		return
	}
	if pauseReader != nil {
		paused, _, pauseErr := pauseReader.IsTaskEffectivelyPaused(ctx, taskID)
		if pauseErr != nil {
			slog.Warn("task: auto-enqueue pause check failed", "task_id", taskID, "error", pauseErr)
			return
		}
		if paused {
			return
		}
	}
	key := trigger.idempotencyKey(taskID)
	run, enqErr := m.EnqueueRun(ctx, EnqueueRun{TaskID: taskID, IdempotencyKey: key}, actor)
	switch {
	case enqErr == nil:
		m.recordAutoEnqueueTriggered(ctx, taskRecord, run, trigger, actor)
	case errors.Is(enqErr, ErrInvalidStatusTransition) || errors.Is(enqErr, ErrConflict):
		// Expected dedup: store rejects a second run (open run) or replayed idempotency key.
		slog.Debug(
			"task: auto-enqueue skipped; task already has an open run",
			"task_id", taskID,
			"trigger_kind", trigger.Kind,
			"error", enqErr,
		)
	default:
		slog.Warn(
			"task: auto-enqueue run failed",
			"task_id", taskID,
			"trigger_kind", trigger.Kind,
			"error", enqErr,
		)
	}
}

func (m *Service) recordAutoEnqueueTriggered(
	ctx context.Context,
	taskRecord Task,
	run *Run,
	trigger autoEnqueueTrigger,
	actor ActorContext,
) {
	if run == nil {
		return
	}
	if err := m.recordTaskEvent(
		ctx,
		taskRecord.ID,
		run.ID,
		taskEventAutoEnqueueTriggered,
		actor,
		autoEnqueueTriggeredPayload{
			Status:      taskRecord.Status,
			RunStatus:   run.Status,
			TriggerKind: trigger.Kind,
			TriggerRef:  trigger.Ref,
		},
	); err != nil {
		slog.Warn(
			"task: auto-enqueue triggered event failed",
			"task_id", taskRecord.ID,
			"run_id", run.ID,
			"trigger_kind", trigger.Kind,
			"error", err,
		)
	}
}
