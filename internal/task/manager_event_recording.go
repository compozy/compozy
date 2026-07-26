package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

const (
	taskWatchEventBlocked        = string(hookspkg.HookTaskBlocked)
	taskWatchEventUnblocked      = string(hookspkg.HookTaskUnblocked)
	taskWatchEventNeedsAttention = string(hookspkg.HookTaskNeedsAttention)
	taskWatchEventRecovered      = string(hookspkg.HookTaskRecovered)
	taskWatchEventStatusChanged  = string(hookspkg.HookTaskStatusChanged)
	taskWatchEventRunCompleted   = string(hookspkg.HookTaskRunCompleted)
	taskWatchEventRunFailed      = string(hookspkg.HookTaskRunFailed)
)

func (m *Service) recordTaskEvent(
	ctx context.Context,
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	payload any,
) error {
	event, err := m.newTaskEvent(taskID, runID, eventType, actor, payload)
	if err != nil {
		return err
	}
	if err := m.store.CreateTaskEvent(ctx, event); err != nil {
		return err
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{event})
	return nil
}

func (m *Service) newTaskEvent(
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	payload any,
) (Event, error) {
	if isTransactionalWatchTaskEvent(eventType) {
		return Event{}, fmt.Errorf(
			"%w: task event %q must be appended inside its owning transaction",
			ErrValidation,
			strings.TrimSpace(eventType),
		)
	}
	rawPayload, err := marshalTaskEventPayload(payload)
	if err != nil {
		return Event{}, err
	}
	event := Event{
		ID:        m.newID("evt"),
		TaskID:    strings.TrimSpace(taskID),
		RunID:     strings.TrimSpace(runID),
		EventType: strings.TrimSpace(eventType),
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Payload:   rawPayload,
		Timestamp: m.now().UTC(),
	}
	if err := event.Validate(); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (m *Service) publishTaskEventsAfterCommand(ctx context.Context, events []Event) {
	if _, ok := m.store.(EventCommitObserverStore); ok {
		return
	}

	postCommitCtx := context.Background()
	if ctx != nil {
		postCommitCtx = context.WithoutCancel(ctx)
	}
	for _, event := range events {
		record, err := m.store.GetTaskEventRecord(postCommitCtx, event.ID)
		if err != nil {
			m.emitTaskLiveEventBestEffort(postCommitCtx, event.ID)
			continue
		}
		m.publishCommittedTaskEvent(postCommitCtx, record)
	}
}

// OnTaskEvent receives records from stores that own post-commit publication.
func (m *Service) OnTaskEvent(ctx context.Context, record EventRecord) {
	postCommitCtx := context.Background()
	if ctx != nil {
		postCommitCtx = context.WithoutCancel(ctx)
	}
	m.publishCommittedTaskEvent(postCommitCtx, record)
}

func (m *Service) publishCommittedTaskEvent(ctx context.Context, record EventRecord) {
	m.notifyTaskObserverBestEffort(ctx, record)
	m.emitTaskLiveRecordBestEffort(ctx, record)
	m.dispatchCommittedTaskStatusChanged(ctx, record)
}

func (m *Service) dispatchCommittedTaskStatusChanged(ctx context.Context, record EventRecord) {
	if strings.TrimSpace(record.Event.EventType) != taskWatchEventStatusChanged {
		return
	}
	var payload hookspkg.TaskStatusChangedPayload
	if err := json.Unmarshal(record.Event.Payload, &payload); err != nil {
		slog.Error(
			"task: decode committed task status event",
			"error", err,
			"event_id", record.Event.ID,
			"task_id", record.Event.TaskID,
		)
		return
	}
	payload.PayloadBase = hookspkg.PayloadBase{
		Event:     hookspkg.HookTaskStatusChanged,
		Timestamp: record.Event.Timestamp,
	}
	_, err := m.taskHooks.DispatchTaskStatusChanged(taskRunObservationHookContext(ctx), payload)
	if err != nil {
		slog.Error(
			"task: committed task status hook failed",
			"error", err,
			"event_id", record.Event.ID,
			"task_id", record.Event.TaskID,
		)
	}
}

func isTransactionalWatchTaskEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case taskWatchEventBlocked,
		taskWatchEventUnblocked,
		taskWatchEventNeedsAttention,
		taskWatchEventRecovered,
		taskWatchEventStatusChanged,
		taskWatchEventRunCompleted,
		taskWatchEventRunFailed:
		return true
	default:
		return false
	}
}

func (m *Service) notifyTaskObserverBestEffort(ctx context.Context, record EventRecord) {
	if m == nil || m.eventObserver == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error(
				"task: task event observer panicked during post-commit notification",
				"panic", recovered,
				"event_id", record.Event.ID,
				"task_id", record.Event.TaskID,
				"run_id", record.Event.RunID,
				"event_type", record.Event.EventType,
			)
		}
	}()
	m.eventObserver.OnTaskEvent(ctx, record)
}

func marshalTaskEventPayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		return nil, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("task: marshal task event payload: %w", err)
	}
	return json.RawMessage(raw), nil
}
