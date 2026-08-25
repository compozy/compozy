package contracts

import "encoding/json"

// UnwrapSingleObject removes one object-valued wrapper introduced by an agent.
// It preserves the original payload when the value is not exactly one nested object.
func UnwrapSingleObject(raw json.RawMessage) json.RawMessage {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || len(object) != 1 {
		return cloneRaw(raw)
	}
	for _, child := range object {
		var nested map[string]any
		if err := json.Unmarshal(child, &nested); err == nil && nested != nil {
			return cloneRaw(child)
		}
	}
	return cloneRaw(raw)
}
