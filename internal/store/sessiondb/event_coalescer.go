package sessiondb

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

const (
	sessionEventTypeAgentMessage = "agent_message"
	sessionEventTypeThought      = "thought"

	sessionEventPayloadSchemaKey     = "schema"
	sessionEventPayloadTypeKey       = "type"
	sessionEventPayloadTextKey       = "text"
	sessionEventPayloadTimestampKey  = "timestamp"
	sessionEventPayloadTurnIDKey     = "turn_id"
	sessionEventPayloadToolResultKey = "tool_result"
	sessionEventPayloadErrorKey      = "error"
)

var coalescingPayloadBlocklist = map[string]struct{}{
	"title":                          {},
	"tool_name":                      {},
	"tool_call_id":                   {},
	"tool_input":                     {},
	sessionEventPayloadToolResultKey: {},
	"tool_error":                     {},
	"stop_reason":                    {},
	"action":                         {},
	"resource":                       {},
	"decision":                       {},
	sessionEventPayloadErrorKey:      {},
	"failure":                        {},
	"synthetic":                      {},
	"usage":                          {},
	"runtime":                        {},
	"raw":                            {},
}

type coalescibleSessionEvent struct {
	event     store.SessionEvent
	payload   map[string]json.RawMessage
	key       string
	text      string
	timestamp time.Time
}

type pendingCoalescedEvent struct {
	event     store.SessionEvent
	payload   map[string]json.RawMessage
	key       string
	text      strings.Builder
	timestamp time.Time
}

func coalesceSessionEventBatch(events []store.SessionEvent) ([]store.SessionEvent, error) {
	if len(events) == 0 {
		return nil, nil
	}
	out := make([]store.SessionEvent, 0, len(events))
	var pending *pendingCoalescedEvent

	flush := func() error {
		if pending == nil {
			return nil
		}
		event, err := pending.build()
		if err != nil {
			return err
		}
		out = append(out, event)
		pending = nil
		return nil
	}

	for _, event := range events {
		candidate, ok, err := newCoalescibleSessionEvent(event)
		if err != nil {
			return nil, err
		}
		if !ok {
			if err := flush(); err != nil {
				return nil, err
			}
			out = append(out, event)
			continue
		}
		if pending != nil && pending.key == candidate.key {
			pending.append(candidate)
			continue
		}
		if err := flush(); err != nil {
			return nil, err
		}
		pending = newPendingCoalescedEvent(candidate)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

func newCoalescibleSessionEvent(event store.SessionEvent) (coalescibleSessionEvent, bool, error) {
	if !coalescibleSessionEventType(event.Type) || strings.TrimSpace(event.Content) == "" {
		return coalescibleSessionEvent{}, false, nil
	}
	payload := make(map[string]json.RawMessage)
	if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
		return coalescibleSessionEvent{}, false, nil
	}
	if !coalescibleCanonicalPayload(event, payload) {
		return coalescibleSessionEvent{}, false, nil
	}
	text, ok := jsonPayloadString(payload, sessionEventPayloadTextKey)
	if !ok || strings.TrimSpace(text) == "" {
		return coalescibleSessionEvent{}, false, nil
	}
	key, err := coalesciblePayloadKey(event, payload)
	if err != nil {
		return coalescibleSessionEvent{}, false, err
	}
	return coalescibleSessionEvent{
		event:     event,
		payload:   clonePayloadMap(payload),
		key:       key,
		text:      text,
		timestamp: payloadTimestamp(payload, event.Timestamp),
	}, true, nil
}

func coalescibleSessionEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case sessionEventTypeAgentMessage, sessionEventTypeThought:
		return true
	default:
		return false
	}
}

