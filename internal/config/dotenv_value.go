package config

import (
	"fmt"

	"strings"
	"unicode"
)

func nextNonSpaceIndex(value string, from int) int {
	for idx := from + 1; idx < len(value); idx++ {
		if !unicode.IsSpace(rune(value[idx])) {
			return idx
		}
	}
	return -1
}

func dotEnvAssignmentKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}
	for idx, r := range trimmed {
		if r == '=' {
			key := strings.TrimSpace(trimmed[:idx])
			return key, validDotEnvKey(key)
		}
		if unicode.IsSpace(r) {
			return "", false
		}
	}
	return "", false
}

func validDotEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for idx, r := range key {
		if idx == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func sanitizeDotEnvValue(key string, value string) string {
	if !secretLikeDotEnvKey(key) {
		return value
	}
	var builder strings.Builder
	for _, r := range value {
		if r <= unicode.MaxASCII {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func secretLikeDotEnvKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	return strings.HasSuffix(upper, "_API_KEY") ||
		strings.HasSuffix(upper, "_TOKEN") ||
		strings.HasSuffix(upper, "_SECRET") ||
		strings.HasSuffix(upper, "_KEY")
}

func formatDotEnvAssignment(key string, value string) string {
	if value == "" {
		return key + "="
	}
	if dotEnvValueNeedsQuoting(value) {
		return fmt.Sprintf("%s=%q", key, value)
	}
	return key + "=" + value
}

func dotEnvValueNeedsQuoting(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) || r == '#' || r == '"' || r == '\'' || r == '=' {
			return true
		}
	}
	return false
}

func dotEnvReport(path string, parsed dotEnvParseResult, repaired bool) DotEnvRepairReport {
	status := DotEnvStatusValid
	switch {
	case parsed.unsupported:
		status = DotEnvStatusUnsupported
	case repaired:
		status = DotEnvStatusRepaired
	case parsed.needsRepair:
		status = DotEnvStatusRepairable
	}
	return DotEnvRepairReport{
		Path:        path,
		Status:      status,
		Repaired:    repaired,
		Diagnostics: append([]DotEnvDiagnostic(nil), parsed.diagnostics...),
	}
}
