package cli

import "strconv"

func automationTriggerBundle(item TriggerRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection("Automation Trigger", []keyValue{
					{Label: "ID", Value: stringOrDash(item.ID)},
					{Label: automationNameValue, Value: stringOrDash(item.Name)},
					{Label: automationScopeValue, Value: stringOrDash(string(item.Scope))},
					{Label: automationWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
					{Label: automationAgentValue, Value: stringOrDash(item.AgentName)},
					{Label: automationEventValue, Value: stringOrDash(item.Event)},
					{Label: automationEnabledValue, Value: strconv.FormatBool(item.Enabled)},
					{Label: automationSourceValue, Value: stringOrDash(string(item.Source))},
					{Label: "Retry", Value: stringOrDash(formatAutomationRetry(item.Retry))},
					{
						Label: "Fire Limit",
						Value: stringOrDash(formatAutomationFireLimit(item.FireLimit)),
					},
					{Label: "Webhook ID", Value: stringOrDash(item.WebhookID)},
					{Label: "Endpoint Slug", Value: stringOrDash(item.EndpointSlug)},
					{Label: "Webhook Path", Value: stringOrDash(displayTriggerEndpoint(item))},
					{Label: automationCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
					{Label: automationUpdatedValue, Value: stringOrDash(formatTime(item.UpdatedAt))},
				}),
				renderHumanSection(
					"Prompt",
					[]keyValue{{Label: automationBodyValue, Value: stringOrDash(item.Prompt)}},
				),
				renderHumanTable(
					"Filters",
					[]string{automationPathValue, "Value"},
					automationFilterRows(item.Filter),
				),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(
				renderToonObject("automation_trigger", []string{
					"id",
					automationNameKey,
					automationScopeKey,
					automationWorkspaceIDKey,
					automationAgentNameKey,
					automationEventKey,
					automationEnabledKey,
					automationSourceKey,
					automationRetryKey,
					"fire_limit",
					"webhook_id",
					"endpoint_slug",
					bridgeSetupWebhookPathKey,
					automationCreatedAtKey,
					automationUpdatedAtKey,
					automationPromptKey,
				}, []string{
					item.ID,
					item.Name,
					string(item.Scope),
					item.WorkspaceID,
					item.AgentName,
					item.Event,
					strconv.FormatBool(item.Enabled),
					string(item.Source),
					formatAutomationRetry(item.Retry),
					formatAutomationFireLimit(item.FireLimit),
					item.WebhookID,
					item.EndpointSlug,
					displayTriggerEndpoint(item),
					formatTime(item.CreatedAt),
					formatTime(item.UpdatedAt),
					item.Prompt,
				}),
				renderToonArray(
					"filters",
					[]string{automationPathKey, hooksValueKey},
					automationFilterRows(item.Filter),
				),
			), nil
		},
	}
}

func automationTriggerListBundle(page AutomationTriggerListRecord) outputBundle {
	items := page.Triggers
	return listBundle(
		page,
		items,
		"Automation Triggers",
		[]string{
			"ID",
			automationNameValue,
			automationEventValue,
			automationScopeValue,
			automationWorkspaceValue,
			automationAgentValue,
			automationEnabledValue,
			automationSourceValue,
		},
		"automation_triggers",
		[]string{
			"id",
			automationNameKey,
			automationEventKey,
			automationScopeKey,
			automationWorkspaceIDKey,
			automationAgentNameKey,
			automationEnabledKey,
			automationSourceKey,
		},
		func(item TriggerRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.Name),
				stringOrDash(item.Event),
				stringOrDash(string(item.Scope)),
				stringOrDash(item.WorkspaceID),
				stringOrDash(item.AgentName),
				strconv.FormatBool(item.Enabled),
				stringOrDash(string(item.Source)),
			}
		},
		func(item TriggerRecord) []string {
			return []string{
				item.ID,
				item.Name,
				item.Event,
				string(item.Scope),
				item.WorkspaceID,
				item.AgentName,
				strconv.FormatBool(item.Enabled),
				string(item.Source),
			}
		},
	)
}
