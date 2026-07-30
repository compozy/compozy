package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/acp"

	bridgepkg "github.com/compozy/compozy/internal/bridges"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"

	"github.com/compozy/compozy/internal/transcript"
)

type hostAPIPromptSubmission struct {
	TurnID             string
	SeedEvents         []bridgepkg.DeliveryProjectionEvent
	DeliveryRegistered bool
}

func (h *HostAPIHandler) submitPrompt(
	ctx context.Context,
	sessionID string,
	message string,
	runtime *session.RuntimeSelection,
) (hostAPIPromptSubmission, error) {
	if h.sessions == nil {
		return hostAPIPromptSubmission{}, errors.New("extension: session manager is not configured")
	}

	lastSequence, err := h.latestSessionSequence(ctx, sessionID)
	if err != nil {
		return hostAPIPromptSubmission{}, err
	}

	promptCtx := context.WithoutCancel(ctx)
	eventsCh, err := h.submitRuntimePrompt(promptCtx, sessionID, message, runtime)
	if err != nil {
		return hostAPIPromptSubmission{}, err
	}
	drainAgentEvents(eventsCh)

	events, err := h.sessions.Events(ctx, sessionID, store.EventQuery{
		AfterSequence: lastSequence,
	})
	if err != nil {
		return hostAPIPromptSubmission{}, err
	}

	return promptSubmissionFromStoredEvents(events)
}

func (h *HostAPIHandler) submitRuntimePrompt(
	ctx context.Context,
	sessionID string,
	message string,
	runtime *session.RuntimeSelection,
) (<-chan acp.AgentEvent, error) {
	if runtime == nil {
		return h.sessions.Prompt(ctx, sessionID, message)
	}
	prompter, ok := h.sessions.(hostAPIRuntimePromptSessionManager)
	if !ok {
		return nil, errors.New("extension: session manager does not support prompt runtime selection")
	}
	result, err := prompter.SendPrompt(ctx, sessionID, session.SendPromptOpts{
		Message: message,
		Runtime: runtime,
	})
	if err != nil {
		return nil, err
	}
	if result.Events == nil {
		return nil, errors.New("extension: prompt runtime selection was not admitted for immediate delivery")
	}
	return result.Events, nil
}

func promptSubmissionFromStoredEvents(events []store.SessionEvent) (hostAPIPromptSubmission, error) {
	turnID := promptTurnIDFromStoredEvents(events)
	if turnID == "" {
		return hostAPIPromptSubmission{}, errors.New("extension: prompt turn id not found after prompt submission")
	}

	seedEvents, err := promptSeedEventsFromStoredEvents(events, turnID)
	if err != nil {
		return hostAPIPromptSubmission{}, err
	}

	return hostAPIPromptSubmission{
		TurnID:     turnID,
		SeedEvents: seedEvents,
	}, nil
}

func promptTurnIDFromStoredEvents(events []store.SessionEvent) string {
	for _, event := range events {
		if !isPromptInitiatingStoredEventType(event.Type) {
			continue
		}
		turnID := strings.TrimSpace(event.TurnID)
		if turnID == "" {
			continue
		}
		return turnID
	}
	return ""
}

func isPromptInitiatingStoredEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case acp.EventTypeUserMessage, acp.EventTypeSyntheticReentry:
		return true
	default:
		return false
	}
}

func promptSeedEventsFromStoredEvents(
	events []store.SessionEvent,
	turnID string,
) ([]bridgepkg.DeliveryProjectionEvent, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" || len(events) == 0 {
		return nil, nil
	}

	seedEvents := make([]bridgepkg.DeliveryProjectionEvent, 0, len(events))
	for _, storedEvent := range events {
		if strings.TrimSpace(storedEvent.TurnID) != turnID {
			continue
		}

		projected, err := promptProjectionEventFromStoredEvent(storedEvent)
		if err != nil {
			return nil, err
		}
		seedEvents = append(seedEvents, projected)
	}
	return seedEvents, nil
}

func promptProjectionEventFromStoredEvent(storedEvent store.SessionEvent) (bridgepkg.DeliveryProjectionEvent, error) {
	decoded, err := transcript.UnmarshalAgentEvent(storedEvent.Content)
	if err != nil {
		return bridgepkg.DeliveryProjectionEvent{}, fmt.Errorf("extension: decode prompt seed event: %w", err)
	}
	if strings.TrimSpace(decoded.Type) == "" {
		decoded.Type = strings.TrimSpace(storedEvent.Type)
	}
	if strings.TrimSpace(decoded.TurnID) == "" {
		decoded.TurnID = strings.TrimSpace(storedEvent.TurnID)
	}
	if decoded.Timestamp.IsZero() {
		decoded.Timestamp = storedEvent.Timestamp
	}

	fingerprint, err := canonicalProjectionFingerprint(storedEvent.Content)
	if err != nil {
		return bridgepkg.DeliveryProjectionEvent{}, fmt.Errorf(
			"extension: fingerprint prompt seed event: %w",
			err,
		)
	}
	return projectionEventFromCanonicalAgentEvent(&decoded, fingerprint), nil
}

func (h *HostAPIHandler) latestSessionSequence(ctx context.Context, sessionID string) (int64, error) {
	events, err := h.sessions.Events(ctx, sessionID, store.EventQuery{Limit: 1})
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	return events[len(events)-1].Sequence, nil
}
