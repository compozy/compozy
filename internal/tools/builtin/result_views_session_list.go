package builtin

import "encoding/json/v2"

func sessionListCompactOutputSchema() string {
	var record map[string]any
	if err := json.Unmarshal([]byte(sessionCompactRecordSchema), &record); err != nil {
		panic("invariant: invalid compact session schema: " + err.Error())
	}
	properties, ok := record["properties"].(map[string]any)
	if !ok {
		panic("invariant: compact session schema has no properties")
	}
	properties["folder_id"] = map[string]any{"type": "string"}
	var health map[string]any
	if err := json.Unmarshal([]byte(sessionListHealthSchema), &health); err != nil {
		panic("invariant: invalid session health schema: " + err.Error())
	}
	properties["health"] = health
	return `{"type":"object","required":["sessions","page"],"properties":{"sessions":{"type":"array","items":` +
		string(
			mustResultViewJSON(record),
		) + `},"page":{"type":"object","required":["has_more","limit"],"properties":{"next_cursor":{"type":"string"},"has_more":{"type":"boolean"},"total":{"type":"integer"},"limit":{"type":"integer"}},"additionalProperties":false}},"additionalProperties":false}`
}

const sessionListHealthSchema = `{"type":"object","properties":{"lifecycle_state":{"type":"string"},"attention":{"type":"string"},"session_id":{"type":"string"},"workspace_id":{"type":"string"},"agent_name":{"type":"string"},"state":{"type":"string"},"health":{"type":"string"},"ineligibility_reason":{"type":"string"},"last_activity_at":{"type":"string"},"last_presence_at":{"type":"string"},"last_error":{"type":"string"},"updated_at":{"type":"string"},"verified":{"type":"boolean"},"escalated":{"type":"boolean"},"active_prompt":{"type":"boolean"},"attachable":{"type":"boolean"},"eligible_for_wake":{"type":"boolean"}},"required":["session_id","workspace_id","agent_name","state","health","active_prompt","attachable","eligible_for_wake","updated_at"],"additionalProperties":false}`
