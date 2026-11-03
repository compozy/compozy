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
		return boundReadable(strings.Join(strings.Fields(raw), " "))
	}
	if data, ok := body["data"].(map[string]any); ok {
		body = data
	}
	var parts []string
	for _, key := range slices.Sorted(maps.Keys(body)) {
		if automationEventEnvelopeKeys[key] {
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
