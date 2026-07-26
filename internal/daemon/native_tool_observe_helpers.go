package daemon

import (
	"fmt"

	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func decodeLogQueryInput(
	req toolspkg.CallRequest,
	scope toolspkg.Scope,
) (logQueryInput, store.EventSummaryQuery, error) {
	var input logQueryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return logQueryInput{}, store.EventSummaryQuery{}, err
	}
	workspaceID, err := nativeCallerWorkspaceInput(req.ToolID, "workspace_id", input.WorkspaceID, scope)
	if err != nil {
		return logQueryInput{}, store.EventSummaryQuery{}, err
	}
	input.WorkspaceID = workspaceID
	query, err := input.eventSummaryQuery(req.ToolID)
	if err != nil {
		return logQueryInput{}, store.EventSummaryQuery{}, err
	}
	return input, query, nil
}

func decodeObserveSearchInput(
	req toolspkg.CallRequest,
	scope toolspkg.Scope,
) (observeSearchInput, store.EventSummaryQuery, error) {
	var input observeSearchInput
	if err := decodeNativeInput(req, &input); err != nil {
		return observeSearchInput{}, store.EventSummaryQuery{}, err
	}
	if _, err := requiredNativeString(req.ToolID, "query", input.Query); err != nil {
		return observeSearchInput{}, store.EventSummaryQuery{}, err
	}
	workspaceID, err := nativeCallerWorkspaceInput(req.ToolID, "workspace_id", input.WorkspaceID, scope)
	if err != nil {
		return observeSearchInput{}, store.EventSummaryQuery{}, err
	}
	input.WorkspaceID = workspaceID
	query, err := input.eventSummaryQuery(req.ToolID)
	if err != nil {
		return observeSearchInput{}, store.EventSummaryQuery{}, err
	}
	return input, query, nil
}

func parseNativeOptionalRFC3339(id toolspkg.ToolID, field string, raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, nil
	}
	timestamp, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			fmt.Sprintf("%s must be an RFC3339 timestamp", field),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return timestamp, nil
}

func logEventPayloads(events []store.EventSummary) []contract.LogEventPayload {
	payload := make([]contract.LogEventPayload, 0, len(events))
	for _, event := range events {
		item := core.LogEventPayloadFromSummary(event)
		item.Summary = taskpkg.RedactClaimTokens(strings.TrimSpace(item.Summary))
		payload = append(payload, item)
	}
	return payload
}

func redactObserveHealthPayload(payload contract.ObserveHealthPayload) contract.ObserveHealthPayload {
	payload.Retention.LastSweepError = taskpkg.RedactClaimTokens(strings.TrimSpace(payload.Retention.LastSweepError))
	for i := range payload.Failures.Recent {
		payload.Failures.Recent[i].Summary = taskpkg.RedactClaimTokens(
			strings.TrimSpace(payload.Failures.Recent[i].Summary),
		)
		payload.Failures.Recent[i].CrashBundlePath = taskpkg.RedactClaimTokens(
			strings.TrimSpace(payload.Failures.Recent[i].CrashBundlePath),
		)
	}
	for i := range payload.AgentProbes {
		payload.AgentProbes[i].Command = taskpkg.RedactClaimTokens(strings.TrimSpace(payload.AgentProbes[i].Command))
		payload.AgentProbes[i].Executable = taskpkg.RedactClaimTokens(
			strings.TrimSpace(payload.AgentProbes[i].Executable),
		)
		payload.AgentProbes[i].Error = taskpkg.RedactClaimTokens(strings.TrimSpace(payload.AgentProbes[i].Error))
	}
	for i := range payload.Activities {
		payload.Activities[i].LastActivityDetail = taskpkg.RedactClaimTokens(
			strings.TrimSpace(payload.Activities[i].LastActivityDetail),
		)
		payload.Activities[i].CurrentTool = taskpkg.RedactClaimTokens(
			strings.TrimSpace(payload.Activities[i].CurrentTool),
		)
		payload.Activities[i].ToolCallID = taskpkg.RedactClaimTokens(
			strings.TrimSpace(payload.Activities[i].ToolCallID),
		)
		payload.Activities[i].StallReason = taskpkg.RedactClaimTokens(
			strings.TrimSpace(payload.Activities[i].StallReason),
		)
	}
	return payload
}

func filterListLogs(
	events []contract.LogEventPayload,
	query string,
) []contract.LogEventPayload {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return events
	}
	filtered := make([]contract.LogEventPayload, 0, len(events))
	for _, event := range events {
		values := []string{event.ID, event.SessionID, event.Type, event.AgentName, event.Summary}
		if slices.ContainsFunc(values, func(value string) bool {
			return strings.Contains(strings.ToLower(value), needle)
		}) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func limitLogPayloads(
	events []contract.LogEventPayload,
	limit int,
) []contract.LogEventPayload {
	if limit <= 0 || limit >= len(events) {
		return events
	}
	return events[:limit]
}

func sessionHistoryPayload(history []store.TurnHistory, info *session.Info) []any {
	payload := make([]any, 0, len(history))
	for _, turn := range history {
		events := make([]any, 0, len(turn.Events))
		for _, event := range turn.Events {
			events = append(events, core.SessionEventPayloadFromEvent(event, info))
		}
		payload = append(payload, map[string]any{
			"turn_id":            turn.TurnID,
			nativeToolsEventsKey: events,
		})
	}
	return payload
}

func actorContextFromScope(scope toolspkg.Scope) (taskpkg.ActorContext, error) {
	if sessionID := strings.TrimSpace(scope.SessionID); sessionID != "" {
		return taskpkg.DeriveAgentSessionActorContext(sessionID, scope.WorkspaceID)
	}
	return taskpkg.DeriveDaemonActorContext("native-tools", "tool.registry")
}

func searchSkills(skillList []*skills.Skill, query string) []*skills.Skill {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return skillList
	}
	filtered := make([]*skills.Skill, 0, len(skillList))
	for _, skill := range skillList {
		if skill == nil {
			continue
		}
		values := []string{
			skill.Meta.Name,
			skill.Meta.Description,
			skill.Meta.Version,
			skills.SkillSourceName(skill.Source),
			skill.InstalledFrom,
		}
		if slices.ContainsFunc(values, func(value string) bool {
			return strings.Contains(strings.ToLower(value), needle)
		}) {
			filtered = append(filtered, skill)
		}
	}
	return filtered
}

func limitSkills(skillList []*skills.Skill, limit int) []*skills.Skill {
	if limit <= 0 || limit >= len(skillList) {
		return skillList
	}
	return skillList[:limit]
}

func limitToolViews(views []toolspkg.ToolView, limit int) []toolspkg.ToolView {
	if limit <= 0 || limit >= len(views) {
		return views
	}
	return views[:limit]
}
