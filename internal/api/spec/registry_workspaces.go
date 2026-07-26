package spec

import "github.com/compozy/agh/internal/api/contract"

func registryWorkspaceOperations() []OperationSpec {
	return []OperationSpec{
		listWorkspacesOperationSpec(),
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
		OperationID: "listWorkspaces",
		Summary:     "List registered workspaces",
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
		OperationID: "createWorkspace",
		Summary:     "Register a workspace",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.CreateWorkspaceRequest{},
		Responses: []ResponseSpec{
			{Status: 201, Description: specCreatedDescription, Body: contract.WorkspaceResponse{}},
			{Status: 400, Description: "Invalid workspace request", Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Workspace conflict", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func getWorkspaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIWorkspacesIDPath,
		OperationID: "getWorkspace",
		Summary:     "Get one resolved workspace with related data",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("id", "Workspace id or path"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorkspaceDetailPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
func updateWorkspaceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPIWorkspacesIDPath,
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
		OperationID: "resolveWorkspace",
		Summary:     "Resolve or register a workspace from a path",
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.ResolveWorkspaceRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.WorkspaceResponse{}},
			{Status: 400, Description: "Invalid workspace path", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specWorkspaceNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
