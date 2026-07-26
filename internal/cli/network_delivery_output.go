package cli

import "github.com/compozy/agh/internal/api/contract"

func networkWorkBundle(work NetworkWorkRecord) outputBundle {
	return outputBundle{
		jsonValue: contract.NetworkWorkResponse{Work: work},
		human: func() (string, error) {
			return renderHumanSection("Network Work", []keyValue{
				{Label: "Work ID", Value: stringOrDash(work.WorkID)},
				{Label: networkChannelValue, Value: stringOrDash(work.Channel)},
				{Label: networkSurfaceValue, Value: stringOrDash(work.Surface)},
				{Label: networkThreadIDValue, Value: stringOrDash(work.ThreadID)},
				{Label: networkDirectIDValue, Value: stringOrDash(work.DirectID)},
				{Label: "Opened Session", Value: stringOrDash(work.OpenedSessionID)},
				{Label: "Target Session", Value: stringOrDash(work.TargetSessionID)},
				{Label: networkStateValue, Value: stringOrDash(work.State)},
				{Label: networkOpenedAtValue, Value: stringOrDash(formatTimePtr(work.OpenedAt))},
				{Label: networkLastActivityValue, Value: stringOrDash(formatTimePtr(work.LastActivityAt))},
				{Label: "Terminal At", Value: stringOrDash(formatTimePtr(work.TerminalAt))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"network_work",
				[]string{
					networkWorkIDKey,
					networkChannelKey,
					networkSurfaceKey,
					networkThreadIDKey,
					networkDirectIDKey,
					"opened_session_id",
					"target_session_id",
					stateKey,
					networkOpenedAtKey,
					networkLastActivityAtKey,
					"terminal_at",
				},
				[]string{
					work.WorkID,
					work.Channel,
					work.Surface,
					work.ThreadID,
					work.DirectID,
					work.OpenedSessionID,
					work.TargetSessionID,
					work.State,
					formatTimePtr(work.OpenedAt),
					formatTimePtr(work.LastActivityAt),
					formatTimePtr(work.TerminalAt),
				},
			), nil
		},
	}
}

func networkSendBundle(message NetworkSendRecord) outputBundle {
	return outputBundle{
		jsonValue: message,
		human: func() (string, error) {
			return renderHumanSection("Network Message", []keyValue{
				{Label: "ID", Value: stringOrDash(message.ID)},
				{Label: agentKernelSessionValue, Value: stringOrDash(message.SessionID)},
				{Label: networkChannelValue, Value: stringOrDash(message.Channel)},
				{Label: networkSurfaceValue, Value: stringOrDash(message.Surface)},
				{Label: networkThreadIDValue, Value: stringOrDash(message.ThreadID)},
				{Label: networkDirectIDValue, Value: stringOrDash(message.DirectID)},
				{Label: networkKindValue, Value: stringOrDash(message.Kind)},
				{Label: "To", Value: stringOrDash(message.To)},
				{Label: "Work ID", Value: stringOrDash(message.WorkID)},
				{Label: "Reply To", Value: stringOrDash(message.ReplyTo)},
				{Label: "Trace ID", Value: stringOrDash(message.TraceID)},
				{Label: "Causation ID", Value: stringOrDash(message.CausationID)},
				{Label: "Expires At", Value: stringOrDash(formatUnixSeconds(message.ExpiresAt))},
				{Label: "Ext", Value: stringOrDash(networkCompactExt(message.Ext))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("network_message", []string{
				"id",
				"session_id",
				networkChannelKey,
				networkSurfaceKey,
				networkThreadIDKey,
				networkDirectIDKey,
				networkKindKey,
				"to",
				networkWorkIDKey,
				"reply_to",
				"trace_id",
				"causation_id",
				mcpAuthExpiresAtKey,
				"ext",
			}, []string{
				message.ID,
				message.SessionID,
				message.Channel,
				message.Surface,
				message.ThreadID,
				message.DirectID,
				message.Kind,
				message.To,
				message.WorkID,
				message.ReplyTo,
				message.TraceID,
				message.CausationID,
				formatUnixSeconds(message.ExpiresAt),
				networkCompactExt(message.Ext),
			}), nil
		},
	}
}

func networkInboxBundle(messages []NetworkEnvelopeRecord) outputBundle {
	return listBundle(
		messages,
		messages,
		"Network Inbox",
		[]string{
			"ID",
			networkKindValue,
			networkChannelValue,
			networkFromValue,
			"To",
			"Reply To",
			"Trace",
			"Workflow",
			"Handoff",
		},
		"network_inbox",
		[]string{
			"id",
			networkKindKey,
			networkChannelKey,
			"from",
			"to",
			"reply_to",
			"trace_id",
			"causation_id",
			"workflow_id",
			"handoff_version",
			mcpAuthExpiresAtKey,
		},
		func(message NetworkEnvelopeRecord) []string {
			return []string{
				stringOrDash(message.ID),
				stringOrDash(message.Kind),
				stringOrDash(message.Channel),
				stringOrDash(message.From),
				stringOrDash(optionalString(message.To)),
				stringOrDash(optionalString(message.ReplyTo)),
				stringOrDash(optionalString(message.TraceID)),
				stringOrDash(networkExtString(message.Ext, "agh.workflow_id")),
				stringOrDash(networkExtString(message.Ext, "agh.handoff_version")),
			}
		},
		func(message NetworkEnvelopeRecord) []string {
			return []string{
				message.ID,
				message.Kind,
				message.Channel,
				message.From,
				optionalString(message.To),
				optionalString(message.ReplyTo),
				optionalString(message.TraceID),
				optionalString(message.CausationID),
				networkExtString(message.Ext, "agh.workflow_id"),
				networkExtString(message.Ext, "agh.handoff_version"),
				formatUnixSeconds(message.ExpiresAt),
			}
		},
	)
}
