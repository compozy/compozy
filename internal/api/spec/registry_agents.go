package spec

import "github.com/compozy/compozy/internal/api/contract"

func registryAgentOperations() []OperationSpec {
	return []OperationSpec{{
		Method: httpMethodPost, Path: "/api/agents/draft-instructions", AppAccess: AppAccessDenied,
		OperationID: "draftAgentInstructions", Summary: "Draft agent instructions without saving a definition",
		Tags: []string{specAgentsKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(), RequestBody: contract.DraftAgentInstructionsRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Draft instructions", Body: contract.DraftAgentInstructionsResponse{}},
			{Status: 400, Description: "Invalid draft request", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Provider does not support read-only triage", Body: contract.ErrorPayload{}},
			{Status: 502, Description: "Instruction drafting failed", Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Instruction drafting unavailable", Body: contract.ErrorPayload{}},
		},
	}, {
		Method:      httpMethodPost,
		Path:        "/api/agents",
		AppAccess:   AppAccessDenied,
		OperationID: "createAgent",
		Summary:     "Create a global or workspace-local AGENT.md definition",
		Tags:        []string{specAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(),
		RequestBody: contract.CreateAgentRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.AgentResponse{}},
			{Status: 400, Description: "Invalid agent definition request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Agent definition already exists", Body: contract.ErrorPayload{}},
			{Status: 410, Description: specWorkspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: agentNameReservedDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Workspace resolver unavailable", Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodGet,
			Path:        "/api/agents",
			AppAccess:   AppAccessAllowed,
			OperationID: "listAgents",
			Summary:     "List all readable agent definitions, optionally resolved for a workspace",
			Tags:        []string{specAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				queryParam(
					specWorkspaceKey,
					"Workspace id, name, or path used to resolve workspace-local agents",
					false,
				),
			),
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.AgentsResponse{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        specAPIAgentsNamePath,
			AppAccess:   AppAccessAllowed,
			OperationID: "getAgent",
			Summary:     "Get one agent definition by name, optionally resolved for a workspace",
			Tags:        []string{specAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: withProfileSelector(
				pathParam("name", "Agent name"),
				queryParam(
					specWorkspaceKey,
					"Workspace id, name, or path used to resolve a workspace-local agent",
					false,
				),
			),
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.AgentResponse{}},
				{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		}}
}
