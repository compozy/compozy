package contract

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func requireField(value string, label string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("bridges: %s is required", label)
	}
	return nil
}

// requireOpaqueIdentity validates an identity without rewriting it. Delivery
// identifiers are negotiated opaque tokens: leading or trailing whitespace is
// data, not presentation, and must therefore remain distinguishable.
func requireOpaqueIdentity(value string, label string) error {
	if value == "" {
		return fmt.Errorf("bridges: %s is required", label)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("bridges: %s must be valid UTF-8", label)
	}
	return nil
}

func validateOptionalOpaqueIdentity(value string, label string) error {
	if value != "" && !utf8.ValidString(value) {
		return fmt.Errorf("bridges: %s must be valid UTF-8", label)
	}
	return nil
}
