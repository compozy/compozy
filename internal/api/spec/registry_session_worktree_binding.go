package spec

import "github.com/compozy/compozy/internal/api/contract"

func sessionWorktreeBindingOperationSpec(bind bool) OperationSpec {
	spec := OperationSpec{
		Method:      httpMethodDelete,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/worktree",
		AppAccess:   AppAccessDenied,
		OperationID: "unbindSessionWorktree",
		Summary:     "Return an idle session to the workspace root in place",
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: withProfileSelector(
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		),
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Same session with updated execution location",
				Body:        contract.SessionResponse{},
			},
			{Status: 400, Description: "Invalid binding request", Body: contract.ErrorPayload{}},
			{Status: 404, Description: "Session or worktree not found", Body: contract.ErrorPayload{}},
			{
				Status:      409,
				Description: "Session busy, stale binding or unavailable worktree",
				Body:        contract.ErrorPayload{},
			},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Binding unavailable", Body: contract.ErrorPayload{}},
		},
	}
	if bind {
		spec.Method = httpMethodPut
		spec.OperationID = "bindSessionWorktree"
		spec.Summary = "Bind an idle existing session to a ready worktree in place"
		spec.RequestBody = contract.BindSessionWorktreeRequest{}
	} else {
		spec.Parameters = append(
			spec.Parameters,
			queryParam("expected_worktree_id", "Expected current worktree id; empty for workspace root", false),
		)
	}
	return spec
}
