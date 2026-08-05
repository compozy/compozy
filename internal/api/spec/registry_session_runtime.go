package spec

import "github.com/compozy/compozy/internal/api/contract"

func getSessionCommandsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/commands",
		OperationID: "getSessionCommands",
		Summary:     "List commands available to a session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionCommandsResponse{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Command catalog unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func setSessionRuntimeOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/runtime",
		OperationID: "setSessionRuntime",
		Summary:     "Select the default runtime for future prompts",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.SetSessionRuntimeRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 400, Description: "Invalid runtime selection", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Runtime selection revision conflict", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

func clearSessionRuntimeOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/runtime",
		OperationID: "clearSessionRuntime",
		Summary:     "Clear the default runtime for future prompts",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
			{
				Name:        specExpectedRevisionKey,
				In:          "query",
				Description: "Current runtime selection revision",
				Required:    true,
				Kind:        specIntegerKey,
				Format:      specFormatInt64,
			},
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 400, Description: "Invalid runtime selection revision", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Runtime selection revision conflict", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
