package spec

import "github.com/compozy/compozy/internal/api/contract"

// moveSessionToProfileOperationSpec copies a stopped user session into another profile.
func moveSessionToProfileOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/move-to-profile",
		AppAccess:   AppAccessAllowed,
		OperationID: "moveSessionToProfile",
		Summary:     "Copy a stopped user session into another profile and archive the original",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		),
		RequestBody: contract.MoveSessionToProfileRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MoveSessionToProfileResponse{}},
			{Status: 400, Description: "Invalid move request or target profile", Body: contract.ErrorPayload{}},
			{Status: 403, Description: "The caller cannot write to the target profile", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: "Move refused by a rule; code names the rule", Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Session profile move unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
