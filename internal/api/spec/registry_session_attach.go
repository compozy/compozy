package spec

import "github.com/compozy/compozy/internal/api/contract"

func attachSessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/attach",
		OperationID: "attachSession",
		Summary:     "Attach to a resumable live session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody:         contract.AttachSessionRequest{},
		RequestBodyOptional: true,
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionAttachResponse{}},
			{Status: 400, Description: "Invalid attach lease request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Session cannot be attached", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: specServiceUnavailableDependentServiceMissingDescription,
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
