package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

func rejectTaskSecretText(path string, value string) error {
	if redactTaskSecretText(value) != value {
		return fmt.Errorf("%w: %s must not embed raw secret material", ErrValidation, path)
	}
	return nil
}

func rejectTaskSecretJSON(path string, value []byte) error {
	if len(value) == 0 {
		return nil
	}
	decoded, ok := decodeTaskSecretJSON(value)
	if !ok {
		if redactTaskSecretText(string(value)) != string(value) {
			return fmt.Errorf("%w: %s must not embed raw secret material", ErrValidation, path)
		}
		return nil
	}
	if taskSecretValueContainsSecret(decoded) {
		return fmt.Errorf("%w: %s must not embed raw secret material", ErrValidation, path)
	}
	return nil
}

func redactTaskSecretText(value string) string { return diagnostics.Redact(RedactClaimTokens(value)) }

func redactTaskSecretJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	decoded, ok := decodeTaskSecretJSON(raw)
	if !ok {
		return json.RawMessage(redactTaskSecretText(string(raw)))
	}
	encoded, err := json.Marshal(redactTaskSecretValue(decoded))
	if err != nil {
		return json.RawMessage(redactTaskSecretText(string(raw)))
	}
	return json.RawMessage(encoded)
}

func decodeTaskSecretJSON(raw []byte) (any, bool) {
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, false
	}
	return decoded, true
}

func taskSecretValueContainsSecret(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if taskSecretKeyCarriesSecret(key) || taskSecretValueContainsSecret(nested) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(typed, taskSecretValueContainsSecret)
	case string:
		return redactTaskSecretText(typed) != typed
	}
	return false
}

func redactTaskSecretValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			if taskSecretKeyCarriesSecret(key) {
				redacted[key] = "[REDACTED]"
				continue
			}
			redacted[key] = redactTaskSecretValue(nested)
		}
		return redacted
	case []any:
		redacted := make([]any, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactTaskSecretValue(nested))
		}
		return redacted
	case string:
		return redactTaskSecretText(typed)
	default:
		return typed
	}
}

func taskSecretKeyCarriesSecret(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	probe := trimmed + "=value"
	return diagnostics.Redact(probe) != probe
}
