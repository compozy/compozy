package tools

import (
	"fmt"
	"strings"
)

// ToolPattern matches exact ToolIDs or namespace-prefix wildcards.
type ToolPattern struct {
	raw      string
	exact    ToolID
	prefix   string
	wildcard bool
}

// ParseToolPattern validates one exact ToolID or namespace-prefix wildcard.
func ParseToolPattern(raw string) (ToolPattern, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ToolPattern{}, NewValidationError("tool_pattern", ReasonIDEmpty, "tool pattern is required")
	}
	if !strings.Contains(trimmed, "*") {
		id := ToolID(trimmed)
		if err := id.Validate(); err != nil {
			return ToolPattern{}, err
		}
		return ToolPattern{raw: trimmed, exact: id}, nil
	}
	if strings.Count(trimmed, "*") != 1 || !strings.HasSuffix(trimmed, "*") {
		return ToolPattern{}, NewValidationError(
			"tool_pattern",
			ReasonIDInvalidFormat,
			"wildcard must appear once at the end",
		)
	}
	prefix := strings.TrimSuffix(trimmed, "*")
	if prefix == "" || (!strings.HasSuffix(prefix, "_") && !strings.HasSuffix(prefix, "__")) {
		return ToolPattern{}, NewValidationError(
			"tool_pattern",
			ReasonIDInvalidFormat,
			"wildcard must use a canonical namespace prefix",
		)
	}
	if err := ToolID(prefix + "x").Validate(); err != nil {
		return ToolPattern{}, err
	}
	return ToolPattern{raw: trimmed, prefix: prefix, wildcard: true}, nil
}

// ParseToolPatterns validates a list of policy patterns.
func ParseToolPatterns(values []string) ([]ToolPattern, error) {
	patterns := make([]ToolPattern, 0, len(values))
	for i, value := range values {
		pattern, err := ParseToolPattern(value)
		if err != nil {
			return nil, wrapField(err, fmt.Sprintf("tool_patterns[%d]", i))
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

// Match reports whether the pattern covers the given canonical ToolID.
func (p ToolPattern) Match(id ToolID) bool {
	if err := id.Validate(); err != nil {
		return false
	}
	if p.wildcard {
		return strings.HasPrefix(id.String(), p.prefix)
	}
	return p.exact == id
}

// String returns the stable policy expression.
func (p ToolPattern) String() string {
	return p.raw
}

func (p ToolPattern) exactID() (ToolID, bool) {
	if p.wildcard {
		return "", false
	}
	return p.exact, true
}
