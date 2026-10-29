package spec

import "github.com/compozy/compozy/internal/api/contract"

const workspaceConfigInvalidDescription = "Unknown workspace config keys (workspace_config_invalid)"

func registryWorkspaceOperations() []OperationSpec {
	return []OperationSpec{
		listWorkspacesOperationSpec(),
		workspaceArchiveOperationSpec("archive", "archiveWorkspace"),
		workspaceArchiveOperationSpec("unarchive", "unarchiveWorkspace"),
		createWorkspaceOperationSpec(),
		getWorkspaceOperationSpec(),
		updateWorkspaceOperationSpec(),
		deleteWorkspaceOperationSpec(),
		resolveWorkspaceOperationSpec(),
	}
}
func listWorkspacesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces",
		AppAccess:   AppAccessAllowed,
		OperationID: "listWorkspaces",
		Summary:     "List active or archived workspaces",
		Parameters:  []ParameterSpec{boolQueryParam("archived", "List archived projects instead of active projects")},
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorkspacesResponse{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func createWorkspaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces",
		AppAccess:   AppAccessDenied,
		OperationID: "createWorkspace",
		Summary:     "Register a workspace",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateWorkspaceRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.WorkspaceResponse{}},
			{Status: 400, Description: "Invalid workspace request", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Workspace conflict", Body: contract.ErrorPayload{}},
			{
				Status:      422,
				Description: workspaceConfigInvalidDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getWorkspaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIWorkspacesIDPath,
		AppAccess:   AppAccessAllowed,
		OperationID: "getWorkspace",
		Summary:     "Get one resolved workspace with related data",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Workspace id or path"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorkspaceDetailPayload{}},
			{
				Status:      422,
				Description: workspaceConfigInvalidDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateWorkspaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPIWorkspacesIDPath,
		AppAccess:   AppAccessDenied,
		OperationID: "updateWorkspace",
		Summary:     "Update a registered workspace",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Workspace id"),
		},
		RequestBody: contract.UpdateWorkspaceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorkspaceResponse{}},
			{Status: 400, Description: "Invalid workspace update", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func deleteWorkspaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIWorkspacesIDPath,
		AppAccess:   AppAccessDenied,
		OperationID: "deleteWorkspace",
		Summary:     "Delete a registered workspace",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Workspace id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: specNoContentDescription},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func resolveWorkspaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/resolve",
		AppAccess:   AppAccessDenied,
		OperationID: "resolveWorkspace",
		Summary:     "Resolve or register a workspace from a path",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.ResolveWorkspaceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorkspaceResponse{}},
			{
				Status:      422,
				Description: workspaceConfigInvalidDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 400, Description: "Invalid workspace path", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func workspaceArchiveOperationSpec(action, operationID string) OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: specAPIWorkspacesIDPath + "/" + action,
		AppAccess: AppAccessDenied, OperationID: operationID,
		Summary: action + " a workspace without removing its data",
		Tags:    []string{specWorkspacesKey}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{pathParam("id", "Workspace name or id")},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorkspaceResponse{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Workspace has active sessions, live Loops or enabled automations",
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
