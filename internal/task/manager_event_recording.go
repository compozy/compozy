package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
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

func (m *Service) recordTaskEventWithID(
	ctx context.Context,
	eventID string,
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	payload any,
) error {
	event, err := m.newTaskEventWithID(eventID, taskID, runID, eventType, actor, payload)
	if err != nil {
		return err
	}
	if err := m.store.CreateTaskEvent(ctx, event); err != nil {
		return err
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{event})
	return nil
}

func (m *Service) reserveTaskEventID() (string, error) {
	eventID, err := m.newID("evt")
	if err != nil {
		return "", fmt.Errorf("task: generate task event id: %w", err)
	}
	return eventID, nil
}

func (m *Service) reserveTaskEventIDs(count int) ([]string, error) {
	if count <= 0 {
		return nil, nil
	}
	reserved := make([]string, 0, count)
	for range count {
		eventID, err := m.reserveTaskEventID()
		if err != nil {
			return nil, err
		}
		reserved = append(reserved, eventID)
	}
	return reserved, nil
}

func (m *Service) newTaskEvent(
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	payload any,
) (Event, error) {
	return m.newTaskEventAt(taskID, runID, eventType, actor, m.now().UTC(), payload)
}

func (m *Service) newTaskEventWithID(
	eventID string,
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	payload any,
) (Event, error) {
	return m.newTaskEventWithIDAt(eventID, taskID, runID, eventType, actor, m.now().UTC(), payload)
}

func (m *Service) newTaskEventAt(
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	timestamp time.Time,
	payload any,
) (Event, error) {
	if isTransactionalWatchTaskEvent(eventType) {
		return Event{}, fmt.Errorf(
			"%w: task event %q must be appended inside its owning transaction",
			ErrValidation,
			strings.TrimSpace(eventType),
		)
	}
	eventID, err := m.reserveTaskEventID()
	if err != nil {
		return Event{}, err
	}
	return m.newTaskEventWithIDAt(eventID, taskID, runID, eventType, actor, timestamp, payload)
}

func (m *Service) newTaskEventWithIDAt(
	eventID string,
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	timestamp time.Time,
	payload any,
) (Event, error) {
	if isTransactionalWatchTaskEvent(eventType) {
		return Event{}, fmt.Errorf(
			"%w: task event %q must be appended inside its owning transaction",
			ErrValidation,
			strings.TrimSpace(eventType),
		)
	}
	return validatedTaskEvent(eventID, taskID, runID, eventType, actor, timestamp, payload)
}

func (m *Service) newTransactionalTaskEventAt(
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	timestamp time.Time,
	payload any,
) (Event, error) {
	if !isTransactionalWatchTaskEvent(eventType) {
		return Event{}, fmt.Errorf(
			"%w: task event %q is not owned by a state transaction",
			ErrValidation,
			strings.TrimSpace(eventType),
		)
	}
	return m.newValidatedTaskEventAt(taskID, runID, eventType, actor, timestamp, payload)
}

// NewCoordinatorRunCompletedEvent builds the canonical event committed by the
// coordinator state transaction. The store supplies the durable event ID while
// the task domain owns the public payload shape.
func NewCoordinatorRunCompletedEvent(
	eventID string,
	run Run,
	taskRecord Task,
	actor ActorContext,
	timestamp time.Time,
) (Event, error) {
	if run.RunKind.Normalize() != RunKindCoordinator || run.Status.Normalize() != TaskRunStatusCompleted {
		return Event{}, fmt.Errorf(
			"%w: coordinator completion event requires a completed coordinator run",
			ErrInvalidStatusTransition,
		)
	}
	return validatedTaskEvent(
		eventID,
		run.TaskID,
		run.ID,
		taskEventRunCompleted,
		actor,
		timestamp,
		completedRunPayload{
			Status:         run.Status,
			TaskStatus:     taskRecord.Status,
			Result:         completionEventResult(rawJSONValue(run.Result)),
			ClaimTokenHash: run.ClaimTokenHash,
		},
	)
}

func (m *Service) newValidatedTaskEventAt(
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	timestamp time.Time,
	payload any,
) (Event, error) {
	eventID, err := m.reserveTaskEventID()
	if err != nil {
		return Event{}, err
	}
	return validatedTaskEvent(
		eventID,
		taskID,
		runID,
		eventType,
		actor,
		timestamp,
		payload,
	)
}

func (m *Service) preflightTaskEvent(
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	payload any,
) error {
	if isTransactionalWatchTaskEvent(eventType) {
		return fmt.Errorf(
			"%w: task event %q must be appended inside its owning transaction",
			ErrValidation,
			strings.TrimSpace(eventType),
		)
	}
	_, err := validatedTaskEvent(
		"evt-preflight",
		taskID,
		runID,
		eventType,
		actor,
		time.Unix(0, 0).UTC(),
		payload,
	)
	return err
}

func validatedTaskEvent(
	eventID string,
	taskID string,
	runID string,
	eventType string,
	actor ActorContext,
	timestamp time.Time,
	payload any,
) (Event, error) {
	rawPayload, err := marshalTaskEventPayload(payload)
	if err != nil {
		return Event{}, err
	}
	event := Event{
		ID:        strings.TrimSpace(eventID),
		TaskID:    strings.TrimSpace(taskID),
		RunID:     strings.TrimSpace(runID),
		EventType: strings.TrimSpace(eventType),
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Payload:   rawPayload,
		Timestamp: timestamp.UTC(),
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

	postCommitCtx, cancel := taskPostCommitContext(ctx)
	defer cancel()
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
	postCommitCtx, cancel := taskPostCommitContext(ctx)
	defer cancel()
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
	hookCtx, cancel := taskRunObservationHookContext(ctx)
	defer cancel()
	_, err := m.taskHooks.DispatchTaskStatusChanged(hookCtx, payload)
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
