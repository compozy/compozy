package cli

import (
	"net/url"

	"strconv"
	"strings"

	"time"
)

func sessionEventValues(query SessionEventQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Type); trimmed != "" {
		values.Set("type", trimmed)
	}
	if trimmed := strings.TrimSpace(query.AgentName); trimmed != "" {
		values.Set("agent_name", trimmed)
	}
	if trimmed := strings.TrimSpace(query.TurnID); trimmed != "" {
		values.Set("turn_id", trimmed)
	}
	if !query.Since.IsZero() {
		values.Set("since", query.Since.UTC().Format(time.RFC3339Nano))
	}
	if query.Last > 0 {
		values.Set("limit", strconv.Itoa(query.Last))
	}
	if query.AfterSequence > 0 {
		values.Set("after_sequence", strconv.FormatInt(query.AfterSequence, 10))
	}
	return values
}

func logsListValues(query LogsListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.WorkspaceRef); trimmed != "" {
		values.Set("workspace_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.SessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.AgentName); trimmed != "" {
		values.Set("agent_name", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Type); trimmed != "" {
		values.Set("type", trimmed)
	}
	if trimmed := strings.TrimSpace(query.RunID); trimmed != "" {
		values.Set("run", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ActorKind); trimmed != "" {
		values.Set("actor_kind", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ActorID); trimmed != "" {
		values.Set("actor_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Provider); trimmed != "" {
		values.Set(cliProviderKey, trimmed)
	}
	if trimmed := strings.TrimSpace(query.Outcome); trimmed != "" {
		values.Set(cliOutcomeKey, trimmed)
	}
	if trimmed := strings.TrimSpace(query.Component); trimmed != "" {
		values.Set("component", trimmed)
	}
	if query.ErrorOnly {
		values.Set("error_only", "true")
	}
	if query.AfterSequence > 0 {
		values.Set("after_seq", strconv.FormatInt(query.AfterSequence, 10))
	}
	if !query.Since.IsZero() {
		values.Set("since", query.Since.UTC().Format(time.RFC3339Nano))
	}
	if query.Last > 0 {
		values.Set("limit", strconv.Itoa(query.Last))
	}
	return values
}

func hookCatalogValues(query HookCatalogQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Workspace); trimmed != "" {
		values.Set("workspace", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Agent); trimmed != "" {
		values.Set("agent", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Event); trimmed != "" {
		values.Set("event", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Source); trimmed != "" {
		values.Set("source", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Mode); trimmed != "" {
		values.Set("mode", trimmed)
	}
	return values
}

func hookRunsValues(query HookRunsQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Session); trimmed != "" {
		values.Set("session", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Event); trimmed != "" {
		values.Set("event", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Outcome); trimmed != "" {
		values.Set(cliOutcomeKey, trimmed)
	}
	if trimmed := strings.TrimSpace(query.Since); trimmed != "" {
		values.Set("since", trimmed)
	}
	if query.Last > 0 {
		values.Set("last", strconv.Itoa(query.Last))
	}
	return values
}

func hookEventsValues(query HookEventsQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Family); trimmed != "" {
		values.Set("family", trimmed)
	}
	if query.SyncOnly {
		values.Set("sync_only", strconv.FormatBool(query.SyncOnly))
	}
	return values
}

func memorySelectorValues(query MemorySelectorQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(string(query.Scope)); trimmed != "" {
		values.Set("scope", trimmed)
	}
	if trimmed := strings.TrimSpace(query.WorkspaceID); trimmed != "" {
		values.Set("workspace_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.AgentName); trimmed != "" {
		values.Set("agent_name", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.AgentTier)); trimmed != "" {
		values.Set("agent_tier", trimmed)
	}
	if query.IncludeSystem {
		values.Set("include_system", strconv.FormatBool(query.IncludeSystem))
	}
	return values
}

func memoryHistoryValues(query MemoryHistoryQuery) url.Values {
	values := memorySelectorValues(MemorySelectorQuery{
		Scope:       query.Scope,
		WorkspaceID: query.WorkspaceID,
		AgentName:   query.AgentName,
		AgentTier:   query.AgentTier,
	})
	if trimmed := strings.TrimSpace(query.Operation); trimmed != "" {
		values.Set("operation", trimmed)
	}
	if !query.Since.IsZero() {
		values.Set("since", query.Since.UTC().Format(time.RFC3339Nano))
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func automationRunValues(query AutomationRunQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.JobID); trimmed != "" {
		values.Set("job_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.TriggerID); trimmed != "" {
		values.Set("trigger_id", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.Status)); trimmed != "" {
		values.Set("status", trimmed)
	}
	if !query.Since.IsZero() {
		values.Set("since", query.Since.UTC().Format(time.RFC3339Nano))
	}
	if !query.Until.IsZero() {
		values.Set("until", query.Until.UTC().Format(time.RFC3339Nano))
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}
