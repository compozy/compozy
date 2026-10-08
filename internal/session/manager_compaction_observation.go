package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type compactionLifecycle struct {
	first    bool
	terminal bool
	payload  CompactionFiredPayload
	post     *acp.CompactionObservation
}

func (m *Manager) prepareCompactionSnapshot(
	ctx context.Context,
	session *Session,
	event *acp.AgentEvent,
) (compactionLifecycle, bool, error) {
	if event.Type != acp.EventTypeCompaction || event.Compaction == nil {
		return compactionLifecycle{}, false, nil
	}
	recorder := session.recorderHandle()
	if recorder == nil {
		return compactionLifecycle{}, false, errors.New("session: event recorder is not available")
	}
	state, err := readCompactionLifecycle(ctx, recorder, event.Compaction.CompactionID, event.TurnID)
	if err != nil {
		return compactionLifecycle{}, false, err
	}
	current := *event.Compaction
	duplicate := state.previous != nil && state.previous.Status == current.Status &&
		state.previous.Summary == current.Summary &&
		state.previous.Error == current.Error
	current.Terminal = state.terminal == nil && compactionTerminalStatus(current.Status)
	event.Compaction = &current
	lifecycle := compactionLifecycle{first: !state.attributed, terminal: current.Terminal, payload: state.payload}
	if lifecycle.first && state.terminal != nil {
		lifecycle.terminal = true
		lifecycle.post = state.terminal
	}
	if lifecycle.first {
		lifecycle.payload = CompactionFiredPayload{CompactionID: current.CompactionID, Trigger: "agent"}
		if state.requested ||
			(state.firstSequence == 0 && session.CurrentPromptDelivery() == PromptDeliveryMaintenance) {
			lifecycle.payload.Trigger = "requested"
		}
		if err := m.fillCompactionContext(ctx, session.ID, &lifecycle.payload, state); err != nil {
			return compactionLifecycle{}, false, err
		}
	}
	return lifecycle, duplicate, nil
}

type compactionPersistedState struct {
	previous, terminal *acp.CompactionObservation
	payload            CompactionFiredPayload
	attributed         bool
	firstSequence      int64
	boundary           *int64
	requested          bool
	originTurn         string
}

func readCompactionLifecycle(
	ctx context.Context,
	recorder EventRecorder,
	id, turnID string,
) (compactionPersistedState, error) {
	rows, err := recorder.Query(ctx, store.EventQuery{})
	state := compactionPersistedState{originTurn: turnID}
	if err != nil {
		return state, fmt.Errorf("session: read compaction lifecycle: %w", err)
	}
	for _, row := range rows {
		if row.Type != acp.EventTypeCompaction && row.Type != events.SessionCompactionFired {
			continue
		}
		stored, err := transcript.UnmarshalAgentEvent(row.Content)
		if err != nil {
			return state, err
		}
		if row.Type == events.SessionCompactionFired {
			var fired CompactionFiredPayload
			if err := json.Unmarshal(stored.Raw, &fired); err != nil {
				return state, err
			}
			if fired.CompactionID == id {
				state.payload = fired
				state.attributed = true
			}
		} else if stored.Compaction != nil {
			if stored.Compaction.CompactionID == id && state.firstSequence == 0 {
				state.originTurn = row.TurnID
			}
			state.observeSnapshot(stored.Compaction, row.Sequence, id)
		}
	}
	for _, row := range rows {
		if row.Type == sessionCompactionRequestedEvent && row.TurnID == state.originTurn {
			state.requested = true
			break
		}
	}
	return state, nil
}

func (s *compactionPersistedState) observeSnapshot(snapshot *acp.CompactionObservation, sequence int64, id string) {
	if snapshot.CompactionID != id {
		if s.firstSequence == 0 && snapshot.Terminal {
			s.boundary = new(sequence)
		}
		return
	}
	if s.firstSequence == 0 {
		s.firstSequence = sequence
	}
	s.previous = snapshot
	if snapshot.Terminal && s.terminal == nil {
		s.terminal = snapshot
	}
}

func (m *Manager) fillCompactionContext(
	ctx context.Context,
	id string,
	payload *CompactionFiredPayload,
	state compactionPersistedState,
) error {
	usage, err := m.UsageEvents(ctx, id)
	if err != nil {
		return err
	}
	for _, reading := range usage {
		if state.firstSequence > 0 && reading.Sequence >= state.firstSequence {
			break
		}
		if (state.boundary == nil || reading.Sequence > *state.boundary) && reading.Usage.ContextUsed != nil {
			payload.ContextUsed = reading.Usage.ContextUsed
			payload.ContextSize = reading.Usage.ContextSize
		}
	}
	return nil
}

func compactionTerminalStatus(status string) bool {
	return status == "completed" || status == "failed" ||
		status == "cancelled" //nolint:misspell // ACP status spelling.
}

func (m *Manager) dispatchObservedCompaction(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
	lifecycle compactionLifecycle,
) error {
	if event.Compaction == nil || event.Type != acp.EventTypeCompaction {
		return nil
	}
	payload := hookspkg.ContextCompactionPayload{
		Timestamp:      event.Timestamp,
		SessionContext: hookSessionContext(session),
		TurnID:         event.TurnID,
		CompactionID:   event.Compaction.CompactionID,
		Trigger:        lifecycle.payload.Trigger,
	}
	dispatcher := m.hooks.Compaction
	if dispatcher == nil {
		dispatcher = noopCompactionHooks{}
	}
	ctx = hookDispatchContext(ctx, m, session)
	if lifecycle.first {
		raw, err := json.Marshal(lifecycle.payload)
		if err != nil {
			return err
		}
		if err := m.recordEvent(
			ctx,
			session,
			acp.AgentEvent{
				Type:      events.SessionCompactionFired,
				TurnID:    event.TurnID,
				Timestamp: event.Timestamp,
				Raw:       raw,
			},
		); err != nil {
			return err
		}
		payload.Event = hookspkg.HookContextPreCompact
		if _, err := dispatcher.DispatchContextPreCompact(ctx, payload); err != nil {
			m.warnHookDispatch(ctx, session, payload.Event, err)
		}
	}
	if lifecycle.terminal {
		payload.Event = hookspkg.HookContextPostCompact
		post := event.Compaction
		if lifecycle.post != nil {
			post = lifecycle.post
		}
		payload.Status = post.Status
		payload.Summary = post.Summary
		payload.Error = post.Error
		if _, err := dispatcher.DispatchContextPostCompact(ctx, payload); err != nil {
			m.warnHookDispatch(ctx, session, payload.Event, err)
		}
	}
	return nil
}
