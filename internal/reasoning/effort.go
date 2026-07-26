// Package reasoning owns the cross-surface reasoning-effort vocabulary.
package reasoning

import (
	"fmt"
	"strings"
)

// Effort identifies one canonical model reasoning level.
type Effort string

const (
	EffortNone    Effort = "none"
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
	EffortMax     Effort = "max"
)

// InvalidEffortError reports a value outside the canonical effort vocabulary.
type InvalidEffortError struct {
	Path  string
	Value string
}

func (e *InvalidEffortError) Error() string {
	if e == nil {
		return "invalid reasoning effort"
	}
	return fmt.Sprintf(
		"%s %q is invalid; expected %s",
		e.Path,
		strings.TrimSpace(e.Value),
		strings.Join(Values(), ", "),
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
	}
}

// IsValid reports whether value is one canonical explicit effort.
func IsValid(value string) bool {
	switch Effort(strings.TrimSpace(value)) {
	case EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax:
		return true
	default:
		return false
	}
}
