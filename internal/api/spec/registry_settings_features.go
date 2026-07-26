package spec

import "github.com/compozy/agh/internal/api/contract"

func registrySettingsFeatureOperations() []OperationSpec {
	return []OperationSpec{
		getSettingsMemoryOperationSpec(),
		updateSettingsMemoryOperationSpec(),
		getSettingsRolesOperationSpec(),
		updateSettingsRolesOperationSpec(),
		getSettingsNetworkOperationSpec(),
		updateSettingsNetworkOperationSpec(),
		getSettingsWindowManagerOperationSpec(),
		updateSettingsWindowManagerOperationSpec(),
		getSettingsObservabilityOperationSpec(),
		updateSettingsObservabilityOperationSpec(),
		streamSettingsObservabilityLogTailOperationSpec(),
		listSettingsProvidersOperationSpec(),
		getSettingsProviderOperationSpec(),
		putSettingsProviderOperationSpec(),
		deleteSettingsProviderOperationSpec(),
		getSettingsSkillsOperationSpec(),
		updateSettingsSkillsOperationSpec(),
	}
}

func getSettingsRolesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsRolesPath,
		OperationID: "getSettingsRoles",
		Summary:     "Read the background-role routing settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsRolesResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func updateSettingsRolesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsRolesPath,
		OperationID: "updateSettingsRoles",
		Summary:     "Update the background-role routing settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsRolesRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsMemoryPath,
		OperationID: "getSettingsMemory",
		Summary:     "Read the memory settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsMemoryResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSettingsMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsMemoryPath,
		OperationID: "updateSettingsMemory",
		Summary:     "Update the memory settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsMemoryRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsNetworkOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsNetworkPath,
		OperationID: "getSettingsNetwork",
		Summary:     "Read the network settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsNetworkResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSettingsNetworkOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsNetworkPath,
		OperationID: "updateSettingsNetwork",
		Summary:     "Update the network settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsNetworkRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsObservabilityOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsObservabilityPath,
		OperationID: "getSettingsObservability",
		Summary:     "Read the observability settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsObservabilityResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSettingsObservabilityOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsObservabilityPath,
		OperationID: "updateSettingsObservability",
		Summary:     "Update the observability settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.UpdateSettingsObservabilityRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func streamSettingsObservabilityLogTailOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/observability/log-tail",
		OperationID: "streamSettingsObservabilityLogTail",
		Summary:     "Stream daemon log output for the observability settings screen",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "SSE stream established"},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func listSettingsProvidersOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/settings/providers",
		OperationID: "listSettingsProviders",
		Summary:     "List settings-backed providers",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsProvidersResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsProviderOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsProvidersNamePath,
		OperationID: "getSettingsProvider",
		Summary:     "Read one settings-backed provider",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Provider name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsProviderResponse{}},
			{Status: 404, Description: specProviderNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func putSettingsProviderOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specAPISettingsProvidersNamePath,
		OperationID: "putSettingsProvider",
		Summary:     "Create or replace one settings-backed provider overlay",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Provider name"),
		},
		RequestBody: contract.PutSettingsProviderRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: "Invalid provider payload", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Conflicting provider change", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteSettingsProviderOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPISettingsProvidersNamePath,
		OperationID: "deleteSettingsProvider",
		Summary:     "Delete one settings-backed provider overlay",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Provider name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specProviderNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSettingsSkillsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPISettingsSkillsPath,
		OperationID: "getSettingsSkills",
		Summary:     "Read the skills settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Select the settings scope", settingsAgentScopeValues()),
			queryParam("workspace_id", "Optional workspace id for agent resolution context", false),
			queryParam("agent_name", "Agent name when scope=agent", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsSkillsResponse{}},
			{Status: 400, Description: "Invalid settings scope", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSettingsSkillsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPISettingsSkillsPath,
		OperationID: "updateSettingsSkills",
		Summary:     "Update the skills settings section",
		Tags:        []string{specSettingsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			enumQueryParam(specScopeKey, "Select the settings scope", settingsAgentScopeValues()),
			queryParam("workspace_id", "Optional workspace id for agent resolution context", false),
			queryParam("agent_name", "Agent name when scope=agent", false),
		},
		RequestBody: contract.UpdateSettingsSkillsRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SettingsApplyResponse{}},
			{Status: 400, Description: specInvalidSettingsPayloadDescription, Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specAgentNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specConflictingSettingsChangeDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
