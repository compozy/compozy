package config

import (
	"encoding/json"

	"fmt"
	"math"

	"strconv"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func coerceConfigBool(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, fmt.Errorf("config: parse bool value %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return false, fmt.Errorf("config: expected bool value, got %T", value)
	}
}

func coerceConfigInt(value any) (int, error) {
	int64Value, err := coerceConfigInt64(value)
	if err != nil {
		return 0, err
	}
	converted := int(int64Value)
	if int64(converted) != int64Value {
		return 0, fmt.Errorf("config: integer value %d overflows int", int64Value)
	}
	return converted, nil
}

func coerceConfigInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("config: expected integer value, got %v", typed)
		}
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("config: parse integer value %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("config: expected integer value, got %T", value)
	}
}

func coerceConfigFloat(value any) (float64, error) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), nil
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, fmt.Errorf("config: parse float value %q: %w", typed, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("config: expected float value, got %T", value)
	}
}

func coerceConfigStringSlice(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		values := make([]string, 0, len(typed))
		for idx, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("config: string array item %d has type %T", idx, item)
			}
			values = append(values, text)
		}
		return values, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return []string{}, nil
		}
		if strings.HasPrefix(trimmed, "[") {
			var values []string
			if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
				return nil, fmt.Errorf("config: parse string array %q: %w", typed, err)
			}
			return values, nil
		}
		parts := strings.Split(trimmed, ",")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			if item := strings.TrimSpace(part); item != "" {
				values = append(values, item)
			}
		}
		return values, nil
	default:
		return nil, fmt.Errorf("config: expected string array value, got %T", value)
	}
}

func hookMatcherOverlayValues(matcher hookspkg.HookMatcher) map[string]any {
	values := map[string]any{}
	addString := func(key string, value string) {
		if strings.TrimSpace(value) != "" {
			values[key] = strings.TrimSpace(value)
		}
	}
	addString("agent_name", matcher.AgentName)
	addString("agent_type", matcher.AgentType)
	addString("workspace_id", matcher.WorkspaceID)
	addString("workspace_root", matcher.WorkspaceRoot)
	addString("session_type", matcher.SessionType)
	addString("input_class", matcher.InputClass)
	addString("acp_event_type", matcher.ACPEventType)
	addString("turn_id", matcher.TurnID)
	addString("tool_id", matcher.ToolID)
	addString("tool_name", matcher.ToolName)
	addString("decision_class", matcher.DecisionClass)
	addString("message_role", matcher.MessageRole)
	addString("message_delta_type", matcher.MessageDeltaType)
	if matcher.NetworkMatcher != nil {
		addString("channel", matcher.Channel)
		addString("surface", matcher.Surface)
		addString("kind", matcher.Kind)
		addString("direction", matcher.Direction)
		addString("work_state", matcher.WorkState)
	}
	if matcher.CompactionMatcher != nil {
		addString("compaction_reason", matcher.Reason)
		addString("compaction_strategy", matcher.Strategy)
	}
	if matcher.ToolReadOnly != nil {
		values["tool_read_only"] = *matcher.ToolReadOnly
	}
	return values
}
