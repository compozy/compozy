package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ClaimTokensJSON removes raw claim-token fields and values from a JSON payload.
// Invalid input is materialized as a JSON string so wire callers never emit malformed JSON.
func ClaimTokensJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}

	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return redactClaimTokensJSONFallback(raw)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return redactClaimTokensJSONFallback(raw)
	}

	redacted, changed := redactClaimTokensJSONValue(decoded)
	if !changed {
		return append(json.RawMessage(nil), raw...)
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return redactClaimTokensJSONFallback(raw)
	}
	return encoded
}

func redactClaimTokensJSONFallback(raw json.RawMessage) json.RawMessage {
	encoded, err := json.Marshal(ClaimTokens(string(raw)))
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}

func redactClaimTokensJSONValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			if strings.EqualFold(strings.TrimSpace(key), "claim_token") || ClaimTokens(key) != key {
				changed = true
				continue
			}
			next, nestedChanged := redactClaimTokensJSONValue(nested)
			redacted[key] = next
			changed = changed || nestedChanged
		}
		return redacted, changed
	case []any:
		changed := false
		redacted := make([]any, len(typed))
		for index, nested := range typed {
			next, nestedChanged := redactClaimTokensJSONValue(nested)
			redacted[index] = next
			changed = changed || nestedChanged
		}
		return redacted, changed
	case string:
		redacted := ClaimTokens(typed)
		return redacted, redacted != typed
	default:
		return value, false
	}
}
