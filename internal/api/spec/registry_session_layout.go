package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specSessionFoldersPath = "/api/workspaces/{workspace_id}/session-folders"
	specSessionFolderPath  = specSessionFoldersPath + "/{folder_id}"
	specSessionOrderPath   = "/api/workspaces/{workspace_id}/session-order"
	specSessionLayout404   = "Workspace, session, or folder not found"
	specSessionLayout409   = "Layout conflict, folder limit, or subagent session"
)

func registrySessionLayoutOperations() []OperationSpec {
	return []OperationSpec{
		sessionLayoutOperation(httpMethodGet, specSessionFoldersPath, "listSessionFolders",
			"List session folders in manual order", nil, 200, contract.SessionFoldersResponse{}),
		sessionLayoutOperation(httpMethodPost, specSessionFoldersPath, "createSessionFolder",
			"Create a session folder", contract.CreateSessionFolderRequest{}, 201, contract.SessionFolderResponse{}),
		sessionLayoutOperation(httpMethodPatch, specSessionFolderPath, "renameSessionFolder",
			"Rename a session folder", contract.RenameSessionFolderRequest{}, 200, contract.SessionFolderResponse{}),
		sessionLayoutOperation(httpMethodDelete, specSessionFolderPath, "deleteSessionFolder",
			"Delete a session folder; its sessions return to the workspace root", nil, 200,
			contract.SessionFoldersResponse{}),
		sessionLayoutOperation(httpMethodPut, specSessionOrderPath+"/folders", "reorderSessionFolders",
			"Set the complete session folder order", contract.ReorderSessionFoldersRequest{}, 200,
			contract.SessionFoldersResponse{}),
		sessionLayoutOperation(httpMethodPut, specSessionOrderPath+"/sessions", "reorderSessions",
			"Set the manual session order in a folder or the workspace root", contract.ReorderSessionsRequest{}, 200,
			contract.SessionFoldersResponse{}),
		sessionLayoutOperation(httpMethodPost, "/api/workspaces/{workspace_id}/sessions/{session_id}/move",
			"moveSession", "Move a session into a folder or back to the workspace root",
			contract.MoveSessionRequest{}, 200, contract.SessionFoldersResponse{}),
	}
}

func sessionLayoutOperation(
	method string,
	path string,
	operationID string,
	summary string,
	request any,
	status int,
	response any,
) OperationSpec {
	params := []ParameterSpec{pathParam("workspace_id", "Workspace id")}
	switch path {
	case specSessionFolderPath:
		params = append(params, pathParam("folder_id", "Session folder id"))
	case "/api/workspaces/{workspace_id}/sessions/{session_id}/move":
		params = append(params, pathParam("session_id", "Session id"))
	}
	responses := []ResponseSpec{
		{Status: status, Description: "OK", Body: response},
		{Status: 400, Description: "Invalid layout request", Body: contract.ErrorPayload{}},
		{Status: 404, Description: specSessionLayout404, Body: contract.ErrorPayload{}},
		{Status: 409, Description: specSessionLayout409, Body: contract.ErrorPayload{}},
		{Status: 503, Description: "Session layout unavailable", Body: contract.ErrorPayload{}},
		{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
	}
	if method == httpMethodGet {
		responses = append(responses[:1], responses[2:]...)
	}
	return OperationSpec{
		Method:      method,
		Path:        path,
		AppAccess:   AppAccessAllowed,
		OperationID: operationID,
		Summary:     summary,
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  withProfileSelector(params...),
		RequestBody: request,
		Responses:   responses,
	}
}
