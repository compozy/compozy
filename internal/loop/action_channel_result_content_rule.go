package loop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	channelResultRuleAny            = "any"
	channelResultRuleJSON           = "json"
	channelResultRuleNonEmpty       = "non_empty"
	channelResultRuleContainsPrefix = "contains:"
	channelResultRuleJSONPathPrefix = "json_path:"
)

func channelResultContentRuleMatches(rule string, payload channelResultPayload) (bool, error) {
	trimmed := strings.TrimSpace(rule)
	if err := validateChannelResultContentRule(trimmed); err != nil {
		return false, err
	}
	switch {
	case trimmed == "", trimmed == channelResultRuleAny:
		return true, nil
	case trimmed == channelResultRuleJSON:
		return channelResultHasJSONObject(payload.structured), nil
	case trimmed == channelResultRuleNonEmpty:
		return len(bytes.TrimSpace(payload.structured)) > 0 || strings.TrimSpace(payload.text) != "", nil
	case strings.HasPrefix(trimmed, channelResultRuleContainsPrefix):
		needle := strings.TrimSpace(strings.TrimPrefix(trimmed, channelResultRuleContainsPrefix))
		return channelResultContains(payload, needle), nil
	case strings.HasPrefix(trimmed, channelResultRuleJSONPathPrefix):
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, channelResultRuleJSONPathPrefix))
		return channelResultJSONPathExists(payload.structured, path)
	default:
		return false, fmt.Errorf("%w: unsupported channel_result content_rule %q", ErrValidation, trimmed)
	}
}

func validateChannelResultContentRule(rule string) error {
	trimmed := strings.TrimSpace(rule)
	switch {
	case trimmed == "",
		trimmed == channelResultRuleAny,
		trimmed == channelResultRuleJSON,
		trimmed == channelResultRuleNonEmpty:
		return nil
	case strings.HasPrefix(trimmed, channelResultRuleContainsPrefix):
		if strings.TrimSpace(strings.TrimPrefix(trimmed, channelResultRuleContainsPrefix)) == "" {
			return fmt.Errorf("%w: channel_result content_rule contains: requires a value", ErrValidation)
		}
		return nil
	case strings.HasPrefix(trimmed, channelResultRuleJSONPathPrefix):
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, channelResultRuleJSONPathPrefix))
		if path == "" {
			return fmt.Errorf("%w: channel_result content_rule json_path: requires a path", ErrValidation)
		}
		for part := range strings.SplitSeq(path, ".") {
			if strings.TrimSpace(part) == "" {
				return fmt.Errorf("%w: channel_result content_rule json_path contains an empty segment", ErrValidation)
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported channel_result content_rule %q", ErrValidation, trimmed)
	}
}

func channelResultHasJSONObject(raw json.RawMessage) bool {
	var value map[string]any
	// JSON content matching is predicate-only; malformed payloads simply do not match.
	return json.Unmarshal(bytes.TrimSpace(raw), &value) == nil
}

func channelResultContains(payload channelResultPayload, needle string) bool {
	haystacks := []string{
		payload.text,
		string(bytes.TrimSpace(payload.structured)),
	}
	for _, haystack := range haystacks {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func channelResultJSONPathExists(raw json.RawMessage, path string) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	var value any
	// JSON-path matching is predicate-only; malformed payloads simply do not match.
	if err := json.Unmarshal(bytes.TrimSpace(raw), &value); err != nil {
		return false, nil
	}
	current := value
	for part := range strings.SplitSeq(path, ".") {
		key := strings.TrimSpace(part)
		if key == "" {
			return false, nil
		}
		object, ok := current.(map[string]any)
		if !ok {
			return false, nil
		}
		next, ok := object[key]
		if !ok {
			return false, nil
		}
		current = next
	}
	return true, nil
}
