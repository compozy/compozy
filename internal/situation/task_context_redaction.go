package situation

import (
	"encoding/json"

	"fmt"
	"slices"
	"strings"

	"unicode/utf8"

	taskpkg "github.com/compozy/agh/internal/task"
)

func selectTaskContextRun(taskRecord taskpkg.Task, runs []taskpkg.Run) (taskpkg.Run, bool) {
	currentRunID := strings.TrimSpace(taskRecord.CurrentRunID)
	if currentRunID != "" {
		for _, run := range runs {
			if strings.TrimSpace(run.ID) == currentRunID {
				return run, true
			}
		}
	}
	if run, ok := selectActiveRun(runs); ok {
		return run, true
	}
	if len(runs) == 0 {
		return taskpkg.Run{}, false
	}
	sorted := append([]taskpkg.Run(nil), runs...)
	sortRunsByAttemptAndActivity(sorted)
	return sorted[0], true
}

func sortRunsByAttemptAndActivity(runs []taskpkg.Run) {
	slices.SortStableFunc(runs, func(left, right taskpkg.Run) int {
		if left.Attempt != right.Attempt {
			return int(right.Attempt - left.Attempt)
		}
		leftTime := runActivityTime(left)
		rightTime := runActivityTime(right)
		if !leftTime.Equal(rightTime) {
			if leftTime.After(rightTime) {
				return -1
			}
			return 1
		}
		return strings.Compare(left.ID, right.ID)
	})
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func redactTaskContextPayload(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("situation: decode task context payload: %w", err)
	}
	cleaned := redactTaskContextJSONValue(decoded)
	content, err := json.Marshal(cleaned)
	if err != nil {
		return nil, fmt.Errorf("situation: encode task context payload: %w", err)
	}
	return json.RawMessage(content), nil
}

func redactTaskContextJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, nested := range typed {
			if isSensitiveTaskContextKey(key) {
				continue
			}
			cleaned[key] = redactTaskContextJSONValue(nested)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(typed))
		for idx, nested := range typed {
			cleaned[idx] = redactTaskContextJSONValue(nested)
		}
		return cleaned
	case string:
		return taskpkg.RedactClaimTokens(typed)
	default:
		return value
	}
}

func isSensitiveTaskContextKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "claim_token", "mcp_auth_token", "oauth_code", "pkce_verifier", "secret_binding", "secret_bindings":
		return true
	}
	return strings.HasPrefix(normalized, "agh_claim_") || strings.HasSuffix(normalized, "_secret")
}

func safeTaskContextText(value string, limit int) string {
	return truncateUTF8Bytes(taskpkg.RedactClaimTokens(singleLine(value)), limit)
}

func truncateUTF8Bytes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	if limit <= len("...") {
		return strings.Repeat(".", limit)
	}
	cut := limit - len("...")
	for cut > 0 && !utf8.ValidString(value[:cut]) {
		cut--
	}
	return strings.TrimSpace(value[:cut]) + "..."
}
