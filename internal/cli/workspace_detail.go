package cli

func renderWorkspaceDetailHuman(detail WorkspaceDetailRecord) (string, error) {
	workspaceBlock, err := workspaceRecordBundle(detail.Workspace).human()
	if err != nil {
		return "", err
	}

	return renderHumanBlocks(
		workspaceBlock,
		renderHumanTable(
			"Sessions",
			[]string{
				"ID",
				workspaceNameValue,
				workspaceAgentValue,
				authoredContextStateValue,
				authoredContextWorkspaceValue,
				workspaceUpdatedValue,
			},
			workspaceSessionRows(detail.Sessions, true),
		),
		renderHumanTable(
			"Agents",
			[]string{
				workspaceNameValue,
				agentKernelProviderValue,
				agentKernelModelValue,
				workspaceCategoryValue,
				installPermissionsValue,
			},
			workspaceAgentRows(detail.Agents, true),
		),
		renderHumanTable(
			"Skills",
			[]string{workspaceNameValue, workspaceSourceValue, "Directory"},
			workspaceSkillRows(detail.Skills, true),
		),
	), nil
}

func renderWorkspaceDetailToon(detail WorkspaceDetailRecord) (string, error) {
	workspaceBlock, err := workspaceRecordBundle(detail.Workspace).toon()
	if err != nil {
		return "", err
	}

	return renderHumanBlocks(
		workspaceBlock,
		renderToonArray(
			"sessions",
			[]string{
				"id",
				automationNameKey,
				workspaceAgentNameKey,
				stateKey,
				workspaceSkillSource,
				automationUpdatedAtKey,
			},
			workspaceSessionRows(detail.Sessions, false),
		),
		renderToonArray(
			"agents",
			[]string{
				automationNameKey,
				cliProviderKey,
				workspaceModelKey,
				workspaceCategoryKey,
				configPermissionsKey,
			},
			workspaceAgentRows(detail.Agents, false),
		),
		renderToonArray(
			"skills",
			[]string{automationNameKey, workspaceSourceKey, "dir"},
			workspaceSkillRows(detail.Skills, false),
		),
	), nil
}

func workspaceSessionRows(items []SessionRecord, human bool) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := []string{
			item.ID,
			item.Name,
			item.AgentName,
			string(item.State),
			displaySessionWorkspace(item),
			formatTime(item.UpdatedAt),
		}
		if human {
			row = []string{
				stringOrDash(item.ID),
				stringOrDash(item.Name),
				stringOrDash(item.AgentName),
				stringOrDash(string(item.State)),
				stringOrDash(displaySessionWorkspace(item)),
				stringOrDash(formatTime(item.UpdatedAt)),
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func workspaceAgentRows(items []AgentRecord, human bool) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := []string{
			item.Name,
			item.Provider,
			item.Model,
			agentCategoryLabel(item.CategoryPath),
			item.Permissions,
		}
		if human {
			row = []string{
				stringOrDash(item.Name),
				stringOrDash(item.Provider),
				stringOrDash(item.Model),
				stringOrDash(agentCategoryLabel(item.CategoryPath)),
				stringOrDash(item.Permissions),
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func workspaceSkillRows(items []WorkspaceSkillRecord, human bool) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := []string{item.Name, item.Source, item.Dir}
		if human {
			row = []string{
				stringOrDash(item.Name),
				stringOrDash(item.Source),
				stringOrDash(item.Dir),
			}
		}
		rows = append(rows, row)
	}
	return rows
}
