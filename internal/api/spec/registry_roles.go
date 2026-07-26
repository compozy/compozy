package spec

import "github.com/compozy/agh/internal/api/contract"

func registryRolesOperations() []OperationSpec {
	workspace := queryParam(
		specWorkspaceKey,
		"Workspace id, name, or path used to resolve effective role configuration",
		false,
	)
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        "/api/roles",
			OperationID: "listRoles",
			Summary:     "List effective background role configuration",
			Tags:        []string{specRolesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{workspace},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.RolesResponse{}},
				{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
				{Status: 410, Description: specWorkspaceRootMissingDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Roles status provider unavailable", Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodGet,
			Path:        "/api/roles/{role}",
			OperationID: "getRole",
			Summary:     "Get effective background role configuration",
			Tags:        []string{specRolesKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				pathParam("role", "Background role name"),
				workspace,
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.RoleStatusResponse{}},
				{Status: 404, Description: "Role or workspace not found", Body: contract.ErrorPayload{}},
				{Status: 410, Description: specWorkspaceRootMissingDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: "Roles status provider unavailable", Body: contract.ErrorPayload{}},
			},
		},
	}
}
