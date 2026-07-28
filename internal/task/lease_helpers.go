package task

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const maxClaimTokenMetadataRedactionDepth = 64

func normalizeCapabilityCriteria(values []string) []string {
	return normalizeStringSet(values)
}

func normalizeCreatedTaskIDs(values []string) []string {
	return normalizeStringSet(values)
}

func canonicalTaskIDTokens(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	matches := canonicalTaskIDTokenPattern.FindAllString(string(raw), -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	tokens := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		tokens = append(tokens, match)
	}
	return tokens
}

func normalizeStringSet(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			// Preserve empty entries so Validate rejects invalid input instead of treating it as omitted.
			normalized = append(normalized, trimmed)
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func validateLeaseRunToken(runID string, claimToken string, path string) error {
	if strings.TrimSpace(runID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "run_id"))
	}
	if strings.TrimSpace(claimToken) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "claim_token"))
	}
	return nil
}

func validateLeaseDuration(duration time.Duration, path string) error {
	if duration <= 0 {
		return fmt.Errorf("%w: %s must be positive", ErrValidation, path)
	}
	if duration > MaxRunLeaseDuration {
		return fmt.Errorf("%w: %s must be <= %s", ErrValidation, path, MaxRunLeaseDuration)
	}
	return nil
}

func normalizeLeaseNow(value time.Time, defaultNow time.Time) time.Time {
	if value.IsZero() {
		return defaultNow.UTC()
	}
	return value.UTC()
}

func canonicalClaimTokenHash(value string) string {
	hash := strings.TrimSpace(value)
	hash = strings.TrimPrefix(hash, claimTokenHashPrefix)
	if !isCanonicalClaimTokenHash(hash) {
		return ""
	}
	return hash
}

func sanitizedCoordinationChannelMetadata(
	metadata *CoordinationChannelMetadata,
) *CoordinationChannelMetadata {
	if metadata == nil {
		return nil
	}
	cloned := *metadata
	cloned.ID = strings.TrimSpace(cloned.ID)
	cloned.DisplayName = strings.TrimSpace(cloned.DisplayName)
	cloned.Purpose = strings.TrimSpace(cloned.Purpose)
	cloned.WorkspaceID = strings.TrimSpace(cloned.WorkspaceID)
	cloned.TaskID = strings.TrimSpace(cloned.TaskID)
	cloned.RunID = strings.TrimSpace(cloned.RunID)
	cloned.WorkflowID = strings.TrimSpace(cloned.WorkflowID)
	cloned.AllowedMessageKinds = normalizeStringSet(cloned.AllowedMessageKinds)
	if cloned.DisplayName == "" {
		cloned.DisplayName = cloned.ID
	}
	if cloned.AllowedMessageKinds == nil {
		cloned.AllowedMessageKinds = append([]string(nil), defaultCoordinationMessageKinds...)
	}
	return &cloned
}

func claimResultWithoutRawTokenInMetadata(result *ClaimResult) {
	if result == nil {
		return
	}
	result.CoordinationChannel = sanitizedCoordinationChannelMetadata(result.CoordinationChannel)
	if result.Task != nil {
		result.Task.Metadata = removeRawClaimTokenFields(result.Task.Metadata)
	}
	result.Run.Metadata = removeRawClaimTokenFields(result.Run.Metadata)
	result.Run.Result = removeRawClaimTokenFields(result.Run.Result)
}

func removeRawClaimTokenFields(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !hasRawClaimTokenField(raw) {
		return normalizeRawJSON(raw)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	cleaned, ok := removeRawClaimTokenFieldValue(decoded, 0)
	if !ok {
		return nil
	}
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return nil
	}
	return normalizeRawJSON(encoded)
}

func removeRawClaimTokenFieldValue(value any, depth int) (any, bool) {
	if depth > maxClaimTokenMetadataRedactionDepth {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, nested := range typed {
			if strings.EqualFold(strings.TrimSpace(key), "claim_token") {
				continue
			}
			value, ok := removeRawClaimTokenFieldValue(nested, depth+1)
			if !ok {
				return nil, false
			}
			cleaned[key] = value
		}
		return cleaned, true
	case []any:
		cleaned := make([]any, len(typed))
		for idx, nested := range typed {
			value, ok := removeRawClaimTokenFieldValue(nested, depth+1)
			if !ok {
				return nil, false
			}
			cleaned[idx] = value
		}
		return cleaned, true
	default:
		return value, true
	}
}