func coalescibleCanonicalPayload(event store.SessionEvent, payload map[string]json.RawMessage) bool {
	schema, ok := jsonPayloadString(payload, sessionEventPayloadSchemaKey)
	if !ok || strings.TrimSpace(schema) != canonicalEventSchema {
		return false
	}
	eventType, ok := jsonPayloadString(payload, sessionEventPayloadTypeKey)
	if !ok || strings.TrimSpace(eventType) != strings.TrimSpace(event.Type) {
		return false
	}
	if turnID, ok := jsonPayloadString(payload, sessionEventPayloadTurnIDKey); ok &&
		strings.TrimSpace(turnID) != strings.TrimSpace(event.TurnID) {
		return false
	}
	for key, raw := range payload {
		if _, blocked := coalescingPayloadBlocklist[key]; blocked && jsonPayloadHasValue(raw) {
			return false
		}
	}
	return true
}

func coalesciblePayloadKey(event store.SessionEvent, payload map[string]json.RawMessage) (string, error) {
	keyPayload := clonePayloadMap(payload)
	delete(keyPayload, sessionEventPayloadTextKey)
	delete(keyPayload, sessionEventPayloadTimestampKey)
	data, err := json.Marshal(keyPayload)
	if err != nil {
		return "", fmt.Errorf("store: marshal coalescible session event key: %w", err)
	}
	return strings.Join([]string{
		strings.TrimSpace(event.SessionID),
		strings.TrimSpace(event.TurnID),
		strings.TrimSpace(event.Type),
		strings.TrimSpace(event.AgentName),
		string(data),
	}, "\x00"), nil
}

func newPendingCoalescedEvent(candidate coalescibleSessionEvent) *pendingCoalescedEvent {
	pending := &pendingCoalescedEvent{
		event:     candidate.event,
		payload:   clonePayloadMap(candidate.payload),
		key:       candidate.key,
		timestamp: candidate.timestamp,
	}
	pending.text.WriteString(candidate.text)
	return pending
}

func (p *pendingCoalescedEvent) append(candidate coalescibleSessionEvent) {
	p.text.WriteString(candidate.text)
	p.timestamp = candidate.timestamp
	if !candidate.event.Timestamp.IsZero() {
		p.event.Timestamp = candidate.event.Timestamp
	}
}

func (p *pendingCoalescedEvent) build() (store.SessionEvent, error) {
	if p == nil {
		return store.SessionEvent{}, nil
	}
	if err := setPayloadString(p.payload, sessionEventPayloadTextKey, p.text.String()); err != nil {
		return store.SessionEvent{}, err
	}
	if err := setPayloadTime(p.payload, sessionEventPayloadTimestampKey, p.timestamp); err != nil {
		return store.SessionEvent{}, err
	}
	data, err := json.Marshal(p.payload)
	if err != nil {
		return store.SessionEvent{}, fmt.Errorf("store: marshal coalesced session event payload: %w", err)
	}
	p.event.Content = string(data)
	p.event.Timestamp = p.timestamp
	return p.event, nil
}

func jsonPayloadString(payload map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := payload[key]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func jsonPayloadHasValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" &&
		trimmed != "null" &&
		trimmed != `""` &&
		trimmed != "false" &&
		trimmed != "{}" &&
		trimmed != "[]"
}

func payloadTimestamp(payload map[string]json.RawMessage, fallback time.Time) time.Time {
	raw, ok := payload[sessionEventPayloadTimestampKey]
	if !ok {
		return fallback
	}
	var value time.Time
	if err := json.Unmarshal(raw, &value); err != nil || value.IsZero() {
		return fallback
	}
	return value.UTC()
}

func setPayloadString(payload map[string]json.RawMessage, key string, value string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("store: marshal coalesced session event %s: %w", key, err)
	}
	payload[key] = data
	return nil
}

func setPayloadTime(payload map[string]json.RawMessage, key string, value time.Time) error {
	data, err := json.Marshal(value.UTC())
	if err != nil {
		return fmt.Errorf("store: marshal coalesced session event %s: %w", key, err)
	}
	payload[key] = data
	return nil
}

func clonePayloadMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	dst := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		dst[key] = append(json.RawMessage(nil), value...)
	}
	return dst
}
