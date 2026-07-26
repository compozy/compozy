package spec

import "github.com/compozy/agh/internal/api/contract"

func registryAgentOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodPost,
		Path:        "/api/agents",
		OperationID: "createAgent",
		Summary:     "Create a global or workspace-local AGENT.md definition",
		Tags:        []string{specAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
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
			OperationID: "listAgents",
			Summary:     "List all readable agent definitions, optionally resolved for a workspace",
			Tags:        []string{specAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam(
					specWorkspaceKey,
					"Workspace id, name, or path used to resolve workspace-local agents",
					false,
				),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.AgentsResponse{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        specAPIAgentsNamePath,
			OperationID: "getAgent",
			Summary:     "Get one agent definition by name, optionally resolved for a workspace",
			Tags:        []string{specAgentsKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("name", "Agent name"),
				queryParam(
					specWorkspaceKey,
					"Workspace id, name, or path used to resolve a workspace-local agent",
					false,
				),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.AgentResponse{}},
				{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		}}
}
