package spec

import "github.com/compozy/compozy/internal/api/contract"

func searchSessionFilesOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specWorkspaceSessionPath + "/files",
		AppAccess:   AppAccessAllowed,
		OperationID: "searchSessionFiles",
		Summary:     "Search files and folders under a session's working directory",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			queryParam("q", "Search text; empty returns the shallowest entries", false),
			intQueryParam("limit", "Maximum entries returned (1-200, default 50)"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionFilesResponse{}},
			{Status: 400, Description: "Invalid limit", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Session working directory no longer exists",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
