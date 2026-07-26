package cli

import (
	"net/url"
	"strconv"
	"strings"
)

func automationJobValues(query AutomationJobQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(string(query.Scope)); trimmed != "" {
		values.Set("scope", trimmed)
	}
	if trimmed := strings.TrimSpace(query.WorkspaceID); trimmed != "" {
		values.Set("workspace_id", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.Source)); trimmed != "" {
		values.Set("source", trimmed)
	}
	if query.Enabled != nil {
		values.Set("enabled", strconv.FormatBool(*query.Enabled))
	}
	if trimmed := strings.TrimSpace(query.LoopName); trimmed != "" {
		values.Set("loop", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Search); trimmed != "" {
		values.Set("q", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func automationTriggerValues(query AutomationTriggerQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(string(query.Scope)); trimmed != "" {
		values.Set("scope", trimmed)
	}
	if trimmed := strings.TrimSpace(query.WorkspaceID); trimmed != "" {
		values.Set("workspace_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Event); trimmed != "" {
		values.Set("event", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.Source)); trimmed != "" {
		values.Set("source", trimmed)
	}
	if query.Enabled != nil {
		values.Set("enabled", strconv.FormatBool(*query.Enabled))
	}
	if trimmed := strings.TrimSpace(query.LoopName); trimmed != "" {
		values.Set("loop", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Search); trimmed != "" {
		values.Set("q", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}
