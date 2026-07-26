package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
	taskpkg "github.com/compozy/agh/internal/task"
)

func validateEnvelopeContainsNoRawSecrets(env Envelope) error {
	if envelopeRawValueContainsSecret(env.Body) {
		return fmt.Errorf("%w: raw secret material is not allowed in network body", ErrInvalidBody)
	}
	if env.Proof != nil {
		for _, raw := range *env.Proof {
			if envelopeRawValueContainsSecret(raw) {
				return fmt.Errorf("%w: raw secret material is not allowed in network proof", ErrInvalidBody)
			}
		}
	}
	for _, raw := range env.Ext {
		if envelopeRawValueContainsSecret(raw) {
			return fmt.Errorf("%w: raw secret material is not allowed in network ext", ErrInvalidBody)
		}
	}
	return nil
}

func envelopeRawValueContainsSecret(raw json.RawMessage) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return envelopeStringContainsSecret(string(raw))
	}
	return envelopeValueContainsSecret("", value)
}

func envelopeValueContainsSecret(key string, value any) bool {
	if envelopeStringContainsSecret(key) || (envelopeKeyCarriesRawSecret(key) && envelopeValueIsNonEmpty(value)) {
		return true
	}
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return envelopeStringContainsSecret(typed)
	case []any:
		for _, item := range typed {
			if envelopeValueContainsSecret("", item) {
				return true
			}
		}
	case map[string]any:
		for nestedKey, nestedValue := range typed {
			if envelopeValueContainsSecret(nestedKey, nestedValue) {
				return true
			}
		}
	}
	return false
}

func envelopeStringContainsSecret(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	return taskpkg.RedactClaimTokens(value) != value || diagnostics.Redact(value) != value
}

func envelopeValueIsNonEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func envelopeKeyCarriesRawSecret(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	if normalized == "" || strings.Contains(normalized, "hash") {
		return false
	}
	switch normalized {
	case "apikey", "accesstoken", "refreshtoken", "mcpauthtoken", "oauthcode",
		"authorizationcode", "codeverifier", "pkceverifier", "secretbinding",
		"clientsecret", "authorization", "password", "secret", "token", "claimtoken":
		return true
	default:
		return false
	}
}

func rejectLegacyEnvelopeFields(data []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil
	}
	if _, ok := object["interaction_id"]; ok {
		return fmt.Errorf("%w: interaction_id", ErrLegacyFieldRejected)
	}
	return nil
}
