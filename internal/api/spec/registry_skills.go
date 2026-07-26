package spec

import "github.com/compozy/agh/internal/api/contract"

func registrySkillOperations() []OperationSpec {
	return []OperationSpec{
		listSkillsOperationSpec(),
		installSkillMarketplaceOperationSpec(),
		updateSkillMarketplaceOperationSpec(),
		removeSkillMarketplaceOperationSpec(),
		getSkillOperationSpec(),
		getSkillContentOperationSpec(),
		getSkillShadowsOperationSpec(),
		enableSkillOperationSpec(),
		disableSkillOperationSpec(),
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
func installSkillMarketplaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/skills/marketplace/install",
		OperationID: "installSkillMarketplace",
		Summary:     "Install one remote marketplace skill",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.SkillMarketplaceInstallRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillMarketplaceInstallResponse{}},
			{Status: 400, Description: "Invalid marketplace install request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Marketplace skill not found", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillMarketplaceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateSkillMarketplaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/skills/marketplace/update",
		OperationID: "updateSkillMarketplace",
		Summary:     "Check or apply updates for marketplace skills",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.SkillMarketplaceUpdateRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillMarketplaceUpdateResponse{}},
			{Status: 400, Description: "Invalid marketplace update request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Installed marketplace skill not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Installed skill is not marketplace-managed", Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSkillMarketplaceIsNotConfiguredDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func removeSkillMarketplaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/skills/marketplace/{name}",
		OperationID: "removeSkillMarketplace",
		Summary:     "Remove one installed marketplace skill",
		Tags:        []string{specSkillsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("name", "Installed skill name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SkillMarketplaceRemoveResponse{}},
			{Status: 400, Description: "Invalid marketplace removal request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Installed marketplace skill not found", Body: contract.ErrorPayload{}},
			{Status: 422, Description: "Installed skill is not marketplace-managed", Body: contract.ErrorPayload{}},
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
			queryParam(specWorkspaceKey, "Workspace id or path for resolution context", false),
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
