package task

import (
	"context"
	"time"
)

type nominalRunSettlement struct {
	mutation          NominalRunMutationResult
	task              Task
	statusTransitions []StatusTransition
	event             Event
}

type nominalRunMutationFunc func(runMutationStore) (NominalRunMutationResult, error)

type nominalRunEventPayloadFunc func(*NominalRunMutationResult, Task) any

func (m *Service) commitNominalRunSettlement(
	ctx context.Context,
	action string,
	eventType string,
	actor ActorContext,
	commandAt time.Time,
	mutate nominalRunMutationFunc,
	payload nominalRunEventPayloadFunc,
) (nominalRunSettlement, error) {
	settlement := nominalRunSettlement{}
	err := m.store.WithTaskMutationTransaction(ctx, action, func(store runMutationStore) error {
		mutation, err := mutate(store)
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
			eventType,
			actor,
			commandAt,
			payload(&mutation, taskRecord),
		)
		if err != nil {
			return err
		}
		if err := store.CreateTaskEvent(ctx, event); err != nil {
			return err
		}
		settlement = nominalRunSettlement{
			mutation:          mutation,
			task:              taskRecord,
			statusTransitions: transitions,
			event:             event,
		}
		return nil
	})
	if err != nil {
		return nominalRunSettlement{}, err
	}
	m.publishLeaseSettlement(ctx, []Event{settlement.event}, settlement.statusTransitions, actor)
	return settlement, nil
}

func transitionRunPayload(mutation *NominalRunMutationResult, taskRecord Task) any {
	return runTransitionPayload{
		Status:     mutation.Run.Status,
		TaskStatus: taskRecord.Status,
		SessionID:  mutation.Run.SessionID,
	}
}

func recoveredRunEventPayload(
	recovery RunBootRecovery,
	previousStatus RunStatus,
	previousSessionID string,
) nominalRunEventPayloadFunc {
	return func(mutation *NominalRunMutationResult, taskRecord Task) any {
		return recoveredRunPayload{
			Action:         recovery.Action,
			PreviousStatus: previousStatus,
			Status:         mutation.Run.Status,
			TaskStatus:     taskRecord.Status,
			Reason:         recovery.Reason,
			SessionID:      previousSessionID,
			SessionState:   recovery.SessionState,
			Classification: recovery.Classification,
			Detail:         recovery.Detail,
		}
	}
}
