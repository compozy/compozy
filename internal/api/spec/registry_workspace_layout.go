package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specWorkspaceFoldersPath = "/api/workspace-folders"
	specWorkspaceFolderPath  = specWorkspaceFoldersPath + "/{folder_id}"
	specWorkspaceOrderPath   = "/api/workspace-order"
	specWorkspaceLayout404   = "Workspace or workspace folder not found"
	specWorkspaceLayout409   = "Layout conflict or folder limit"
)

func registryWorkspaceLayoutOperations() []OperationSpec {
	return []OperationSpec{
		workspaceLayoutOperation(httpMethodGet, specWorkspaceFoldersPath, "getWorkspaceLayout",
			"Read the profile's workspace folders and manual workspace order", nil, 200,
			contract.WorkspaceLayoutResponse{}),
		workspaceLayoutOperation(httpMethodPost, specWorkspaceFoldersPath, "createWorkspaceFolder",
			"Create a root workspace folder", contract.CreateWorkspaceFolderRequest{}, 201,
			contract.WorkspaceFolderResponse{}),
		workspaceLayoutOperation(httpMethodPatch, specWorkspaceFolderPath, "renameWorkspaceFolder",
			"Rename a workspace folder", contract.RenameWorkspaceFolderRequest{}, 200,
			contract.WorkspaceFolderResponse{}),
		workspaceLayoutOperation(httpMethodDelete, specWorkspaceFolderPath, "deleteWorkspaceFolder",
			"Delete a workspace folder; its workspaces return to the root", nil, 200,
			contract.WorkspaceLayoutResponse{}),
		workspaceLayoutOperation(httpMethodPut, specWorkspaceOrderPath+"/folders", "reorderWorkspaceFolders",
			"Set the complete workspace folder order", contract.ReorderWorkspaceFoldersRequest{}, 200,
			contract.WorkspaceLayoutResponse{}),
		workspaceLayoutOperation(httpMethodPut, specWorkspaceOrderPath+"/workspaces", "reorderWorkspaces",
			"Set the manual workspace order in a folder or the root", contract.ReorderWorkspacesRequest{}, 200,
			contract.WorkspaceLayoutResponse{}),
		workspaceLayoutOperation(httpMethodPost, specWorkspaceOrderPath+"/move", "moveWorkspace",
			"Move a workspace into a folder or back to the root", contract.MoveWorkspaceRequest{}, 200,
			contract.WorkspaceLayoutResponse{}),
	}
}

func workspaceLayoutOperation(
	method string,
	path string,
	operationID string,
	summary string,
	request any,
	status int,
	response any,
) OperationSpec {
	var params []ParameterSpec
	if path == specWorkspaceFolderPath {
		params = append(params, pathParam("folder_id", "Workspace folder id"))
	}
	responses := []ResponseSpec{
		{Status: status, Description: "OK", Body: response},
		{Status: 400, Description: "Invalid layout request", Body: contract.ErrorPayload{}},
		{Status: 403, Description: "Operator or unrestricted agent required", Body: contract.ErrorPayload{}},
		{Status: 404, Description: specWorkspaceLayout404, Body: contract.ErrorPayload{}},
		{Status: 409, Description: specWorkspaceLayout409, Body: contract.ErrorPayload{}},
		{Status: 503, Description: "Workspace layout unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
	if method == httpMethodGet {
		responses = append(responses[:4], responses[5:]...)
	}
	return OperationSpec{
		Method:      method,
		Path:        path,
		AppAccess:   AppAccessDenied,
		OperationID: operationID,
		Summary:     summary,
		Tags:        []string{specWorkspacesKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(params...),
		RequestBody: request,
		Responses:   responses,
	}
}
