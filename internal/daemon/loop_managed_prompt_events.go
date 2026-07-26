package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	eventspkg "github.com/compozy/agh/internal/events"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

func readManagedGoalPromptOutput(
	ctx context.Context,
	reader loopSessionEventReader,
	entry store.SessionInputQueueEntry,
	result looppkg.ActionPromptResult,
) (string, json.RawMessage, error) {
	events, err := reader.Events(ctx, entry.SessionID, store.EventQuery{TurnID: entry.PromptID})
	if err != nil {
		return "", nil, fmt.Errorf("daemon: read managed Goal prompt events: %w", err)
	}
	if len(events) == 0 {
		return "", nil, fmt.Errorf("%w: managed Goal terminal has no correlated events", looppkg.ErrTransitionConflict)
	}
	var text strings.Builder
	var structured json.RawMessage
	terminalCount := 0
	previousSequence := int64(0)
	firstSequence := int64(0)
	terminalSequence := int64(0)
	for _, event := range events {
		if event.TurnID != entry.PromptID ||
			event.Sequence <= 0 ||
			(previousSequence > 0 && event.Sequence <= previousSequence) {
			return "", nil, fmt.Errorf("%w: managed Goal event correlation changed", looppkg.ErrTransitionConflict)
		}
		previousSequence = event.Sequence
		agentEvent, decodeErr := transcript.UnmarshalAgentEvent(event.Content)
		if decodeErr != nil {
			return "", nil, fmt.Errorf("daemon: decode managed Goal prompt event %d: %w", event.Sequence, decodeErr)
		}
		if isManagedGoalProofAuxiliaryEvent(agentEvent.Type) {
			continue
		}
		if terminalCount > 0 {
			return "", nil, fmt.Errorf("%w: managed Goal terminal event is not final", looppkg.ErrTransitionConflict)
		}
		if firstSequence == 0 {
			firstSequence = event.Sequence
		}
		if agentEvent.Type == acp.EventTypeAgentMessage && strings.TrimSpace(agentEvent.Text) != "" {
			if text.Len() > 0 {
				text.WriteByte('\n')
			}
			text.WriteString(agentEvent.Text)
		}
		if isManagedGoalTerminalEvent(agentEvent.Type) {
			terminalCount++
			terminalSequence = event.Sequence
		}
	}
	if terminalCount != 1 {
		return "", nil, fmt.Errorf("%w: managed Goal event range requires one terminal", looppkg.ErrTransitionConflict)
	}
	if firstSequence != result.EventStartSeq || terminalSequence != result.EventEndSeq {
		return "", nil, fmt.Errorf("%w: managed Goal event range changed", looppkg.ErrTransitionConflict)
	}
	candidate := bytes.TrimSpace([]byte(text.String()))
	if json.Valid(candidate) {
		structured = append(structured[:0], candidate...)
	}
	return text.String(), structured, nil
}

func isManagedGoalTerminalEvent(eventType string) bool {
	return eventType == acp.EventTypeDone || eventType == acp.EventTypeError
}

func isManagedGoalProofAuxiliaryEvent(eventType string) bool {
	return eventType == eventspkg.TranscriptMarkerCreated || eventType == eventspkg.TranscriptMarkerRedacted
}
