package heartbeat

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	automationEventFenceOpen  = "<untrusted_automation_event>\n"
	automationEventFenceClose = "\n</untrusted_automation_event>"
)

// readableFencedSummary recovers the event body from an admitted digest item's fence and reads it.
func readableFencedSummary(fenced string) string {
	quoted, ok := strings.CutPrefix(fenced, automationEventFenceOpen)
	if !ok {
		return ""
	}
	quoted, ok = strings.CutSuffix(quoted, automationEventFenceClose)
	if !ok {
		return ""
	}
	raw, err := strconv.Unquote(quoted)
	if err != nil {
		return ""
	}
	return ReadableAutomationEventSummary(raw)
}

// AutomationEventReadableBytes bounds the operator-facing line an event-derived proposal shows.
const AutomationEventReadableBytes = 280

// automationEventEnvelopeKeys are delivery plumbing, not what happened, so the readable line skips them.
var automationEventEnvelopeKeys = map[string]bool{
	"prompt": true, "webhook_id": true, "delivery_id": true, "endpoint": true, "endpoint_slug": true,
	"timestamp": true,
}

// ReadableAutomationEventSummary turns a stored event body into one short plain line for the
// operator: the event's own scalar fields as "key: value", without delivery identifiers, raw JSON
// or nested structures. The result is untrusted event text and must be shown as plain text only.
// A body that is not a JSON object reads as its first line, collapsed and bounded.
func ReadableAutomationEventSummary(raw string) string {
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		if trimmed := strings.TrimSpace(raw); strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			// A cut-off envelope is structure, not text: never show it raw.
			return ""
		}
		return boundReadable(strings.Join(strings.Fields(raw), " "))
	}
	if data, ok := body["data"].(map[string]any); ok {
		body = data
	}
	var parts []string
	for _, key := range slices.Sorted(maps.Keys(body)) {
		if automationEventEnvelopeKeys[key] || isTransportPayloadCopy(body, key) {
			continue
		}
		switch value := body[key].(type) {
		case string:
			if text := strings.Join(strings.Fields(value), " "); text != "" {
				parts = append(parts, key+": "+text)
			}
		case float64, bool:
			parts = append(parts, fmt.Sprintf("%s: %v", key, value))
		}
	}
	return boundReadable(strings.Join(parts, " · "))
}

func boundReadable(text string) string {
	if len(text) <= AutomationEventReadableBytes {
		return text
	}
	text = text[:AutomationEventReadableBytes]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + "…"
}

// isTransportPayloadCopy reports the webhook transport's raw `payload` copy of a JSON object body
// whose fields the event already carries decoded. A payload with its own content stays.
func isTransportPayloadCopy(body map[string]any, key string) bool {
	text, ok := body[key].(string)
	if key != "payload" || !ok {
		return false
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return false
	}
	for field := range decoded {
		if _, present := body[field]; !present {
			return false
		}
	}
	return true
}

// automationEventCompactFloor is the shortest string the compactor still halves.
const automationEventCompactFloor = 32

// CompactAutomationEventSummary shrinks a JSON event envelope structurally to fit the stored
// summary limit, so storage never cuts it into unparseable text. It drops the transport's
// duplicate `payload` copy, then halves the longest strings. Non-JSON input is returned unchanged.
func CompactAutomationEventSummary(raw string) string {
	if len(raw) <= AutomationEventSummaryBytes {
		return raw
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return raw
	}
	if data, ok := envelope["data"].(map[string]any); ok && isTransportPayloadCopy(data, "payload") {
		delete(data, "payload")
	}
	for range 64 {
		encoded, err := json.Marshal(envelope)
		if err != nil {
			return raw
		}
		if len(encoded) <= AutomationEventSummaryBytes || !halveLongestString(envelope) {
			return string(encoded)
		}
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return raw
	}
	return string(encoded)
}

// halveLongestString shortens the longest string value in place; false when none is worth shrinking.
func halveLongestString(node any) bool {
	var (
		owner   map[string]any
		key     string
		longest int
	)
	var walk func(value any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for name, child := range typed {
				if text, ok := child.(string); ok && len(text) > longest {
					owner, key, longest = typed, name, len(text)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(node)
	if owner == nil || longest < automationEventCompactFloor {
		return false
	}
	text, ok := owner[key].(string)
	if !ok {
		return false
	}
	cut := len(text) / 2
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	owner[key] = text[:cut] + "…"
	return true
}
