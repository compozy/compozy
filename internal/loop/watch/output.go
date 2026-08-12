package watch

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"
)

const (
	outputKindPending              = "watch_source_pending"
	outputKindConfirmed            = "watch_source_confirmed"
	outputKindWatchEventsPending   = "watch_events_pending"
	outputKindWatchEventsConfirmed = "watch_events_confirmed"

	// EventsCursorVersion namespaces durable watch-event cursor values.
	EventsCursorVersion = 1
)

// OutputRef is the JSON payload persisted in loop_generation_outputs.output_ref
// for coordinator-owned watch nodes.
type OutputRef struct {
	Kind          string                 `json:"kind"`
	StateDigest   string                 `json:"state_digest,omitempty"`
	EventKey      string                 `json:"event_key,omitempty"`
	Payload       json.RawMessage        `json:"payload,omitempty"`
	SettledAt     *time.Time             `json:"settled_at,omitempty"`
	Subscriptions []EventSubscriptionRef `json:"subscriptions,omitempty"`
	Cursors       map[string]int64       `json:"cursors,omitempty"`
	CursorVersion int                    `json:"cursor_version,omitempty"`
	Events        json.RawMessage        `json:"events,omitempty"`
}

// EventSubscriptionRef is the durable subscription shape stored while a
// watch-events node is parked.
type EventSubscriptionRef struct {
	Kind   string `json:"kind"`
	Filter string `json:"filter,omitempty"`
}

// EventsPendingState is the parked watch-events output_ref state.
type EventsPendingState struct {
	Subscriptions []EventSubscriptionRef `json:"subscriptions"`
	Cursors       map[string]int64       `json:"cursors"`
	CursorVersion int                    `json:"cursor_version"`
}

// PendingOutputRef stores the last observed source digest while a loop is watching.
func PendingOutputRef(response PollResponse) (string, error) {
	return marshalOutputRef(OutputRef{
		Kind:        outputKindPending,
		EventKey:    strings.TrimSpace(response.EventKey),
		StateDigest: strings.TrimSpace(response.StateDigest),
	})
}

// ConfirmedOutputRef stores the confirmed ready payload as the source node output.
func ConfirmedOutputRef(response PollResponse) (string, error) {
	return marshalOutputRef(OutputRef{
		Kind:        outputKindConfirmed,
		EventKey:    strings.TrimSpace(response.EventKey),
		StateDigest: strings.TrimSpace(response.StateDigest),
		Payload:     cloneRawMessage(response.Payload),
		SettledAt:   response.SettledAt,
	})
}

// ExpectedStateDigestFromOutputRef recovers the prior state digest for the next poll.
func ExpectedStateDigestFromOutputRef(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", nil
	}
	var output OutputRef
	if err := json.Unmarshal([]byte(trimmed), &output); err != nil {
		return "", fmt.Errorf("loop watch output ref decode: %w", err)
	}
	if strings.TrimSpace(output.Kind) != outputKindPending {
		return "", nil
	}
	return strings.TrimSpace(output.StateDigest), nil
}

// EventsPendingOutputRef stores rendered subscriptions and stream cursors
// while a watch-events loop is dormant.
func EventsPendingOutputRef(state EventsPendingState) (string, error) {
	return marshalOutputRef(OutputRef{
		Kind:          outputKindWatchEventsPending,
		Subscriptions: cloneEventSubscriptionRefs(state.Subscriptions),
		Cursors:       cloneCursors(state.Cursors),
		CursorVersion: EventsCursorVersion,
	})
}

// EventsPendingFromOutputRef recovers parked watch-events state.
func EventsPendingFromOutputRef(ref string) (EventsPendingState, bool, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return EventsPendingState{}, false, nil
	}
	var output OutputRef
	if err := json.Unmarshal([]byte(trimmed), &output); err != nil {
		return EventsPendingState{}, false, fmt.Errorf("loop watch-events output ref decode: %w", err)
	}
	if strings.TrimSpace(output.Kind) != outputKindWatchEventsPending {
		return EventsPendingState{}, false, nil
	}
	if output.CursorVersion != EventsCursorVersion {
		return EventsPendingState{}, false, fmt.Errorf(
			"loop watch-events cursor version %d is unsupported",
			output.CursorVersion,
		)
	}
	return EventsPendingState{
		Subscriptions: cloneEventSubscriptionRefs(output.Subscriptions),
		Cursors:       cloneCursors(output.Cursors),
		CursorVersion: output.CursorVersion,
	}, true, nil
}

// EventsConfirmedOutputPayload returns the complete confirmed watch-events
// JSON payload. Callers may persist it inline or by content-addressed output ref.
func EventsConfirmedOutputPayload(events any, cursors map[string]int64) (json.RawMessage, error) {
	rawEvents, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("loop watch-events output events: %w", err)
	}
	payload, err := json.Marshal(OutputRef{
		Kind:    outputKindWatchEventsConfirmed,
		Events:  rawEvents,
		Cursors: cloneCursors(cursors),
	})
	if err != nil {
		return nil, fmt.Errorf("loop watch-events output ref: %w", err)
	}
	return payload, nil
}

// EventsConfirmedOutputRef stores the confirmed watch-events batch inline.
func EventsConfirmedOutputRef(events any, cursors map[string]int64) (string, error) {
	payload, err := EventsConfirmedOutputPayload(events, cursors)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func marshalOutputRef(output OutputRef) (string, error) {
	if strings.TrimSpace(output.Kind) == "" {
		return "", errors.New("loop watch output kind is required")
	}
	payload, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("loop watch output ref: %w", err)
	}
	return string(payload), nil
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), src...)
}

func cloneEventSubscriptionRefs(src []EventSubscriptionRef) []EventSubscriptionRef {
	if len(src) == 0 {
		return nil
	}
	return append([]EventSubscriptionRef(nil), src...)
}

func cloneCursors(src map[string]int64) map[string]int64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]int64, len(src))
	maps.Copy(dst, src)
	return dst
}
