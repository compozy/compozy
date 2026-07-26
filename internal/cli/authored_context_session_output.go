package cli

import (
	"strconv"

	"github.com/compozy/agh/internal/api/contract"
)

func sessionHealthBundle(record SessionHealthRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return sessionHealthSection("Session Health", record), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_health", []string{
				sessionSessionKey,
				workspaceSkillSource,
				agentAgentKey,
				stateKey,
				authoredContextHealthKey,
				"eligible_for_wake",
				memoryReasonKey,
			}, []string{
				record.SessionID,
				record.WorkspaceID,
				record.AgentName,
				string(record.State),
				string(record.Health),
				boolString(record.EligibleForWake),
				string(record.IneligibilityReason),
			}), nil
		},
	}
}

func sessionStatusBundle(record SessionStatusRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			status := renderHumanSection("Session Status", []keyValue{
				{Label: authoredContextSessionValue, Value: stringOrDash(record.SessionID)},
				{Label: authoredContextWorkspaceValue, Value: stringOrDash(record.WorkspaceID)},
				{Label: authoredContextAgentValue, Value: stringOrDash(record.AgentName)},
				{Label: authoredContextStateValue, Value: stringOrDash(string(record.State))},
				{Label: authoredContextHealthValue, Value: stringOrDash(string(record.Health))},
				{Label: "Active Prompt", Value: boolString(record.ActivePrompt)},
				{Label: "Attachable", Value: boolString(record.Attachable)},
				{Label: "Eligible For Wake", Value: boolString(record.EligibleForWake)},
				{Label: "Ineligibility Reason", Value: stringOrDash(string(record.IneligibilityReason))},
				{Label: authoredContextUpdatedValue, Value: stringOrDash(formatTime(record.UpdatedAt))},
			})
			if record.WakeState == nil {
				return status, nil
			}
			return renderHumanBlocks(status, wakeStateSection("Wake State", *record.WakeState)), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_status", []string{
				sessionSessionKey,
				workspaceSkillSource,
				agentAgentKey,
				stateKey,
				authoredContextHealthKey,
				"eligible_for_wake",
				memoryReasonKey,
			}, []string{
				record.SessionID,
				record.WorkspaceID,
				record.AgentName,
				string(record.State),
				string(record.Health),
				boolString(record.EligibleForWake),
				string(record.IneligibilityReason),
			}), nil
		},
	}
}

func sessionInspectBundle(record SessionInspectRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			blocks := []string{
				sessionHealthSection("Session Health", record.Health),
				renderHumanSection("Policy Correlation", []keyValue{
					{Label: "Policy Digest", Value: stringOrDash(record.PolicyDigest)},
					{Label: authoredContextConfigDigestValue, Value: stringOrDash(record.ConfigDigest)},
				}),
				diagnosticsTable(record.Diagnostics),
			}
			if record.WakeState != nil {
				blocks = append(blocks, wakeStateSection("Wake State", *record.WakeState))
			}
			if len(record.WakeEvents) > 0 {
				blocks = append(blocks, wakeEventsTable(record.WakeEvents))
			}
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_inspect", []string{
				sessionSessionKey, "policy_digest", authoredContextConfigDigestKey, "wake_events",
			}, []string{
				record.SessionID,
				record.PolicyDigest,
				record.ConfigDigest,
				strconv.Itoa(len(record.WakeEvents)),
			}), nil
		},
	}
}

func sessionHealthSection(title string, record SessionHealthRecord) string {
	return renderHumanSection(title, []keyValue{
		{Label: authoredContextSessionValue, Value: stringOrDash(record.SessionID)},
		{Label: authoredContextWorkspaceValue, Value: stringOrDash(record.WorkspaceID)},
		{Label: authoredContextAgentValue, Value: stringOrDash(record.AgentName)},
		{Label: authoredContextStateValue, Value: stringOrDash(string(record.State))},
		{Label: authoredContextHealthValue, Value: stringOrDash(string(record.Health))},
		{Label: "Active Prompt", Value: boolString(record.ActivePrompt)},
		{Label: "Attachable", Value: boolString(record.Attachable)},
		{Label: "Eligible For Wake", Value: boolString(record.EligibleForWake)},
		{Label: "Ineligibility Reason", Value: stringOrDash(string(record.IneligibilityReason))},
		{Label: authoredContextLastActivityValue, Value: stringOrDash(formatTimePtr(record.LastActivityAt))},
		{Label: "Last Presence", Value: stringOrDash(formatTimePtr(record.LastPresenceAt))},
		{Label: authoredContextUpdatedValue, Value: stringOrDash(formatTime(record.UpdatedAt))},
	})
}

func wakeStateSection(title string, state contract.HeartbeatWakeStatePayload) string {
	return renderHumanSection(title, []keyValue{
		{Label: authoredContextSessionValue, Value: stringOrDash(state.SessionID)},
		{Label: authoredContextAgentValue, Value: stringOrDash(state.AgentName)},
		{Label: "Policy Snapshot", Value: stringOrDash(state.PolicySnapshotID)},
		{Label: "Last Wake", Value: stringOrDash(formatTimePtr(state.LastWakeAt))},
		{Label: "Next Allowed", Value: stringOrDash(formatTimePtr(state.NextAllowedAt))},
		{Label: "Coalesced", Value: strconv.Itoa(state.CoalescedCount)},
		{Label: "Last Result", Value: stringOrDash(string(state.LastResult))},
		{Label: "Last Reason", Value: stringOrDash(string(state.LastReason))},
		{Label: authoredContextUpdatedValue, Value: stringOrDash(formatTime(state.UpdatedAt))},
	})
}

func diagnosticsTable(items []contract.AuthoredContextDiagnosticPayload) string {
	if len(items) == 0 {
		return ""
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		location := firstNonEmpty(item.Field, item.Section)
		rows = append(rows, []string{
			stringOrDash(string(item.Severity)),
			stringOrDash(item.Code),
			stringOrDash(location),
			stringOrDash(item.Message),
		})
	}
	return renderHumanTable(
		"Diagnostics",
		[]string{cliSeverityValue, cliCodeValue, "Location", authoredContextMessageValue},
		rows,
	)
}

func wakeEventsTable(items []contract.HeartbeatWakeEventPayload) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			stringOrDash(item.ID),
			stringOrDash(item.SessionID),
			stringOrDash(string(item.Source)),
			stringOrDash(string(item.Result)),
			stringOrDash(string(item.Reason)),
			stringOrDash(formatTime(item.CreatedAt)),
		})
	}
	return renderHumanTable(
		"Wake Events",
		[]string{
			"ID",
			authoredContextSessionValue,
			authoredContextSourceValue,
			authoredContextResultValue,
			authoredContextReasonValue,
			authoredContextCreatedValue,
		},
		rows,
	)
}
