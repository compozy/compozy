package cli

import (
	"strconv"

	"github.com/compozy/agh/internal/api/contract"
)

func automationSuggestionBundle(item SuggestionRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Automation Suggestion", []keyValue{
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: automationNameValue, Value: stringOrDash(item.Payload.Name)},
				{Label: automationWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
				{Label: automationStatusValue, Value: stringOrDash(string(item.Status))},
				{Label: automationSourceValue, Value: stringOrDash(string(item.Source))},
				{Label: automationScheduleValue, Value: stringOrDash(formatAutomationSchedule(item.Payload.Schedule))},
				{Label: automationAgentValue, Value: stringOrDash(item.Payload.AgentName)},
				{Label: automationCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
				{Label: "Resolved", Value: stringOrDash(formatOptionalTime(item.ResolvedAt))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("automation_suggestion", []string{
				"id", automationNameKey, automationWorkspaceIDKey, automationStatusKey,
				automationSourceKey, automationScheduleKey, automationAgentNameKey,
				automationCreatedAtKey, "resolved_at",
			}, []string{
				item.ID, item.Payload.Name, item.WorkspaceID, string(item.Status), string(item.Source),
				formatAutomationSchedule(item.Payload.Schedule), item.Payload.AgentName,
				formatTime(item.CreatedAt), formatOptionalTime(item.ResolvedAt),
			}), nil
		},
	}
}

func automationSuggestionListBundle(response AutomationSuggestionListRecord) outputBundle {
	return listBundle(
		response,
		response.Suggestions,
		"Automation Suggestions",
		[]string{"ID", automationNameValue, automationStatusValue, automationScheduleValue, automationAgentValue},
		"automation_suggestions",
		[]string{"id", automationNameKey, automationStatusKey, automationScheduleKey, automationAgentNameKey},
		func(item contract.AutomationSuggestionPayload) []string {
			return []string{
				stringOrDash(item.ID), stringOrDash(item.Payload.Name), stringOrDash(string(item.Status)),
				stringOrDash(formatAutomationSchedule(item.Payload.Schedule)), stringOrDash(item.Payload.AgentName),
			}
		},
		func(item contract.AutomationSuggestionPayload) []string {
			return []string{
				item.ID, item.Payload.Name, string(item.Status),
				formatAutomationSchedule(item.Payload.Schedule), item.Payload.AgentName,
			}
		},
	)
}

func automationSuggestionAcceptanceBundle(response *AutomationSuggestionAcceptanceRecord) outputBundle {
	return outputBundle{
		jsonValue: response,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Accepted Automation Suggestion", []keyValue{
					{Label: "ID", Value: response.Suggestion.ID},
					{Label: automationNameValue, Value: response.Suggestion.Payload.Name},
					{Label: automationStatusValue, Value: string(response.Suggestion.Status)},
				}),
				renderHumanSection("Created Job", []keyValue{
					{Label: "ID", Value: response.Job.ID},
					{Label: automationNameValue, Value: response.Job.Name},
					{Label: automationEnabledValue, Value: strconv.FormatBool(response.Job.Enabled)},
				}),
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject("automation_suggestion_acceptance", []string{
				"suggestion_id", "suggestion_status", "job_id", "job_name", "job_enabled",
			}, []string{
				response.Suggestion.ID, string(response.Suggestion.Status), response.Job.ID,
				response.Job.Name, strconv.FormatBool(response.Job.Enabled),
			}), nil
		},
	}
}
