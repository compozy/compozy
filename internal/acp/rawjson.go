package acp

import "encoding/json"

// CloneRawMessage returns an independent copy of one raw JSON payload.
func CloneRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}
