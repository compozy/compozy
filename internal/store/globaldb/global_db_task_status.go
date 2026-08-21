package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type taskStatusChangedEventPayload struct {
	hookspkg.TaskContext
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Reason     string `json:"reason,omitempty"`
	Detail     string `json:"detail,omitempty"`
	LoopRunID  string `json:"loop_run_id,omitempty"`
}

type taskStatusEventContext struct {
	reason        string
	detail        string
	runID         string
	loopRunID     string
	releaseReason string
}

func setTaskStatusWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	from taskpkg.Status,
	to taskpkg.Status,
	actor taskpkg.ActorContext,
	timestamp time.Time,
) error {
	return setTaskStatusWithEventContext(ctx, exec, taskID, from, to, actor, timestamp, taskStatusEventContext{})
}

func setTaskStatusWithEventContext(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	from taskpkg.Status,
	to taskpkg.Status,
	actor taskpkg.ActorContext,
	timestamp time.Time,
	eventContext taskStatusEventContext,
) error {
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return err
	}
	fromStatus, toStatus, err := normalizeTaskStatusEvent(actor, from, to)
	if err != nil {
		return err
	}
	if fromStatus == toStatus {
		return nil
	}
	changed, err := sqlcgen.New(exec).UpdateTaskStatusChokepoint(ctx, sqlcgen.UpdateTaskStatusChokepointParams{
		ToStatus: string(toStatus), FromStatus: string(fromStatus), ID: trimmedTaskID,
	})
	if err != nil {
		return fmt.Errorf("store: update task %q status: %w", trimmedTaskID, err)
	}
	if changed == 0 {
		return explainTaskStatusChokepointMiss(ctx, exec, trimmedTaskID, fromStatus, toStatus)
	}

	committedTask, err := taskRecordAfterStatusChange(ctx, exec, trimmedTaskID, toStatus)
	if err != nil {
		return err
	}
	return appendTaskStatusChangedEvent(ctx, exec, committedTask, fromStatus, toStatus, actor, timestamp, eventContext)
}

func appendTaskStatusChangedAuditEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	from taskpkg.Status,
	to taskpkg.Status,
	actor taskpkg.ActorContext,
	timestamp time.Time,
	eventContext taskStatusEventContext,
) error {
	fromStatus, toStatus, err := normalizeTaskStatusEvent(actor, from, to)
	if err != nil {
		return err
	}
	committedTask, err := taskRecordAfterStatusChange(ctx, exec, taskID, toStatus)
	if err != nil {
		return err
	}
	return appendTaskStatusChangedEvent(ctx, exec, committedTask, fromStatus, toStatus, actor, timestamp, eventContext)
}

func appendTaskStatusChangedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	committedTask taskpkg.Task,
	fromStatus taskpkg.Status,
	toStatus taskpkg.Status,
	actor taskpkg.ActorContext,
	timestamp time.Time,
	eventContext taskStatusEventContext,
) error {
	eventID, err := store.NewID("evt")
	if err != nil {
		return fmt.Errorf("store: generate task status event id: %w", err)
	}
	taskContext := immutableTaskStatusContext(committedTask, actor)
	taskContext.RunID = strings.TrimSpace(eventContext.runID)
	taskContext.ReleaseReason = strings.TrimSpace(eventContext.releaseReason)
	payload, err := json.Marshal(taskStatusChangedEventPayload{
		TaskContext: taskContext,
		FromStatus:  string(fromStatus),
		ToStatus:    string(toStatus),
		Reason:      strings.TrimSpace(eventContext.reason),
		Detail:      strings.TrimSpace(eventContext.detail),
		LoopRunID:   strings.TrimSpace(eventContext.loopRunID),
	})
	if err != nil {
		return fmt.Errorf("store: marshal task %q status_changed event: %w", committedTask.ID, err)
	}
	if err := appendTaskEventWithExecutor(ctx, exec, EventRecordInsert{
		ID:        eventID,
		TaskID:    committedTask.ID,
		RunID:     strings.TrimSpace(eventContext.runID),
		EventType: string(hookspkg.HookTaskStatusChanged),
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Payload:   payload,
		Timestamp: timestamp,
	}); err != nil {
		return err
	}

	return nil
}

func normalizeTaskStatusEvent(
	actor taskpkg.ActorContext,
	from taskpkg.Status,
	to taskpkg.Status,
) (taskpkg.Status, taskpkg.Status, error) {
	if err := actor.Validate(); err != nil {
		return "", "", err
	}
	fromStatus := from.Normalize()
	if err := fromStatus.Validate("task.status.from"); err != nil {
		return "", "", err
	}
	toStatus := to.Normalize()
	if err := toStatus.Validate("task.status.to"); err != nil {
		return "", "", err
	}
	return fromStatus, toStatus, nil
}

func taskRecordAfterStatusChange(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	status taskpkg.Status,
) (taskpkg.Task, error) {
	record, err := (&TaskRepo{}).getTaskWithExecutor(ctx, exec, taskID)
	if err != nil {
		return taskpkg.Task{}, err
	}
	record.Status = status
	return record, nil
}

func immutableTaskStatusContext(
	record taskpkg.Task,
	actor taskpkg.ActorContext,
) hookspkg.TaskContext {
	agentName := ""
	if actor.Actor.Kind.Normalize() == taskpkg.ActorKindAgentSession {
		agentName = strings.TrimSpace(actor.Actor.Ref)
	}
	return hookspkg.TaskContext{
		TaskID:       strings.TrimSpace(record.ID),
		ParentTaskID: strings.TrimSpace(record.ParentTaskID),
		WorkspaceID:  strings.TrimSpace(record.WorkspaceID),
		AgentName:    agentName,
		ActorKind:    string(actor.Actor.Kind.Normalize()),
		ActorID:      strings.TrimSpace(actor.Actor.Ref),
		OriginKind:   string(actor.Origin.Kind.Normalize()),
		OriginRef:    strings.TrimSpace(actor.Origin.Ref),
		TaskStatus:   string(record.Status.Normalize()),
	}
}

func setTaskStatusIfChangedWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Task,
	updated taskpkg.Task,
	actor taskpkg.ActorContext,
	changed bool,
) error {
	if !changed {
		return nil
	}
	return setTaskStatusWithExecutor(
		ctx,
		exec,
		updated.ID,
		current.Status,
		updated.Status,
		actor,
		updated.UpdatedAt,
	)
}

func taskStatusChangedForUpdate(
	current taskpkg.Status,
	updated taskpkg.Status,
	actor taskpkg.ActorContext,
) (bool, error) {
	changed := current.Normalize() != updated.Normalize()
	if !changed {
		return false, nil
	}
	if err := actor.Validate(); err != nil {
		return false, err
	}
	return true, nil
}

func explainTaskStatusChokepointMiss(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	from taskpkg.Status,
	to taskpkg.Status,
) error {
	current, err := sqlcgen.New(exec).GetTaskStatus(ctx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskNotFound
		}
		return fmt.Errorf("store: lookup task %q status: %w", taskID, err)
	}
	return fmt.Errorf(
		"%w: task %q status changed before chokepoint update from %q to %q; current %q",
		taskpkg.ErrInvalidStatusTransition,
		taskID,
		from,
		to,
		strings.TrimSpace(current),
	)
}
