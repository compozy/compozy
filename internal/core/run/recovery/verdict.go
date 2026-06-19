package recovery

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// ParseTriageVerdict parses the recovery agent's constrained JSON verdict.
// Malformed output, missing JSON, or unknown decisions fail safe to reject.
func ParseTriageVerdict(output string) TriageVerdict {
	payload, ok := extractVerdictJSON(output)
	if !ok {
		return rejectVerdict("recovery agent output did not contain a JSON verdict")
	}
	var verdict TriageVerdict
	if err := json.Unmarshal([]byte(payload), &verdict); err != nil {
		return rejectVerdict("recovery agent JSON verdict could not be parsed")
	}
	verdict.Decision = VerdictDecision(strings.ToLower(strings.TrimSpace(string(verdict.Decision))))
	verdict.Reason = strings.TrimSpace(verdict.Reason)
	verdict.ChangedFiles = normalizeChangedFiles(verdict.ChangedFiles)
	switch verdict.Decision {
	case VerdictFixed:
		if verdict.Reason == "" {
			verdict.Reason = "recovery agent reported a fix"
		}
		return verdict
	case VerdictReject:
		if verdict.Reason == "" {
			verdict.Reason = "recovery agent rejected remediation"
		}
		return verdict
	default:
		return rejectVerdict("recovery agent JSON verdict used an unknown decision")
	}
}

func extractVerdictJSON(output string) (string, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "{") {
		if payload, ok := firstValidVerdictObject(trimmed); ok {
			return payload, true
		}
	}
	return firstValidVerdictObject(trimmed)
}

func firstValidVerdictObject(text string) (string, bool) {
	for start := strings.IndexByte(text, '{'); start >= 0; {
		if payload, ok := balancedJSONObject(text[start:]); ok && looksLikeVerdict(payload) {
			return payload, true
		}
		nextOffset := start + 1
		next := strings.IndexByte(text[nextOffset:], '{')
		if next < 0 {
			return "", false
		}
		start = nextOffset + next
	}
	return "", false
}

func balancedJSONObject(text string) (string, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size == 0 {
			break
		}
		if inString {
			switch {
			case escaped:
				escaped = false
			case r == '\\':
				escaped = true
			case r == '"':
				inString = false
			}
			i += size
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[:i+size], true
			}
			if depth < 0 {
				return "", false
			}
		}
		i += size
	}
	return "", false
}

func looksLikeVerdict(payload string) bool {
	var probe struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal([]byte(payload), &probe); err != nil {
		return false
	}
	return strings.TrimSpace(probe.Decision) != ""
}

func normalizeChangedFiles(files []string) []string {
	if len(files) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
