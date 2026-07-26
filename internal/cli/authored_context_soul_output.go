package cli

func agentSoulBundle(record AgentSoulRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			summary := renderHumanSection("Agent Soul", []keyValue{
				{Label: authoredContextAgentValue, Value: stringOrDash(record.AgentName)},
				{Label: authoredContextEnabledValue, Value: boolString(record.Enabled)},
				{Label: authoredContextPresentValue, Value: boolString(record.Present)},
				{Label: authoredContextActiveValue, Value: boolString(record.Active)},
				{Label: authoredContextValidValue, Value: boolString(record.Valid)},
				{Label: "Validation", Value: stringOrDash(string(record.ValidationStatus))},
				{Label: authoredContextDigestValue, Value: stringOrDash(record.Digest)},
				{Label: authoredContextSnapshotValue, Value: stringOrDash(record.SnapshotID)},
				{Label: cliRevisionValue, Value: stringOrDash(record.RevisionID)},
				{Label: authoredContextSourceValue, Value: stringOrDash(record.SourcePath)},
			})
			return renderHumanBlocks(summary, diagnosticsTable(record.Diagnostics)), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_soul", []string{
				agentAgentKey,
				automationEnabledKey,
				providerAuthStatePresent,
				authoredContextActiveKey,
				configValidKey,
				"validation_status",
				authoredContextDigestKey,
				"source_path",
			}, []string{
				record.AgentName,
				boolString(record.Enabled),
				boolString(record.Present),
				boolString(record.Active),
				boolString(record.Valid),
				string(record.ValidationStatus),
				record.Digest,
				record.SourcePath,
			}), nil
		},
	}
}

func agentSoulMutationBundle(record *AgentSoulMutationRecord) outputBundle {
	if record == nil {
		empty := AgentSoulMutationRecord{}
		record = &empty
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			revision := renderHumanSection("Soul Revision", []keyValue{
				{Label: "ID", Value: stringOrDash(record.Revision.ID)},
				{Label: authoredContextActionValue, Value: stringOrDash(string(record.Revision.Action))},
				{Label: authoredContextPreviousDigestValue, Value: stringOrDash(record.Revision.PreviousDigest)},
				{Label: authoredContextNewDigestValue, Value: stringOrDash(record.Revision.NewDigest)},
				{Label: authoredContextCreatedValue, Value: stringOrDash(formatTime(record.Revision.CreatedAt))},
			})
			soul, err := agentSoulBundle(record.Soul).human()
			if err != nil {
				return "", err
			}
			return renderHumanBlocks(soul, revision), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_soul_revision", []string{
				"id",
				authoredContextActionKey,
				authoredContextPreviousDigestKey,
				authoredContextNewDigestKey,
				automationCreatedAtKey,
			}, []string{
				record.Revision.ID,
				string(record.Revision.Action),
				record.Revision.PreviousDigest,
				record.Revision.NewDigest,
				formatTime(record.Revision.CreatedAt),
			}), nil
		},
	}
}

func agentSoulHistoryBundle(record AgentSoulHistoryRecord) outputBundle {
	return listBundle(
		record,
		record.Revisions,
		"Soul Revisions",
		[]string{
			"ID",
			authoredContextAgentValue,
			authoredContextActionValue,
			authoredContextPreviousDigestValue,
			authoredContextNewDigestValue,
			authoredContextCreatedValue,
		},
		"agent_soul_revisions",
		[]string{
			"id",
			agentAgentKey,
			authoredContextActionKey,
			authoredContextPreviousDigestKey,
			authoredContextNewDigestKey,
			automationCreatedAtKey,
		},
		func(item AgentSoulRevisionRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.AgentName),
				stringOrDash(string(item.Action)),
				stringOrDash(item.PreviousDigest),
				stringOrDash(item.NewDigest),
				stringOrDash(formatTime(item.CreatedAt)),
			}
		},
		func(item AgentSoulRevisionRecord) []string {
			return []string{
				item.ID,
				item.AgentName,
				string(item.Action),
				item.PreviousDigest,
				item.NewDigest,
				formatTime(item.CreatedAt),
			}
		},
	)
}
