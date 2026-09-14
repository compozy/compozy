package spec

import "github.com/compozy/compozy/internal/api/contract"

func registrySkillOperations() []OperationSpec {
	return []OperationSpec{
		listSkillsOperationSpec(),
		getSkillOperationSpec(),
		getSkillContentOperationSpec(),
		getSkillShadowsOperationSpec(),
		exposeSkillOperationSpec(),
		unexposeSkillOperationSpec(),
		enableSkillOperationSpec(),
		disableSkillOperationSpec(),
	}
}

func exposeSkillOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/skills/{name}/expose",
		OperationID: "exposeSkill",
		Summary:     "Expose one user- or workspace-owned skill to provider roots",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Skill name"),
			queryParam("profile", "Exact acting profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		RequestBody: contract.SkillExposureRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillExposeResponse{}},
			{Status: 409, Description: "Exposure failed", Body: contract.SkillExposureFailureResponse{}},
		},
	}
}

func unexposeSkillOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/skills/{name}/unexpose",
		OperationID: "unexposeSkill",
		Summary:     "Remove owned provider-root links for one skill",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Skill name"),
			queryParam("profile", "Exact acting profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		RequestBody: contract.SkillExposureRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillUnexposeResponse{}},
			{Status: 409, Description: "Removal failed", Body: contract.SkillExposureFailureResponse{}},
		},
	}
}
func listSkillsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/skills",
		OperationID: "listSkills",
		Summary:     "List effective skills for the selected global, workspace, or agent scope",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam(specWorkspaceKey, "Workspace id or path for resolution context", false),
			queryParam("profile", "Exact profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillsResponse{}},
			{Status: 400, Description: specInvalidSkillLookupDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Skill scope not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillsRegistryIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSkillOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/skills/{name}",
		OperationID: "getSkill",
		Summary:     "Get one skill definition",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Skill name"),
			queryParam("workspace_id", "Canonical workspace id for resolution context", false),
			queryParam("profile", "Exact profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillResponse{}},
			{Status: 400, Description: specInvalidSkillLookupDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSkillOrScopeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillsRegistryIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSkillContentOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/skills/{name}/content",
		OperationID: "getSkillContent",
		Summary:     "Get the raw content for one skill",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Skill name"),
			queryParam(specWorkspaceKey, "Workspace id or path for resolution context", false),
			queryParam("profile", "Exact profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillContentResponse{}},
			{Status: 400, Description: specInvalidSkillLookupDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSkillOrScopeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillsRegistryIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getSkillShadowsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/skills/{name}/shadows",
		OperationID: "getSkillShadows",
		Summary:     "Get resolver provenance and shadow declarations for one skill",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Skill name"),
			queryParam(specWorkspaceKey, "Workspace id or path for resolution context", false),
			queryParam("profile", "Exact profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillShadowsResponse{}},
			{Status: 400, Description: specInvalidSkillLookupDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSkillOrScopeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillsRegistryIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func enableSkillOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/skills/{name}/enable",
		OperationID: "enableSkill",
		Summary:     "Enable one skill",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Skill name"),
			queryParam(specWorkspaceKey, "Workspace id or path for resolution context", false),
			queryParam("profile", "Exact profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillActionResponse{}},
			{Status: 400, Description: specInvalidSkillLookupDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSkillOrScopeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillsRegistryIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func disableSkillOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/skills/{name}/disable",
		OperationID: "disableSkill",
		Summary:     "Disable one skill",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Skill name"),
			queryParam(specWorkspaceKey, "Workspace id or path for resolution context", false),
			queryParam("profile", "Exact profile name", false),
			queryParam("for_agent", "Logical agent name for agent-local resolution", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillActionResponse{}},
			{Status: 400, Description: specInvalidSkillLookupDescription, Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSkillOrScopeNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 422, Description: specInvalidAgentLocalLayerDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillsRegistryIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
