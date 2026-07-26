package core

import (
	"encoding/json"

	"strings"

	"github.com/compozy/agh/internal/diagnostics"

	ssepkg "github.com/compozy/agh/internal/sse"
	taskpkg "github.com/compozy/agh/internal/task"
)

func promptRedactRaw(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err == nil {
		redacted, marshalErr := json.Marshal(promptRedactValue(payload))
		if marshalErr == nil {
			return redacted
		}
	}
	return PayloadJSON(promptRedactString(string(raw)))
}

func promptRedactValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return promptRedactString(typed)
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			if promptKeyCarriesSecret(key) {
				redacted[key] = "[REDACTED]"
				continue
			}
			redacted[key] = promptRedactValue(nested)
		}
		return redacted
	case []any:
		redacted := make([]any, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, promptRedactValue(nested))
		}
		return redacted
	default:
		return typed
	}
}

func promptRedactString(value string) string {
	return ssepkg.ScrubMemoryContextString(diagnostics.Redact(taskpkg.RedactClaimTokens(value)))
}

func promptKeyCarriesSecret(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	if normalized == "" {
		return false
	}
	for _, marker := range []string{
		"apikey",
		"accesstoken",
		"refreshtoken",
		"mcpauthtoken",
		"oauthcode",
		"authorizationcode",
		"codeverifier",
		"pkceverifier",
		"secretbinding",
		"authorization",
		"password",
		"secret",
		"token",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func promptStringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func promptMapValue(value any) map[string]any {
	if payload, ok := value.(map[string]any); ok {
		return payload
	}
	return nil
}
