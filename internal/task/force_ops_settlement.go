package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type forceRunSettlement struct {
	mutation          ForceRunMutationResult
	task              Task
	statusTransitions []StatusTransition
	event             Event
}

type recoveredRunSettlement struct {
	result            RetryRunResult
	task              Task
	statusTransitions []StatusTransition
	event             Event
}

type retryRunSettlementResult struct {
	result            RetryRunResult
	task              Task
	statusTransitions []StatusTransition
	event             Event
}

func (m *Service) forceReleaseRunSettlement(
	ctx context.Context,
	previous Run,
	release ForceReleaseRun,
	actor ActorContext,
	commandAt time.Time,
) (forceRunSettlement, error) {
	settlement := forceRunSettlement{}
	err := m.store.WithTaskMutationTransaction(ctx, "force release task run", func(store runMutationStore) error {
		mutation, err := store.ForceReleaseTaskRun(
			ctx,
			NewForceReleaseRunMutation(previous, commandAt),
		)
		if err != nil {
			return err
		}
		inputInvalidation, err := invalidateForceRunInputsWithStore(ctx, store, mutation.Previous, commandAt)
		if err != nil {
			return err
		}
		taskRecord, transitions, err := m.reconcileTaskSettlementCascadeWithStore(
			ctx,
			store,
			mutation.Run.TaskID,
			actor,
			commandAt,
		)
		if err != nil {
			return err
		}
		event, err := m.newTaskEventAt(
			mutation.Run.TaskID,
			mutation.Run.ID,
			taskEventRunReleased,
			actor,
			commandAt,
			forceReleaseRunPayload(
				mutation.Previous,
				mutation.Run.Status,
				taskRecord.Status,
				release,
				inputInvalidation.QueueGeneration,
				inputInvalidation.CanceledInputs,
				actor,
			),
		)
		if err != nil {
			return err
		}
		if err := store.CreateTaskEvent(ctx, event); err != nil {
			return err
		}
		settlement = forceRunSettlement{
			mutation:          mutation,
			task:              taskRecord,
			statusTransitions: transitions,
			event:             event,
		}
		return nil
	})
	if err != nil {
		return forceRunSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, []Event{settlement.event}, settlement.statusTransitions, actor)
	return settlement, nil
}

func (m *Service) forceFailRunSettlement(
	ctx context.Context,
	previous Run,
	failure ForceFailRun,
	actor ActorContext,
	commandAt time.Time,
) (forceRunSettlement, error) {
	settlement := forceRunSettlement{}
	err := m.store.WithTaskMutationTransaction(ctx, "force fail task run", func(store runMutationStore) error {
		mutation, err := store.ForceFailTaskRun(
			ctx,
			NewForceFailRunMutation(previous, failure.Reason, commandAt),
		)
		if err != nil {
			return err
		}
		inputInvalidation, err := invalidateForceRunInputsWithStore(ctx, store, mutation.Previous, commandAt)
		if err != nil {
			return err
		}
		taskRecord, transitions, err := m.reconcileTaskSettlementCascadeWithStore(
			ctx,
			store,
			mutation.Run.TaskID,
			actor,
			commandAt,
		)
		if err != nil {
			return err
		}
		event, err := m.newTaskEventAt(
			mutation.Run.TaskID,
			mutation.Run.ID,
			taskEventRunOperatorForcedFail,
			actor,
			commandAt,
			forceFailRunPayload(
				mutation.Previous,
				mutation.Run.Status,
				taskRecord.Status,
				failure,
				inputInvalidation.QueueGeneration,
				inputInvalidation.CanceledInputs,
				actor,
			),
		)
		if err != nil {
			return err
		}
		if err := store.CreateTaskEvent(ctx, event); err != nil {
			return err
		}
		settlement = forceRunSettlement{
			mutation:          mutation,
			task:              taskRecord,
			statusTransitions: transitions,
			event:             event,
		}
		return nil
	})
	if err != nil {
		return forceRunSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, []Event{settlement.event}, settlement.statusTransitions, actor)
	return settlement, nil
}

