package cli

func agentHeartbeatBundle(record *AgentHeartbeatRecord) outputBundle {
	if record == nil {
		empty := AgentHeartbeatRecord{}
		record = &empty
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			summary := renderHumanSection("Agent Heartbeat", []keyValue{
				{Label: authoredContextAgentValue, Value: stringOrDash(record.AgentName)},
				{Label: authoredContextEnabledValue, Value: boolString(record.Enabled)},
				{Label: authoredContextPresentValue, Value: boolString(record.Present)},
				{Label: authoredContextActiveValue, Value: boolString(record.Active)},
				{Label: authoredContextValidValue, Value: boolString(record.Valid)},
				{Label: "Validation", Value: stringOrDash(string(record.ValidationStatus))},
				{Label: authoredContextDigestValue, Value: stringOrDash(record.Digest)},
				{Label: authoredContextConfigDigestValue, Value: stringOrDash(record.ConfigDigest)},
				{Label: authoredContextSnapshotValue, Value: stringOrDash(record.SnapshotID)},
				{Label: authoredContextSourceValue, Value: stringOrDash(record.SourcePath)},
				{Label: authoredContextSummaryValue, Value: stringOrDash(record.Summary)},
			})
			return renderHumanBlocks(summary, diagnosticsTable(record.Diagnostics)), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_heartbeat", []string{
				agentAgentKey,
				automationEnabledKey,
				providerAuthStatePresent,
				authoredContextActiveKey,
				configValidKey,
				"validation_status",
				authoredContextDigestKey,
				authoredContextConfigDigestKey,
			}, []string{
				record.AgentName,
				boolString(record.Enabled),
				boolString(record.Present),
				boolString(record.Active),
				boolString(record.Valid),
				string(record.ValidationStatus),
				record.Digest,
				record.ConfigDigest,
			}), nil
		},
	}
}

func agentHeartbeatMutationBundle(record *AgentHeartbeatMutationRecord) outputBundle {
	if record == nil {
		empty := AgentHeartbeatMutationRecord{}
		record = &empty
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			revision := renderHumanSection("Heartbeat Revision", []keyValue{
				{Label: "ID", Value: stringOrDash(record.Revision.ID)},
				{Label: authoredContextOperationValue, Value: stringOrDash(string(record.Revision.Operation))},
				{Label: authoredContextPreviousDigestValue, Value: stringOrDash(record.Revision.PreviousDigest)},
				{Label: authoredContextNewDigestValue, Value: stringOrDash(record.Revision.NewDigest)},
				{Label: authoredContextSnapshotValue, Value: stringOrDash(record.Revision.NewSnapshotID)},
				{Label: authoredContextCreatedValue, Value: stringOrDash(formatTime(record.Revision.CreatedAt))},
			})
			heartbeat, err := agentHeartbeatBundle(&record.Heartbeat).human()
			if err != nil {
				return "", err
			}
			return renderHumanBlocks(heartbeat, revision), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_heartbeat_revision", []string{
				"id",
				authoredContextOperationKey,
				authoredContextPreviousDigestKey,
				authoredContextNewDigestKey,
				cliSnapshotKey,
				automationCreatedAtKey,
			}, []string{
				record.Revision.ID,
				string(record.Revision.Operation),
				record.Revision.PreviousDigest,
				record.Revision.NewDigest,
				record.Revision.NewSnapshotID,
				formatTime(record.Revision.CreatedAt),
			}), nil
		},
	}
}

func agentHeartbeatHistoryBundle(record AgentHeartbeatHistoryRecord) outputBundle {
	return listBundle(
		record,
		record.Revisions,
		"Heartbeat Revisions",
		[]string{
			"ID",
			authoredContextAgentValue,
			authoredContextOperationValue,
			authoredContextPreviousDigestValue,
			authoredContextNewDigestValue,
			authoredContextCreatedValue,
		},
		"agent_heartbeat_revisions",
		[]string{
			"id",
			agentAgentKey,
			authoredContextOperationKey,
			authoredContextPreviousDigestKey,
			authoredContextNewDigestKey,
			automationCreatedAtKey,
		},
		func(item AgentHeartbeatRevisionRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.AgentName),
				stringOrDash(string(item.Operation)),
				stringOrDash(item.PreviousDigest),
				stringOrDash(item.NewDigest),
				stringOrDash(formatTime(item.CreatedAt)),
			}
		},
		func(item AgentHeartbeatRevisionRecord) []string {
			return []string{
				item.ID,
				item.AgentName,
				string(item.Operation),
				item.PreviousDigest,
				item.NewDigest,
				formatTime(item.CreatedAt),
			}
		},
	)
}

func agentHeartbeatStatusBundle(record AgentHeartbeatStatusRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			summary := renderHumanSection("Heartbeat Status", []keyValue{
				{Label: authoredContextAgentValue, Value: stringOrDash(record.AgentName)},
				{Label: authoredContextEnabledValue, Value: boolString(record.Enabled)},
				{Label: authoredContextPresentValue, Value: boolString(record.Present)},
				{Label: authoredContextActiveValue, Value: boolString(record.Active)},
				{Label: authoredContextValidValue, Value: boolString(record.Valid)},
				{Label: authoredContextDigestValue, Value: stringOrDash(record.Digest)},
				{Label: authoredContextConfigDigestValue, Value: stringOrDash(record.ConfigDigest)},
				{Label: authoredContextSummaryValue, Value: stringOrDash(record.Summary)},
			})
			blocks := []string{summary, diagnosticsTable(record.Diagnostics)}
			if record.WakeState != nil {
				blocks = append(blocks, wakeStateSection("Wake State", *record.WakeState))
			}
			if record.SessionHealth != nil {
				blocks = append(blocks, sessionHealthSection("Session Health", *record.SessionHealth))
			}
			if len(record.WakeEvents) > 0 {
				blocks = append(blocks, wakeEventsTable(record.WakeEvents))
			}
			return renderHumanBlocks(blocks...), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_heartbeat_status", []string{
				agentAgentKey,
				automationEnabledKey,
				providerAuthStatePresent,
				authoredContextActiveKey,
				configValidKey,
				authoredContextDigestKey,
				authoredContextConfigDigestKey,
			}, []string{
				record.AgentName,
				boolString(record.Enabled),
				boolString(record.Present),
				boolString(record.Active),
				boolString(record.Valid),
				record.Digest,
				record.ConfigDigest,
			}), nil
		},
	}
}

func agentHeartbeatWakeBundle(record AgentHeartbeatWakeDecisionRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Heartbeat Wake", []keyValue{
				{Label: authoredContextEventValue, Value: stringOrDash(record.WakeEventID)},
				{Label: authoredContextResultValue, Value: stringOrDash(string(record.Result))},
				{Label: authoredContextReasonValue, Value: stringOrDash(string(record.Reason))},
				{Label: "Policy Snapshot", Value: stringOrDash(record.PolicySnapshotID)},
				{Label: "Policy Digest", Value: stringOrDash(record.PolicyDigest)},
				{Label: authoredContextConfigDigestValue, Value: stringOrDash(record.ConfigDigest)},
				{Label: "Synthetic Prompt", Value: stringOrDash(record.SyntheticPromptID)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_heartbeat_wake", []string{
				authoredContextEventKey, "result", memoryReasonKey, "policy_digest", authoredContextConfigDigestKey,
			}, []string{
				record.WakeEventID,
				string(record.Result),
				string(record.Reason),
				record.PolicyDigest,
				record.ConfigDigest,
			}), nil
		},
	}
}
