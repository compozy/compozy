package gate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

const maxVerdictDiagnosticBytes = 16 * 1024

// SanitizeVerdict returns the canonical verdict representation used by both the gate output
// projection and durable verdict intents.
func SanitizeVerdict(verdict Verdict) (Verdict, error) {
	blocking, err := sanitizeVerdictDiagnostics(verdict.BlockingIssues)
	if err != nil {
		return Verdict{}, err
	}
	criteria, err := sanitizeVerdictDiagnostics(verdict.Criteria)
	if err != nil {
		return Verdict{}, err
	}
	payload, err := sanitizeVerdictDiagnostics(verdict.Payload)
	if err != nil {
		return Verdict{}, err
	}
	warnings, err := sanitizeVerdictDiagnostics(verdict.Warnings)
	if err != nil {
		return Verdict{}, err
	}
	sanitized := verdict
	if err := json.Unmarshal(blocking, &sanitized.BlockingIssues); err != nil {
		return Verdict{}, fmt.Errorf("decode sanitized blocking issues: %w", err)
	}
	if err := json.Unmarshal(criteria, &sanitized.Criteria); err != nil {
		return Verdict{}, fmt.Errorf("decode sanitized criteria: %w", err)
	}
	if err := json.Unmarshal(payload, &sanitized.Payload); err != nil {
		return Verdict{}, fmt.Errorf("decode sanitized verdict payload: %w", err)
	}
	if err := json.Unmarshal(warnings, &sanitized.Warnings); err != nil {
		return Verdict{}, fmt.Errorf("decode sanitized verdict warnings: %w", err)
	}
	return sanitized, nil
}

func sanitizeVerdictDiagnostics(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal verdict diagnostics: %w", err)
	}
	redacted, err := diagnostics.RedactJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("redact verdict diagnostics: %w", err)
	}
	if len(redacted) <= maxVerdictDiagnosticBytes {
		return json.RawMessage(redacted), nil
	}
	var valueTree any
	if err := json.Unmarshal(redacted, &valueTree); err != nil {
		return nil, fmt.Errorf("decode redacted verdict diagnostics: %w", err)
	}
	bounded, err := json.Marshal(boundDiagnosticValue(valueTree, maxVerdictDiagnosticBytes/8))
	if err != nil {
		return nil, fmt.Errorf("marshal bounded verdict diagnostics: %w", err)
	}
	if len(bounded) > maxVerdictDiagnosticBytes {
		return json.RawMessage("[]"), nil
	}
	return json.RawMessage(bounded), nil
}

func boundDiagnosticValue(value any, maxStringBytes int) any {
	switch typed := value.(type) {
	case string:
		return diagnostics.RedactAndBound(typed, maxStringBytes)
	case []any:
		bounded := make([]any, 0, len(typed))
		for _, nested := range typed {
			bounded = append(bounded, boundDiagnosticValue(nested, maxStringBytes))
		}
		return bounded
	case map[string]any:
		bounded := make(map[string]any, len(typed))
		for key, nested := range typed {
			bounded[strings.TrimSpace(key)] = boundDiagnosticValue(nested, maxStringBytes)
		}
		return bounded
	default:
		return value
	}
}
