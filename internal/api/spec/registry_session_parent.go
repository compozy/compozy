package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specSessionParentPath              = "/api/workspaces/{workspace_id}/sessions/{session_id}/parent"
	specSessionReparentRefusedDescribe = "Reparent refused by a lineage rule; code names the rule"
	specSessionReparentUnavailable     = "Session reparent unavailable"
)

// setSessionParentOperationSpec adopts a parentless user session as a child.
func setSessionParentOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPut,
		Path:        specSessionParentPath,
		AppAccess:   AppAccessAllowed,
		OperationID: "setSessionParent",
		Summary:     "Adopt a parentless user session as a child of another session",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		),
		RequestBody: contract.SetSessionParentRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 400, Description: "Invalid reparent request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specSessionReparentRefusedDescribe, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSessionReparentUnavailable, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}

// clearSessionParentOperationSpec makes an adopted session top-level again.
func clearSessionParentOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specSessionParentPath,
		AppAccess:   AppAccessAllowed,
		OperationID: "clearSessionParent",
		Summary:     "Make an adopted child session top-level, or take over a stopped spawned worker",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.SessionResponse{}},
			{Status: 400, Description: "Invalid reparent request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 409, Description: specSessionReparentRefusedDescribe, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specSessionReparentUnavailable, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
