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
