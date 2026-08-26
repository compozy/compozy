package cli

import "encoding/json"

func marshalStructuredPayload(args []string, payload any) ([]byte, bool) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	switch requestedOutputFormat(args) {
	case OutputJSON:
		return encoded, true
	case OutputJSONL:
		return append(encoded, '\n'), true
	default:
		return nil, false
	}
}
