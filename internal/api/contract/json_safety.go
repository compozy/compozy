package contract

import (
	"encoding/json"
	"strings"
	"unicode"
)

type jsonSafetyKeyPredicate func(string) bool
type jsonSafetyStringPredicate func(string) bool

func containsUnsafeJSON(
	data []byte,
	keyMatches jsonSafetyKeyPredicate,
	stringMatches jsonSafetyStringPredicate,
) bool {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false
	}
	return containsUnsafeJSONValue(value, keyMatches, stringMatches)
}

func containsUnsafeJSONValue(
	value any,
	keyMatches jsonSafetyKeyPredicate,
	stringMatches jsonSafetyStringPredicate,
) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if keyMatches != nil && keyMatches(key) {
				return true
			}
			if containsUnsafeJSONValue(child, keyMatches, stringMatches) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsUnsafeJSONValue(child, keyMatches, stringMatches) {
				return true
			}
		}
	case string:
		return stringMatches != nil && stringMatches(typed)
	}
	return false
}

func containsUnsafePublicContractJSON(data []byte) bool {
	return containsUnsafeJSON(data, isUnsafePublicContractKey, isUnsafePublicContractString)
}

func isUnsafePublicContractKey(key string) bool {
	normalized := canonicalizeUnsafePublicContractKey(key)
	switch normalized {
	case "claim_token",
		"provider_token",
		"access_token",
		"refresh_token",
		"id_token",
		"authorization_code",
		"oauth_code",
		"code_verifier",
		"pkce_verifier",
		"mcp_auth_token",
		"api_key",
		"secret",
		"secret_ref",
		"client_secret_ref",
		"webhook_secret_ref",
		"secret_binding",
		"token",
		"client_secret",
		"provider_credentials",
		"raw_prompt",
		"prompt_body",
		"transcript":
		return true
	default:
		return false
	}
}

func canonicalizeUnsafePublicContractKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	lastUnderscore := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
			lastUnderscore = false
		case !lastUnderscore:
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func isUnsafePublicContractString(value string) bool {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "agh_claim_") ||
		strings.Contains(lower, "env:") ||
		strings.Contains(lower, "vault:") ||
		strings.HasPrefix(trimmed, "sk-") ||
		strings.HasPrefix(trimmed, "github_pat_") ||
		strings.HasPrefix(trimmed, "xoxb-") ||
		strings.HasPrefix(trimmed, "xoxp-")
}
