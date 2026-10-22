package spec

func catalogAggregateOperations() []OperationSpec {
	terminals := terminalListOperation(
		[]Transport{TransportHTTP, TransportUDS},
		pathParam("workspace_id", "Workspace id"),
	)
	terminals.Path, terminals.OperationID = "/api/terminals", "listTerminalsAggregate"
	stream := terminalCatalogOperation(
		[]Transport{TransportHTTP, TransportUDS},
		pathParam("workspace_id", "Workspace id"),
	)
	stream.Path, stream.OperationID = "/api/terminals/catalog-stream", "streamTerminalsAggregate"
	worktrees := listWorktreesOperationSpec()
	worktrees.Path, worktrees.OperationID = "/api/worktrees", "listWorktreesAggregate"
	suggestions := automationSuggestionListOperationSpec()
	suggestions.Path, suggestions.OperationID = "/api/automation/suggestions", "listAutomationSuggestionsAggregate"
	agents := agentCatalogOperations()[0]
	agents.Path, agents.OperationID = "/api/agents/aggregate", "listAgentsAggregate"
	ops := []OperationSpec{terminals, stream, worktrees, suggestions, agents}
	for i := range ops {
		ops[i].AppAccess = AppAccessDenied
		params := aggregateWorkspaceParameters()
		for _, param := range ops[i].Parameters {
			if param.Name != specWorkspaceIDKey && param.Name != specWorkspaceKey && param.Name != "refresh" {
				params = append(params, param)
			}
		}
		ops[i].Parameters = params
	}
	return ops
}

func aggregateWorkspaceParameters() []ParameterSpec {
	return []ParameterSpec{
		boolQueryParam("all_workspaces", "Read all projects and User rows"),
		workspaceIDsQueryParam("Repeatable project IDs or @user; requires all_workspaces=true"),
	}
}
