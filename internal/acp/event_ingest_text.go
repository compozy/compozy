package acp

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

func ingestEventBytes(event AgentEvent) int {
	return len(event.Text) + len(event.Raw) + len(event.Title) + len(event.Error) + 256
}

func ingestTextKey(event AgentEvent) (string, bool) {
	if event.Type != EventTypeAgentMessage && event.Type != EventTypeThought {
		return "", false
	}
	if len(event.Raw) == 0 {
		return "", true
	}
	var raw map[string]any
	if json.Unmarshal(event.Raw, &raw) != nil {
		return "", false
	}
	content, ok := raw["content"].(map[string]any)
	if !ok || content["type"] != "text" || content["text"] != event.Text {
		return "", false
	}
	content["text"] = ""
	key, err := json.Marshal(raw)
	return string(key), err == nil
}

func equivalentIngestText(left, right AgentEvent) bool {
	if left.Type != EventTypeAgentMessage && left.Type != EventTypeThought {
		return false
	}
	left.Text, right.Text = "", ""
	left.Raw, right.Raw = nil, nil
	left.Timestamp, right.Timestamp = time.Time{}, time.Time{}
	return reflect.DeepEqual(left, right)
}

func materializeIngestText(entry *ingestEntry) (AgentEvent, error) {
	event := entry.event
	event.Text = entry.text.String()
	if entry.rawKey == "" || event.Text == entry.event.Text {
		return event, nil
	}
	// The key was decoded and encoded at admission, so it is a JSON object
	// containing text content. Marshal only JSON-native values from that key.
	var raw map[string]any
	if err := json.Unmarshal([]byte(entry.rawKey), &raw); err != nil {
		return AgentEvent{}, fmt.Errorf("acp: decode admitted text key: %w", err)
	}
	content, ok := raw["content"].(map[string]any)
	if !ok {
		return AgentEvent{}, fmt.Errorf("acp: admitted text key is missing content")
	}
	content["text"] = event.Text
	encoded, err := json.Marshal(raw)
	if err != nil {
		return AgentEvent{}, fmt.Errorf("acp: encode admitted text content: %w", err)
	}
	event.Raw = encoded
	return event, nil
}
