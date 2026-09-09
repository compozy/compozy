// Package reasoning owns the cross-surface reasoning-effort vocabulary.
package reasoning

import (
	"fmt"
	"strings"
	"unicode"
)

// Effort identifies one provider-advertised model reasoning level.
type Effort string

const (
	EffortNone    Effort = "none"
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
	EffortMax     Effort = "max"
	EffortUltra   Effort = "ultra"
)

// InvalidEffortError reports a malformed reasoning effort identifier.
type InvalidEffortError struct {
	Path  string
	Value string
}

func (e *InvalidEffortError) Error() string {
	if e == nil {
		return "invalid reasoning effort"
	}
	return fmt.Sprintf(
		"%s %q is invalid; expected a non-empty identifier without whitespace or control characters",
		e.Path, strings.TrimSpace(e.Value),
	)
}

// Values returns the canonical explicit effort vocabulary in display order.
func Values() []string {
	return []string{
		string(EffortNone),
		string(EffortMinimal),
		string(EffortLow),
		string(EffortMedium),
		string(EffortHigh),
		string(EffortXHigh),
		string(EffortMax),
		string(EffortUltra),
	}
}

// IsValid validates the wire identifier, not a model's capability. Providers may
// introduce new values; the active model's advertised options own membership.
func IsValid(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	})
}
