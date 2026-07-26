package spec

import "github.com/compozy/agh/internal/api/contract"

const agentNameReservedDescription = "Agent name is reserved"

var agentDefinitionMutationOperationRegistry = []OperationSpec{
	{
		Method:      httpMethodPut,
		Path:        specAPIAgentsNamePath,
		OperationID: "updateAgent",
		Summary:     "Replace one effective AGENT.md definition with digest CAS",
		Tags:        []string{specAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
		},
		RequestBody: contract.UpdateAgentRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.AgentResponse{}},
			{Status: 400, Description: "Invalid agent definition request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Agent definition digest conflict", Body: contract.ErrorPayload{}},
			{Status: 410, Description: specWorkspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: agentNameReservedDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specAgentDefinitionServicesUnavailableDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	},
	{
		Method:      httpMethodDelete,
		Path:        specAPIAgentsNamePath,
		OperationID: "deleteAgent",
		Summary:     "Durably delete one effective authored agent definition",
		Tags:        []string{specAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Agent name"),
			queryParam(specWorkspaceKey, "Workspace id, name, or path used to resolve the effective definition", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.DeleteAgentResponse{}},
			{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 410, Description: specWorkspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specAgentDefinitionServicesUnavailableDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	},
	{
		Method:      httpMethodPost,
		Path:        "/api/agents/{name}/duplicate",
		OperationID: "duplicateAgent",
		Summary:     "Duplicate one effective authored agent definition server-side",
		Tags:        []string{specAgentsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Source agent name"),
		},
		RequestBody: contract.DuplicateAgentRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.AgentResponse{}},
			{Status: 400, Description: "Invalid duplicate agent request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Target agent definition already exists", Body: contract.ErrorPayload{}},
			{Status: 410, Description: specWorkspaceRootMissingDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: agentNameReservedDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specAgentDefinitionServicesUnavailableDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	},
}

func agentDefinitionMutationOperations() []OperationSpec {
	return cloneOperationSpecs(agentDefinitionMutationOperationRegistry)
}

func agentOriginValues() []string {
	return []string{
		string(contract.AgentOriginGlobal),
		string(contract.AgentOriginWorkspace),
	}
}
