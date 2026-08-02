package watch

import (
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const maxEventKeyBytes = 256

// NormalizeEventKey validates and canonicalizes one stable watch delivery identity.
func NormalizeEventKey(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", errors.New("loop watch event_key must be valid UTF-8")
	}
	normalized := norm.NFC.String(strings.TrimSpace(value))
	if normalized == "" {
		return "", errors.New("loop watch event_key is required")
	}
	if len([]byte(normalized)) > maxEventKeyBytes {
		return "", errors.New("loop watch event_key must be at most 256 bytes")
	}
	return normalized, nil
}
