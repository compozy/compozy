package cli

func agentMeBundle(record AgentMeRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanBlocks(
				renderHumanSection(agentKernelAgentValue, []keyValue{
					{Label: agentKernelSessionValue, Value: record.Self.SessionID},
					{Label: agentKernelAgentValue, Value: record.Self.AgentName},
					{Label: agentKernelProviderValue, Value: record.Self.Provider},
					{Label: agentKernelModelValue, Value: stringOrDash(record.Self.Model)},
				}),
				renderHumanSection("Workspace", []keyValue{
					{Label: "ID", Value: stringOrDash(record.Workspace.ID)},
					{Label: agentKernelRootValue, Value: stringOrDash(record.Workspace.RootDir)},
				}),
			), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_me", []string{
				automationSessionIDKey,
				agentKernelAgentNameKey,
				cliProviderKey,
				agentKernelModelKey,
				automationWorkspaceIDKey,
				"workspace_root",
			}, []string{
				record.Self.SessionID,
				record.Self.AgentName,
				record.Self.Provider,
				record.Self.Model,
				record.Workspace.ID,
				record.Workspace.RootDir,
			}), nil
		},
	}
}

func agentContextBundle(record *AgentContextRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderJSONPreview(record)
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func agentChannelsBundle(channels []AgentChannelRecord) outputBundle {
	return listBundle(
		channels,
		channels,
		"Agent Channels",
		[]string{"ID", "Purpose", taskTaskValue, taskRunValue},
		"agent_channels",
		[]string{"id", "purpose", taskTaskIDKey, agentKernelRunIDKey},
		func(channel AgentChannelRecord) []string {
			return []string{
				channel.ID,
				stringOrDash(channel.Purpose),
				stringOrDash(channel.TaskID),
				stringOrDash(channel.RunID),
			}
		},
		func(channel AgentChannelRecord) []string {
			return []string{channel.ID, channel.Purpose, channel.TaskID, channel.RunID}
		},
	)
}

func agentChannelMessagesBundle(messages []AgentChannelMessageRecord) outputBundle {
	return listBundle(
		messages,
		messages,
		"Agent Channel Messages",
		[]string{"ID", agentKernelChannelValue, bridgeKindValue, agentKernelFromValue, "Time"},
		"agent_channel_messages",
		[]string{"message_id", "channel_id", agentKernelKindKey, "from_session_id", networkTimestampKey},
		func(message AgentChannelMessageRecord) []string {
			return []string{
				message.MessageID,
				message.ChannelID,
				string(message.Metadata.MessageKind),
				message.FromSessionID,
				formatAgentTime(message.Timestamp),
			}
		},
		func(message AgentChannelMessageRecord) []string {
			return []string{
				message.MessageID,
				message.ChannelID,
				string(message.Metadata.MessageKind),
				message.FromSessionID,
				formatAgentTime(message.Timestamp),
			}
		},
	)
}

func agentChannelMessageBundle(message AgentChannelMessageRecord) outputBundle {
	return outputBundle{
		jsonValue: message,
		human: func() (string, error) {
			return renderHumanSection("Agent Channel Message", []keyValue{
				{Label: "ID", Value: message.MessageID},
				{Label: agentKernelChannelValue, Value: message.ChannelID},
				{Label: bridgeKindValue, Value: string(message.Metadata.MessageKind)},
				{Label: taskTaskValue, Value: stringOrDash(message.Metadata.TaskID)},
				{Label: taskRunValue, Value: stringOrDash(message.Metadata.RunID)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("agent_channel_message", []string{
				"message_id", "channel_id", agentKernelKindKey, taskTaskIDKey, agentKernelRunIDKey,
			}, []string{
				message.MessageID,
				message.ChannelID,
				string(message.Metadata.MessageKind),
				message.Metadata.TaskID,
				message.Metadata.RunID,
			}), nil
		},
	}
}
