package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) ClearPendingInputs(
	ctx context.Context,
	id string,
	caller PromptCaller,
) (ClearPendingInputsResult, error) {
	if m == nil || m.inputQueue == nil {
		return ClearPendingInputsResult{}, errors.New("session: input queue is not configured")
	}
	if err := caller.Validate(); err != nil {
		return ClearPendingInputsResult{}, err
	}
	session, err := m.lookupPromptSession(ctx, id)
	if err != nil {
		return ClearPendingInputsResult{}, err
	}
	turnID := session.CurrentTurnID()
	if turnID == "" {
		turnID, err = m.newPromptTurnID()
		if err != nil {
			return ClearPendingInputsResult{}, err
		}
	}
	cleared, err := m.inputQueue.Clear(ctx, store.SessionInputClearRequest{
		SessionID: session.ID, TurnID: turnID, ActorKind: caller.Kind, ActorID: caller.ID,
	})
	if err != nil {
		return ClearPendingInputsResult{}, m.recordInputClearFailure(ctx, session, turnID, caller, err)
	}
	result := ClearPendingInputsResult{
		Inputs:       make([]PendingInput, 0, len(cleared.Inputs)),
		ClearedCount: cleared.Cleared, QueueGeneration: cleared.Generation,
	}
	for index := range cleared.Inputs {
		result.Inputs = append(result.Inputs, pendingInputFromStore(&cleared.Inputs[index]))
	}
	if err := m.projectInputClearTraces(ctx, session); err != nil {
		return result, fmt.Errorf("session: queue cleared; transcript projection pending: %w",
			m.recordInputClearFailure(ctx, session, turnID, caller, err))
	}
	if cleared.Cleared == 0 {
		if err := m.recordInputClearEvent(ctx, session, events.SessionQueueCleared, inputClearEventPayload{
			TurnID: turnID, ActorKind: caller.Kind, ActorID: caller.ID,
		}, "queue-clear-empty:"+turnID); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (m *Manager) projectInputClearTraces(ctx context.Context, session *Session) error {
	if m.inputQueue == nil {
		return nil
	}
	traces, err := m.inputQueue.PendingClearTraces(ctx, session.ID)
	if err != nil {
		return err
	}
	for _, trace := range traces {
		marker, err := transcript.NewMarker(
			transcript.MarkerQueueCleared, "Queued input removed by explicit clear.", trace.CreatedAt,
			map[string]any{
				"queue_entry_id": trace.EntryID, "queue_generation": trace.Generation,
				"queue_status": store.SessionInputQueueStatusCanceled,
				"actor_kind":   trace.ActorKind, "actor_id": trace.ActorID,
			},
		)
		if err != nil {
			return err
		}
		event, err := marker.AgentEvent(session.ID, trace.TurnID)
		if err != nil {
			return err
		}
		event = event.WithEventID("queue-clear:" + trace.EntryID)
		if err := m.recordEventWithWriter(ctx, session, event, "", recordIdempotentSessionEvent); err != nil {
			return err
		}
		if err := m.recordInputClearEvent(ctx, session, events.SessionQueueCleared, inputClearEventPayload{
			TurnID: trace.TurnID, EntryID: trace.EntryID, Count: 1, ActorKind: trace.ActorKind, ActorID: trace.ActorID,
		}, "queue-clear-event:"+trace.EntryID); err != nil {
			return err
		}
		if err := m.inputQueue.MarkClearTraceProjected(ctx, session.ID, trace.EntryID); err != nil {
			return err
		}
		m.notifyAgentEvent(ctx, session, event)
	}
	return nil
}