func (m *Service) retryRunSettlement(
	ctx context.Context,
	source Run,
	newRunID string,
	retry RetryRunRequest,
	actor ActorContext,
	commandAt time.Time,
) (retryRunSettlementResult, error) {
	settlement := retryRunSettlementResult{}
	err := m.store.WithTaskMutationTransaction(ctx, "retry task run", func(store runMutationStore) error {
		result, err := store.RetryTaskRun(
			ctx,
			NewRetryRunMutation(source, newRunID, actor.Origin, retry.Metadata, commandAt),
		)
		if err != nil {
			return err
		}
		taskRecord, transitions, err := m.reconcileTaskSettlementCascadeWithStore(
			ctx,
			store,
			result.Run.TaskID,
			actor,
			commandAt,
		)
		if err != nil {
			return err
		}
		event, err := m.newTaskEventAt(
			result.Run.TaskID,
			result.Run.ID,
			taskEventRunOperatorRetry,
			actor,
			commandAt,
			operatorRetryPayload{
				Manual:      true,
				ActorKind:   actor.Actor.Kind.Normalize(),
				ActorID:     actor.Actor.Ref,
				SourceRunID: result.PreviousRun.ID,
				NewRunID:    result.Run.ID,
				TaskStatus:  taskRecord.Status,
			},
		)
		if err != nil {
			return err
		}
		if err := store.CreateTaskEvent(ctx, event); err != nil {
			return err
		}
		settlement = retryRunSettlementResult{
			result:            result,
			task:              taskRecord,
			statusTransitions: transitions,
			event:             event,
		}
		return nil
	})
	if err != nil {
		return retryRunSettlementResult{}, err
	}
	m.publishLeaseSettlement(ctx, []Event{settlement.event}, settlement.statusTransitions, actor)
	return settlement, nil
}

func (m *Service) recoverRunSettlement(
	ctx context.Context,
	source Run,
	newRunID string,
	recovery RecoverRunRequest,
	metadata json.RawMessage,
	actor ActorContext,
	commandAt time.Time,
) (recoveredRunSettlement, error) {
	settlement := recoveredRunSettlement{}
	err := m.store.WithTaskMutationTransaction(ctx, "recover task run", func(store runMutationStore) error {
		result, err := store.RecoverTaskRun(
			ctx,
			NewRecoverRunMutation(
				source,
				newRunID,
				actor.Origin,
				recovery.Reason,
				metadata,
				commandAt,
			),
		)
		if err != nil {
			return err
		}
		inputInvalidation, err := invalidateForceRunInputsWithStore(
			ctx,
			store,
			result.PreviousRun,
			commandAt,
		)
		if err != nil {
			return err
		}
		taskRecord, transitions, err := m.reconcileTaskSettlementCascadeWithStore(
			ctx,
			store,
			result.Run.TaskID,
			actor,
			commandAt,
		)
		if err != nil {
			return err
		}
		event, err := m.newTaskEventAt(
			result.Run.TaskID,
			result.Run.ID,
			taskEventRunRecoveredFromAttention,
			actor,
			commandAt,
			recoveredFromAttentionEventPayload(
				result.PreviousRun,
				result.Run.ID,
				result.Run.Status.Normalize(),
				taskRecord.Status,
				recovery.Reason,
				inputInvalidation.QueueGeneration,
				inputInvalidation.CanceledInputs,
				actor,
			),
		)
		if err != nil {
			return err
		}
		if err := store.CreateTaskEvent(ctx, event); err != nil {
			return err
		}
		settlement = recoveredRunSettlement{
			result:            result,
			task:              taskRecord,
			statusTransitions: transitions,
			event:             event,
		}
		return nil
	})
	if err != nil {
		return recoveredRunSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, []Event{settlement.event}, settlement.statusTransitions, actor)
	return settlement, nil
}

func invalidateForceRunInputsWithStore(
	ctx context.Context,
	store runMutationStore,
	previous Run,
	now time.Time,
) (ForceRunInputInvalidation, error) {
	sessionID := strings.TrimSpace(previous.SessionID)
	if sessionID == "" {
		return ForceRunInputInvalidation{}, nil
	}
	result, err := store.InvalidateForceRunInputs(ctx, sessionID, now)
	if err != nil {
		return ForceRunInputInvalidation{}, fmt.Errorf(
			"task: invalidate inputs for force operation on session %q: %w",
			sessionID,
			err,
		)
	}
	return result, nil
}
