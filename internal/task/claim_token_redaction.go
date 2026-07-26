package task

import (
	"encoding/json"
	"strings"
)

// RedactClaimTokenJSON removes raw claim-token fields and values from a JSON payload.
func RedactClaimTokenJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return json.RawMessage([]byte(RedactClaimTokens(string(raw))))
	}
	redacted, changed := redactClaimTokenJSONValue(decoded)
	if !changed {
		return append(json.RawMessage(nil), raw...)
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return nil
	}
	return encoded
}

func redactClaimTokenJSONValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			if strings.EqualFold(strings.TrimSpace(key), "claim_token") {
				changed = true
				continue
			}
			next, nestedChanged := redactClaimTokenJSONValue(nested)
			redacted[key] = next
			changed = changed || nestedChanged
		}
		return redacted, changed
	case []any:
		changed := false
		redacted := make([]any, len(typed))
		for idx, nested := range typed {
			next, nestedChanged := redactClaimTokenJSONValue(nested)
			redacted[idx] = next
			changed = changed || nestedChanged
		}
		return redacted, changed
	case string:
		redacted := RedactClaimTokens(typed)
		return redacted, redacted != typed
	default:
		return value, false
	}
}
