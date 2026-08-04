package bridges

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// isBlank marks values that carry no identity: whitespace is opaque data, but an all-whitespace value names nothing.
func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func requireField(value string, label string) error {
	if isBlank(value) {
		return fmt.Errorf("bridges: %s is required", label)
	}
	return nil
}

func requireOpaqueIdentity(value string, label string) error {
	if isBlank(value) {
		return fmt.Errorf("bridges: %s is required", label)
	}
	return requireValidUTF8(value, label)
}

func requireOpaqueDeliveryID(value string, label string) error {
	if value == "" {
		return fmt.Errorf("bridges: %s is required", label)
	}
	return requireValidUTF8(value, label)
}

func requireValidUTF8(value string, label string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("bridges: %s must be valid UTF-8", label)
	}
	return nil
}
