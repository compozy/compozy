package task

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (m *Service) publishTaskIntentWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	id string,
	actor ActorContext,
) (*Task, []Event, error) {
	record, err := store.GetTask(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if record.Status.Normalize() != TaskStatusDraft {
		return nil, nil, fmt.Errorf(
			"%w: task %q cannot publish from %q",
			ErrInvalidStatusTransition,
			record.ID,
			record.Status,
		)
	}

	candidate := record
	candidate.Status = TaskStatusPending
	if err := m.ensureTaskExecutableWithStore(ctx, store, candidate); err != nil {
		return nil, nil, err
	}

	previousStatus := record.Status
	record.Status = TaskStatusPending
	record.UpdatedAt = m.now().UTC()
	record.ClosedAt = time.Time{}
	if err := store.UpdateTask(ctx, record, actor); err != nil {
		return nil, nil, err
	}
	m.dispatchTaskStatusChangedAfterWrite(ctx, record, previousStatus, record.Status, actor)

	reconciled, err := m.reconcileTaskCascadeWithStore(ctx, store, record.ID, actor)
	if err != nil {
		return nil, nil, err
	}
	event, err := m.newTaskEvent(reconciled.ID, "", taskEventPublished, actor, publishedTaskPayload{
		PreviousStatus: TaskStatusDraft,
		Status:         reconciled.Status,
		ApprovalState:  reconciled.ApprovalState,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := store.CreateTaskEvent(ctx, event); err != nil {
		return nil, nil, err
	}
	return &reconciled, []Event{event}, nil
}

func (m *Service) transitionTaskApproval(
	ctx context.Context,
	id string,
	target ApprovalState,
	eventType string,
	actor ActorContext,
) (*Task, error) {
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

	var record Task
	var event Event
	command := func(store ExecutionMutationStore) error {
		var commandErr error
		result, committedEvent, commandErr := m.transitionTaskApprovalWithStore(
			ctx,
			store,
			trimmedID,
			target,
			eventType,
			actor,
		)
		if result != nil {
			record = *result
		}
		event = committedEvent
		return commandErr
	}
	if transactions, ok := m.store.(ExecutionTransactionStore); ok {
		if err := transactions.WithTaskExecutionTransaction(ctx, command); err != nil {
			return nil, err
		}
	} else if err := command(m.store); err != nil {
		return nil, err
	}
	m.publishTaskEventsAfterCommand(ctx, []Event{event})
	return &record, nil
}

func (m *Service) transitionTaskApprovalWithStore(
	ctx context.Context,
	store ExecutionMutationStore,
	id string,
	target ApprovalState,
	eventType string,
	actor ActorContext,
) (*Task, Event, error) {
	record, err := store.GetTask(ctx, id)
	if err != nil {
		return nil, Event{}, err
	}

	previousApprovalState := normalizeApprovalStateOrDefault(
		record.ApprovalPolicy,
		record.ApprovalState,
	)
	if !taskApprovalDecisionAllowed(record, target) {
		return nil, Event{}, fmt.Errorf(
			"%w: task %q cannot transition approval from %q to %q",
			ErrInvalidStatusTransition,
			record.ID,
			previousApprovalState,
			target.Normalize(),
		)
	}

	record.ApprovalState = target.Normalize()
	record.UpdatedAt = m.now().UTC()
	record.ClosedAt = time.Time{}
	if err := store.UpdateTask(ctx, record, actor); err != nil {
		return nil, Event{}, err
	}

	reconciled, err := m.reconcileTaskCascadeWithStore(ctx, store, record.ID, actor)
	if err != nil {
		return nil, Event{}, err
	}
	event, err := m.newTaskEvent(reconciled.ID, "", eventType, actor, approvalDecisionTaskPayload{
		PreviousApprovalState: previousApprovalState,
		ApprovalState:         reconciled.ApprovalState,
		Status:                reconciled.Status,
	})
	if err != nil {
		return nil, Event{}, err
	}
	if err := store.CreateTaskEvent(ctx, event); err != nil {
		return nil, Event{}, err
	}
	return &reconciled, event, nil
}
