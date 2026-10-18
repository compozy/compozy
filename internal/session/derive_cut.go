package session

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// deriveTurn is one turn of the source ledger, in order of its first event.
type deriveTurn struct {
	id            string
	firstSequence int64
	lastSequence  int64
	settled       bool
	// prompt reports a prompt turn (user or synthetic): one that carries a prompt-stream
	// event. Lifecycle-only turn ids (hook dispatch, stop escalation, session stopped) are
	// not prompt turns and never count as a turn in progress.
	prompt bool
}

// deriveTurns groups events by turn id in first-appearance order. A turn is settled when
// it carries a terminal event (done or error) or a prompt-settlement marker (cancel,
// timeout, interrupt, supersede); its through sequence is its last event, so markers the
// session records after the terminal event stay inside the cut.
func deriveTurns(events []store.SessionEvent) []deriveTurn {
	index := make(map[string]int)
	turns := make([]deriveTurn, 0)
	for _, event := range events {
		turnID := strings.TrimSpace(event.TurnID)
		if turnID == "" {
			continue
		}
		position, ok := index[turnID]
		if !ok {
			position = len(turns)
			index[turnID] = position
			turns = append(turns, deriveTurn{id: turnID, firstSequence: event.Sequence})
		}
		turn := &turns[position]
		turn.lastSequence = max(turn.lastSequence, event.Sequence)
		if deriveTerminalEvent(event) {
			turn.settled = true
		}
		if derivePromptEvent(event.Type) {
			turn.prompt = true
		}
	}
	return turns
}

// derivePromptEvent reports the event types a prompt turn records (the prompt itself,
// its delivery, agent output, and its terminal event).
func derivePromptEvent(eventType string) bool {
	switch eventType {
	case acp.EventTypeUserMessage, acp.EventTypeSyntheticReentry, acp.EventTypePromptDelivery,
		acp.EventTypeAgentMessage, acp.EventTypeThought, acp.EventTypeToolCall, acp.EventTypeToolResult,
		acp.EventTypePlan, acp.EventTypePermission, acp.EventTypeClarify, acp.EventTypeDone, acp.EventTypeError:
		return true
	default:
		return false
	}
}

func deriveTerminalEvent(event store.SessionEvent) bool {
	switch event.Type {
	case acp.EventTypeDone, acp.EventTypeError:
		return true
	}
	if !strings.HasPrefix(event.Type, "transcript_marker") {
		return false
	}
	var payload struct {
		Kind   string `json:"kind"`
		Marker *struct {
			Kind string `json:"kind"`
		} `json:"marker"`
	}
	if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
		return false
	}
	kind := payload.Kind
	if payload.Marker != nil && strings.TrimSpace(payload.Marker.Kind) != "" {
		kind = payload.Marker.Kind
	}
	switch kind {
	case transcript.MarkerPromptCancel, transcript.MarkerPromptTimeout,
		transcript.MarkerPromptInterrupted, transcript.MarkerPromptSuperseded:
		return true
	default:
		return false
	}
}

// lastSettledTurn returns the last settled turn (user or synthetic), its through
// sequence, and whether a later prompt turn is still open.
func lastSettledTurn(events []store.SessionEvent) (string, int64, bool) {
	turns := deriveTurns(events)
	last := -1
	for position, turn := range turns {
		if turn.settled {
			last = position
		}
	}
	laterOpen := false
	for position := last + 1; position < len(turns); position++ {
		if turns[position].prompt && !turns[position].settled {
			laterOpen = true
		}
	}
	if last < 0 {
		return "", 0, laterOpen
	}
	return turns[last].id, turns[last].lastSequence, laterOpen
}

// resolveDeriveCut maps a durable user anchor to its turn and that turn's terminal
// event (inclusive: the message, the agent's reply, and its tool work); a turn without a
// terminal event is ErrDeriveTurnInProgress. The anchor's turn id comes from its
// transcript entry; its start event is the fallback for entries written without one.
func resolveDeriveCut(anchor store.TranscriptUserAnchor, events []store.SessionEvent) (DeriveCut, error) {
	messageID := strings.TrimSpace(anchor.MessageID)
	turnID := strings.TrimSpace(anchor.TurnID)
	if turnID == "" {
		for _, event := range events {
			if event.Sequence == anchor.StartSequence {
				turnID = strings.TrimSpace(event.TurnID)
				break
			}
		}
	}
	if turnID == "" {
		return DeriveCut{}, deriveErr(ErrDeriveMessageNotFound, "message %s not found in session", messageID)
	}
	for _, turn := range deriveTurns(events) {
		if turn.id != turnID {
			continue
		}
		cut := DeriveCut{
			MessageID: messageID, TurnID: turnID, ThroughSequence: turn.lastSequence, TurnSettled: turn.settled,
		}
		if !turn.settled {
			return cut, deriveErr(
				ErrDeriveTurnInProgress,
				"the turn started by %s has not settled; fork from an earlier message or wait", messageID,
			)
		}
		return cut, nil
	}
	return DeriveCut{}, deriveErr(ErrDeriveMessageNotFound, "message %s not found in session", messageID)
}

// carriedEvents keeps the events through the cut, excluding every unsettled turn.
func carriedEvents(events []store.SessionEvent, afterSequence int64, throughSequence int64) []store.SessionEvent {
	open := make(map[string]struct{})
	for _, turn := range deriveTurns(events) {
		if !turn.settled {
			open[turn.id] = struct{}{}
		}
	}
	carried := make([]store.SessionEvent, 0, len(events))
	for _, event := range events {
		if event.Sequence <= afterSequence || event.Sequence > throughSequence {
			continue
		}
		if _, skip := open[strings.TrimSpace(event.TurnID)]; skip {
			continue
		}
		carried = append(carried, event)
	}
	return carried
}
