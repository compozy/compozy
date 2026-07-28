package transcript

import "encoding/json"

// toolResultDataPayload preserves the canonical ToolResult when projecting a
// persisted result into the AI SDK tool-part envelope. UnmarshalAgentEvent
// restores tool identity but intentionally does not recreate the provider's
// original raw ACP update, so the parsed canonical result is the replay source
// of truth.
func (d *decodedStoredEvent) toolResultDataPayload() json.RawMessage {
	if d == nil || d.parsed.ToolResult == nil {
		return d.dataPayload()
	}

	result, err := json.Marshal(d.parsed.ToolResult)
	if err != nil {
		return d.dataPayload()
	}
	payload := UIAgentEventPayloadFromEvent(d.agent)
	payload.Raw = result
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}
