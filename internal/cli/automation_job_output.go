package cli

import "strconv"

func automationJobBundle(item JobRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Automation Job", []keyValue{
					{Label: "ID", Value: stringOrDash(item.ID)},
					{Label: automationNameValue, Value: stringOrDash(item.Name)},
					{Label: automationScopeValue, Value: stringOrDash(string(item.Scope))},
					{Label: automationWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
					{Label: automationAgentValue, Value: stringOrDash(item.AgentName)},
					{Label: automationEnabledValue, Value: strconv.FormatBool(item.Enabled)},
					{Label: automationSourceValue, Value: stringOrDash(string(item.Source))},
					{
						Label: automationScheduleValue,
						Value: stringOrDash(formatAutomationSchedule(item.Schedule)),
					},
					{Label: "Retry", Value: stringOrDash(formatAutomationRetry(item.Retry))},
					{
						Label: "Fire Limit",
						Value: stringOrDash(formatAutomationFireLimit(item.FireLimit)),
					},
					{Label: "Next Run", Value: stringOrDash(formatOptionalTime(item.NextRun))},
					{
						Label: "Last Scheduled",
						Value: stringOrDash(formatOptionalTime(automationJobLastScheduledAt(item))),
					},
					{Label: "Last Fire ID", Value: stringOrDash(automationJobLastFireID(item))},
					{Label: "Catch-up Policy", Value: stringOrDash(automationJobCatchUpPolicy(item))},
					{Label: "Misfires", Value: strconv.Itoa(automationJobMisfireCount(item))},
					{Label: automationCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
					{Label: automationUpdatedValue, Value: stringOrDash(formatTime(item.UpdatedAt))},
				}),
				renderHumanSection(
					"Prompt",
					[]keyValue{{Label: automationBodyValue, Value: stringOrDash(item.Prompt)}},
				),
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject("automation_job", []string{
				"id",
				automationNameKey,
				automationScopeKey,
				automationWorkspaceIDKey,
				automationAgentNameKey,
				automationEnabledKey,
				automationSourceKey,
				automationScheduleKey,
				automationRetryKey,
				"fire_limit",
				"next_run",
				"last_scheduled_at",
				"last_fire_id",
				"catch_up_policy",
				"misfire_count",
				automationCreatedAtKey,
				automationUpdatedAtKey,
				automationPromptKey,
			}, []string{
				item.ID,
				item.Name,
				string(item.Scope),
				item.WorkspaceID,
				item.AgentName,
				strconv.FormatBool(item.Enabled),
				string(item.Source),
				formatAutomationSchedule(item.Schedule),
				formatAutomationRetry(item.Retry),
				formatAutomationFireLimit(item.FireLimit),
				formatOptionalTime(item.NextRun),
				formatOptionalTime(automationJobLastScheduledAt(item)),
				automationJobLastFireID(item),
				automationJobCatchUpPolicy(item),
				strconv.Itoa(automationJobMisfireCount(item)),
				formatTime(item.CreatedAt),
				formatTime(item.UpdatedAt),
				item.Prompt,
			}), nil
		},
	}
}

func automationJobListBundle(page AutomationJobListRecord) outputBundle {
	items := page.Jobs
	return listBundle(
		page,
		items,
		"Automation Jobs",
		[]string{
			"ID",
			automationNameValue,
			automationScopeValue,
			automationWorkspaceValue,
			automationScheduleValue,
			automationAgentValue,
			automationEnabledValue,
			automationSourceValue,
			"Next Run",
		},
		"automation_jobs",
		[]string{
			"id",
			automationNameKey,
			automationScopeKey,
			automationWorkspaceIDKey,
			automationScheduleKey,
			automationAgentNameKey,
			automationEnabledKey,
			automationSourceKey,
			"next_run",
		},
		func(item JobRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.Name),
				stringOrDash(string(item.Scope)),
				stringOrDash(item.WorkspaceID),
				stringOrDash(formatAutomationSchedule(item.Schedule)),
				stringOrDash(item.AgentName),
				strconv.FormatBool(item.Enabled),
				stringOrDash(string(item.Source)),
				stringOrDash(formatOptionalTime(item.NextRun)),
			}
		},
		func(item JobRecord) []string {
			return []string{
				item.ID,
				item.Name,
				string(item.Scope),
				item.WorkspaceID,
				formatAutomationSchedule(item.Schedule),
				item.AgentName,
				strconv.FormatBool(item.Enabled),
				string(item.Source),
				formatOptionalTime(item.NextRun),
			}
		},
	)
}
