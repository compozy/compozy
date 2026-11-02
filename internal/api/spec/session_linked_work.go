package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
)

func sessionLinkedWorkOperation(method, id, summary string) OperationSpec {
	op := OperationSpec{
		Method:      method,
		Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/links",
		AppAccess:   AppAccessAllowed,
		OperationID: id,
		Summary:     summary,
		Tags:        []string{specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  sessionLinkedWorkParameters(method),
		Responses: []ResponseSpec{
			{Status: 200, Description: "Linked work", Body: contract.SessionLinkedWorkResponse{}},
			{Status: 400, Description: "Invalid link or repository mismatch", Body: contract.ErrorPayload{}},
			{Status: 404, Description: specSessionNotFoundDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: "Linked work unavailable", Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
	if method != httpMethodGet {
		op.AppAccess = AppAccessDenied
		op.RequestBody = contract.SessionWorkLinkRequest{}
	}
	return op
}

// sessionLinkedWorkParameters offers the all-profiles lens on reads only: a write resolves an owner.
func sessionLinkedWorkParameters(method string) []ParameterSpec {
	path := []ParameterSpec{pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Session id")}
	if method == httpMethodGet {
		return withProfileScope(path...)
	}
	return withProfileSelector(path...)
}

func sessionLinkedWorkRefreshOperation() OperationSpec {
	op := sessionLinkedWorkOperation(
		httpMethodPost,
		"refreshSessionWorkLink",
		"Ask the forge again about one linked item",
	)
	op.Path = "/api/workspaces/{workspace_id}/sessions/{session_id}/links/refresh"
	return op
}

func sessionWorkCandidatesOperation() OperationSpec {
	op := sessionLinkedWorkOperation(
		httpMethodGet,
		"searchSessionWorkCandidates",
		"Search this project for link candidates",
	)
	op.AppAccess = AppAccessDenied
	op.Path = "/api/workspaces/{workspace_id}/sessions/{session_id}/link-candidates"
	op.Parameters = append(
		op.Parameters,
		enumQueryParam(
			"kind",
			"Candidate source; defaults to pull requests with local branch fallback",
			[]string{"branches", "pull_requests"},
		),
		queryParam("q", "Search repository pull requests or local branches", false),
		queryParam("page", "Page number, starting at 1", false),
		queryParam(
			"limit",
			"Page size, at most 100; page 1 may include one additional exact PR match, which is omitted from later pages",
			false,
		),
	)
	op.Responses[0].Body = session.WorkCandidates{}
	op.Responses = append(
		op.Responses,
		ResponseSpec{Status: 502, Description: "Forge discovery failed", Body: contract.ErrorPayload{}},
	)
	return op
}
